package combine

import "testing"

func TestEvaluatePublicMergeEligibility(t *testing.T) {
	ok := evaluatePublicMergeEligibility([]ResolvedToken{{RemainQuota: 1, UsedQuota: 0, IntervalUnit: publicSourceUnit, Status: 1}, {RemainQuota: 1, UsedQuota: 0, IntervalUnit: publicSourceUnit, Status: 1}})
	if !ok.Eligible {
		t.Fatalf("expected eligible: %#v", ok.Reasons)
	}
	bad := evaluatePublicMergeEligibility([]ResolvedToken{{RemainQuota: 1, UsedQuota: 10, IntervalUnit: publicSourceUnit, Status: 1}, {RemainQuota: 1, UsedQuota: 0, IntervalUnit: publicSourceUnit, Status: 1}})
	if bad.Eligible {
		t.Fatalf("expected ineligible")
	}
}

func TestMergeLockRejectsConcurrentIDs(t *testing.T) {
	app := &App{mergeLocks: map[int]struct{}{}}
	if !app.acquireMergeLock([]int{1, 2}) {
		t.Fatalf("first lock failed")
	}
	if app.acquireMergeLock([]int{2}) {
		t.Fatalf("overlapping lock should fail")
	}
	app.releaseMergeLock([]int{1, 2})
	if !app.acquireMergeLock([]int{2}) {
		t.Fatalf("lock after release failed")
	}
}
