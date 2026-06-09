package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func newTestApp(t *testing.T) *App {
	t.Helper()
	db, err := initTraceDB(filepath.Join(t.TempDir(), "trace.db"))
	if err != nil {
		t.Fatalf("init trace db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return newApp(Config{
		URL:       "http://127.0.0.1:1",
		Token:     "test-token",
		UserID:    "1",
		QuotaUnit: 500000,
	}, db)
}

func TestTraceResultsForSourceAndResultKeys(t *testing.T) {
	app := newTestApp(t)
	ctx := context.Background()

	mergeID, err := app.createMergeTrace(ctx, "job-test", RoleUser, 9)
	if err != nil {
		t.Fatalf("create trace: %v", err)
	}
	sourceA := ResolvedToken{ID: 101, Key: "sk-source-a-abcdef", Name: "source-a", RemainQuota: 500000, IntervalUnit: 3, Group: "mix", Status: 1}
	sourceB := ResolvedToken{ID: 102, Key: "sk-source-b-abcdef", Name: "source-b", RemainQuota: 500000, IntervalUnit: 3, Group: "mix", Status: 1}
	result := ResolvedToken{ID: 201, Key: "sk-result-abcdef", Name: "2", RemainQuota: 1000000, IntervalUnit: 9, Group: "mix", Status: 1}
	for _, token := range []ResolvedToken{sourceA, sourceB} {
		if err := app.upsertTraceToken(ctx, mergeID, "source", token); err != nil {
			t.Fatalf("insert source token: %v", err)
		}
	}
	if err := app.upsertTraceToken(ctx, mergeID, "result", result); err != nil {
		t.Fatalf("insert result token: %v", err)
	}
	app.setTraceFinal(ctx, mergeID, result.RemainQuota, result.Name, result.Group)
	app.finishTrace(ctx, mergeID, "done", "")

	byResult, err := app.traceResultsForKeys(ctx, []string{result.Key})
	if err != nil {
		t.Fatalf("trace by result: %v", err)
	}
	if len(byResult) != 1 {
		t.Fatalf("expected 1 trace by result, got %d", len(byResult))
	}
	if byResult[0].Direction != "result" {
		t.Fatalf("expected result direction, got %q", byResult[0].Direction)
	}
	if byResult[0].ResultKey == nil || byResult[0].ResultKey.Key != result.Key {
		t.Fatalf("expected full result key, got %#v", byResult[0].ResultKey)
	}
	if len(byResult[0].SourceKeys) != 2 || byResult[0].SourceKeys[0].Key != sourceA.Key {
		t.Fatalf("expected full source keys, got %#v", byResult[0].SourceKeys)
	}

	bySource, err := app.traceResultsForKeys(ctx, []string{sourceA.Key})
	if err != nil {
		t.Fatalf("trace by source: %v", err)
	}
	if len(bySource) != 1 {
		t.Fatalf("expected 1 trace by source, got %d", len(bySource))
	}
	if bySource[0].Direction != "source" {
		t.Fatalf("expected source direction, got %q", bySource[0].Direction)
	}
	if bySource[0].ResultKey == nil || bySource[0].ResultKey.Key != result.Key {
		t.Fatalf("expected resulting key for source lookup, got %#v", bySource[0].ResultKey)
	}
}

func TestTraceResultsExpandMultiStepChain(t *testing.T) {
	app := newTestApp(t)
	ctx := context.Background()

	sourceA := ResolvedToken{ID: 101, Key: "sk-chain-source-a-abcdef", Name: "55", RemainQuota: 500000, IntervalUnit: 3, Group: "mix", Status: 1}
	sourceB := ResolvedToken{ID: 102, Key: "sk-chain-source-b-abcdef", Name: "55", RemainQuota: 500000, IntervalUnit: 3, Group: "mix", Status: 1}
	intermediate := ResolvedToken{ID: 201, Key: "sk-chain-intermediate-abcdef", Name: "110", RemainQuota: 1000000, IntervalUnit: 8, Group: "mix", Status: 1}
	sourceC := ResolvedToken{ID: 103, Key: "sk-chain-source-c-abcdef", Name: "55", RemainQuota: 500000, IntervalUnit: 3, Group: "mix", Status: 1}
	final := ResolvedToken{ID: 301, Key: "sk-chain-final-abcdef", Name: "165", RemainQuota: 1500000, IntervalUnit: 9, Group: "mix", Status: 1}

	firstID, err := app.createMergeTrace(ctx, "job-chain-1", RoleUser, 8)
	if err != nil {
		t.Fatalf("create first trace: %v", err)
	}
	for _, token := range []ResolvedToken{sourceA, sourceB} {
		if err := app.upsertTraceToken(ctx, firstID, "source", token); err != nil {
			t.Fatalf("insert first source: %v", err)
		}
	}
	if err := app.upsertTraceToken(ctx, firstID, "result", intermediate); err != nil {
		t.Fatalf("insert first result: %v", err)
	}
	app.setTraceFinal(ctx, firstID, intermediate.RemainQuota, intermediate.Name, intermediate.Group)
	app.finishTrace(ctx, firstID, "done", "")
	if _, err := app.db.ExecContext(ctx, `UPDATE merge_records SET created_at = ?, updated_at = ?, completed_at = ? WHERE id = ?`, int64(1000), int64(1000), int64(1000), firstID); err != nil {
		t.Fatalf("set first trace time: %v", err)
	}

	secondID, err := app.createMergeTrace(ctx, "job-chain-2", RoleUser, 9)
	if err != nil {
		t.Fatalf("create second trace: %v", err)
	}
	for _, token := range []ResolvedToken{intermediate, sourceC} {
		if err := app.upsertTraceToken(ctx, secondID, "source", token); err != nil {
			t.Fatalf("insert second source: %v", err)
		}
	}
	if err := app.upsertTraceToken(ctx, secondID, "result", final); err != nil {
		t.Fatalf("insert second result: %v", err)
	}
	app.setTraceFinal(ctx, secondID, final.RemainQuota, final.Name, final.Group)
	app.finishTrace(ctx, secondID, "done", "")
	if _, err := app.db.ExecContext(ctx, `UPDATE merge_records SET created_at = ?, updated_at = ?, completed_at = ? WHERE id = ?`, int64(2000), int64(2000), int64(2000), secondID); err != nil {
		t.Fatalf("set second trace time: %v", err)
	}

	traces, err := app.traceResultsForKeys(ctx, []string{sourceA.Key})
	if err != nil {
		t.Fatalf("trace by first source: %v", err)
	}
	if len(traces) != 2 {
		t.Fatalf("expected expanded two-step chain, got %#v", traces)
	}
	if traces[0].MergeID != firstID || traces[1].MergeID != secondID {
		t.Fatalf("expected chronological chain %d -> %d, got %d -> %d", firstID, secondID, traces[0].MergeID, traces[1].MergeID)
	}
	if traces[0].Direction != "source" || traces[1].Direction != "related" {
		t.Fatalf("unexpected directions: %q, %q", traces[0].Direction, traces[1].Direction)
	}
	if traces[0].ResultKey == nil || traces[0].ResultKey.Key != intermediate.Key {
		t.Fatalf("expected intermediate result in first trace, got %#v", traces[0].ResultKey)
	}
	if len(traces[1].SourceKeys) == 0 || traces[1].SourceKeys[0].Key != intermediate.Key {
		t.Fatalf("expected intermediate source in second trace, got %#v", traces[1].SourceKeys)
	}
}

func TestTraceResultsEmptyForUnknownKey(t *testing.T) {
	app := newTestApp(t)
	traces, err := app.traceResultsForKeys(context.Background(), []string{"sk-unknown-abcdef"})
	if err != nil {
		t.Fatalf("trace unknown: %v", err)
	}
	if len(traces) != 0 {
		t.Fatalf("expected no traces, got %#v", traces)
	}
}

func TestSearchKeysReturnsTraceForDeletedSourceKey(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/token/search" {
			writeJSON(w, 200, map[string]any{"data": []any{}})
			return
		}
		writeJSON(w, 404, map[string]string{"error": "not found"})
	}))
	defer upstream.Close()

	app := newTestApp(t)
	app.apiURL = upstream.URL
	ctx := context.Background()
	mergeID, err := app.createMergeTrace(ctx, "job-deleted-source", RoleUser, 9)
	if err != nil {
		t.Fatalf("create trace: %v", err)
	}
	source := ResolvedToken{ID: 101, Key: "sk-deleted-source-abcdef", Name: "old", RemainQuota: 500000, IntervalUnit: 3, Group: "mix", Status: 1}
	result := ResolvedToken{ID: 201, Key: "sk-result-for-deleted-abcdef", Name: "1", RemainQuota: 500000, IntervalUnit: 9, Group: "mix", Status: 1}
	if err := app.upsertTraceToken(ctx, mergeID, "source", source); err != nil {
		t.Fatalf("insert source: %v", err)
	}
	if err := app.upsertTraceToken(ctx, mergeID, "result", result); err != nil {
		t.Fatalf("insert result: %v", err)
	}
	app.finishTrace(ctx, mergeID, "done", "")

	body, _ := json.Marshal(map[string]any{"keys": []string{source.Key}})
	req := httptest.NewRequest(http.MethodPost, "/api/search-keys", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Found        []ResolvedToken `json:"found"`
		Missing      []string        `json:"missing"`
		TraceResults []TraceResult   `json:"traceResults"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Found) != 0 {
		t.Fatalf("expected no upstream tokens, got %#v", payload.Found)
	}
	if len(payload.Missing) != 1 {
		t.Fatalf("expected missing source key, got %#v", payload.Missing)
	}
	if len(payload.TraceResults) != 1 || payload.TraceResults[0].ResultKey == nil || payload.TraceResults[0].ResultKey.Key != result.Key {
		t.Fatalf("expected trace result key, got %#v", payload.TraceResults)
	}
}

func TestSearchKeysFallsBackToGeneratedTokenCache(t *testing.T) {
	const generatedKey = "sk-cache-generated-abcdef"
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/token/search":
			writeJSON(w, 200, map[string]any{"data": []any{}})
		case "/api/token/301":
			writeJSON(w, 200, map[string]any{"data": map[string]any{
				"id":            301,
				"key":           "cache-generated-abcdef",
				"name":          "cached",
				"remain_quota":  500000,
				"used_quota":    0,
				"interval_unit": 9,
				"group":         "mix",
				"status":        1,
			}})
		default:
			writeJSON(w, 404, map[string]string{"error": "not found"})
		}
	}))
	defer upstream.Close()

	app := newTestApp(t)
	app.apiURL = upstream.URL
	if err := app.upsertGeneratedToken(context.Background(), ResolvedToken{
		ID:           301,
		Key:          generatedKey,
		Name:         "cached",
		RemainQuota:  500000,
		IntervalUnit: 9,
		Group:        "mix",
		Status:       1,
	}); err != nil {
		t.Fatalf("insert generated token cache: %v", err)
	}

	body, _ := json.Marshal(map[string]any{"keys": []string{generatedKey}})
	req := httptest.NewRequest(http.MethodPost, "/api/search-keys", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Found   []ResolvedToken `json:"found"`
		Missing []string        `json:"missing"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Found) != 1 || payload.Found[0].Key != generatedKey {
		t.Fatalf("expected cached generated token to be found, got %#v", payload.Found)
	}
	if len(payload.Missing) != 0 {
		t.Fatalf("expected no missing keys, got %#v", payload.Missing)
	}
}

func TestGenerateReturnsVerifiedCanonicalKey(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/token/":
			writeJSON(w, 200, map[string]any{"success": true})
		case r.Method == http.MethodGet && r.URL.Path == "/api/token/search":
			writeJSON(w, 200, map[string]any{"data": []any{map[string]any{
				"id":            777,
				"key":           "stale-search-key",
				"name":          r.URL.Query().Get("keyword"),
				"remain_quota":  500000,
				"interval_unit": 9,
				"group":         "mix",
				"status":        1,
			}}})
		case r.Method == http.MethodPut && r.URL.Path == "/api/token/":
			writeJSON(w, 200, map[string]any{"success": true})
		case r.Method == http.MethodGet && r.URL.Path == "/api/token/777":
			writeJSON(w, 200, map[string]any{"data": map[string]any{
				"id":            777,
				"key":           "canonical-generated-key",
				"name":          "1",
				"remain_quota":  500000,
				"used_quota":    0,
				"interval_unit": 9,
				"group":         "mix",
				"status":        1,
			}})
		default:
			writeJSON(w, 404, map[string]string{"error": "not found"})
		}
	}))
	defer upstream.Close()

	app := newTestApp(t)
	app.apiURL = upstream.URL
	app.mu.Lock()
	app.sessions["admin-session"] = SessionInfo{Expiry: nowPlusHour(), Role: RoleAdmin}
	app.mu.Unlock()

	body, _ := json.Marshal(map[string]any{"count": 1, "quota": 1, "intervalUnit": 9, "group": "mix"})
	req := httptest.NewRequest(http.MethodPost, "/api/generate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Session-Token", "admin-session")
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Keys   []string `json:"keys"`
		Errors []string `json:"errors"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Errors) != 0 {
		t.Fatalf("expected no errors, got %#v", payload.Errors)
	}
	if len(payload.Keys) != 1 || payload.Keys[0] != "sk-canonical-generated-key" {
		t.Fatalf("expected canonical verified key, got %#v", payload.Keys)
	}
	if id, ok, err := app.generatedTokenIDByKey(context.Background(), payload.Keys[0]); err != nil || !ok || id != 777 {
		t.Fatalf("expected generated cache for canonical key, id=%d ok=%v err=%v", id, ok, err)
	}
}

func nowPlusHour() time.Time {
	return time.Now().Add(time.Hour)
}
