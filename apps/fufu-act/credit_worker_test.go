package activityapp

import (
	"errors"
	"strconv"
	"testing"
)

func TestProcessCreditsWithRoutesQuotaSuccessAndFailure(t *testing.T) {
	store := &fakeCreditProcessorStore{rows: &fakeCreditRows{results: []fakeCreditRowResult{
		{item: creditQueueItem{ID: 1, CardKey: "ok-card", PrizeDollars: 10, Retries: 0}},
		{item: creditQueueItem{ID: 2, CardKey: "fail-card", PrizeDollars: 20, Retries: 1}},
	}}}
	quota := &fakeCreditQuota{failures: map[string]error{"fail-card": errors.New("raw upstream detail")}}

	processCreditsWith(store, quota, 2)

	if store.pendingMaxRetries != 2 || store.pendingLimit != creditBatchLimit {
		t.Fatalf("pending args max=%d limit=%d", store.pendingMaxRetries, store.pendingLimit)
	}
	if len(quota.calls) != 2 || quota.calls[0] != "ok-card:10" || quota.calls[1] != "fail-card:20" {
		t.Fatalf("quota calls=%#v", quota.calls)
	}
	if len(store.doneIDs) != 1 || store.doneIDs[0] != 1 {
		t.Fatalf("done IDs=%#v", store.doneIDs)
	}
	if len(store.quotaFailures) != 1 {
		t.Fatalf("quota failures=%#v", store.quotaFailures)
	}
	failure := store.quotaFailures[0]
	if failure.id != 2 || failure.update.Retries != 2 || failure.update.Status != creditStatusFailed || failure.update.Error != "派奖失败，请人工处理" {
		t.Fatalf("failure=%#v", failure)
	}
}

func TestProcessCreditsWithMarksScannedRowIDFailedOnScanError(t *testing.T) {
	store := &fakeCreditProcessorStore{rows: &fakeCreditRows{results: []fakeCreditRowResult{
		{item: creditQueueItem{ID: 7}, err: errors.New("scan leaked key")},
	}}}

	processCreditsWith(store, &fakeCreditQuota{}, 5)

	if len(store.scanFailures) != 1 {
		t.Fatalf("scan failures=%#v", store.scanFailures)
	}
	if store.scanFailures[0].id != 7 || store.scanFailures[0].message != "队列数据异常，请人工检查" {
		t.Fatalf("scan failure=%#v", store.scanFailures[0])
	}
}

type fakeCreditProcessorStore struct {
	rows              creditPendingRows
	pendingMaxRetries int
	pendingLimit      int
	scanFailures      []fakeCreditScanFailure
	quotaFailures     []fakeCreditQuotaFailure
	doneIDs           []int
}

func (s *fakeCreditProcessorStore) Pending(maxRetries, limit int) (creditPendingRows, error) {
	s.pendingMaxRetries = maxRetries
	s.pendingLimit = limit
	return s.rows, nil
}

func (s *fakeCreditProcessorStore) MarkScanFailed(id int, message string) error {
	s.scanFailures = append(s.scanFailures, fakeCreditScanFailure{id: id, message: message})
	return nil
}

func (s *fakeCreditProcessorStore) MarkQuotaFailed(id int, update creditFailureUpdate) error {
	s.quotaFailures = append(s.quotaFailures, fakeCreditQuotaFailure{id: id, update: update})
	return nil
}

func (s *fakeCreditProcessorStore) MarkDone(id int) error {
	s.doneIDs = append(s.doneIDs, id)
	return nil
}

type fakeCreditScanFailure struct {
	id      int
	message string
}

type fakeCreditQuotaFailure struct {
	id     int
	update creditFailureUpdate
}

type fakeCreditRowResult struct {
	item creditQueueItem
	err  error
}

type fakeCreditRows struct {
	results []fakeCreditRowResult
	index   int
}

func (r *fakeCreditRows) Next() bool {
	return r.index < len(r.results)
}

func (r *fakeCreditRows) Scan() (creditQueueItem, error) {
	result := r.results[r.index]
	r.index++
	return result.item, result.err
}

func (r *fakeCreditRows) Close() error {
	return nil
}

func (r *fakeCreditRows) Err() error {
	return nil
}

type fakeCreditQuota struct {
	failures map[string]error
	calls    []string
}

func (q *fakeCreditQuota) AddQuota(key string, prizeDollars int) error {
	q.calls = append(q.calls, key+":"+strconv.Itoa(prizeDollars))
	return q.failures[key]
}
