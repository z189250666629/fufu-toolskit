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

func TestKeyHelpers(t *testing.T) {
	if got := ensureFullKey("abc123"); got != "sk-abc123" {
		t.Fatalf("ensureFullKey = %q", got)
	}
	if got := ensureFullKey(" sk-existing "); got != "sk-existing" {
		t.Fatalf("ensureFullKey trims/preserves prefix = %q", got)
	}
	if got := displayKey("sk-abcdefghijkl"); got != "sk-abcd…ijkl" {
		t.Fatalf("displayKey = %q", got)
	}
	if got := keyMask("abcdefghijkl"); got != "sk-abcd…ijkl" {
		t.Fatalf("keyMask = %q", got)
	}
}

func TestNormalizeKeysDedupesAndSkipsBlankValues(t *testing.T) {
	got := normalizeKeys([]string{" abc ", "sk-abc", "", "sk-", "def"})
	if len(got) != 2 || got[0] != "sk-abc" || got[1] != "sk-def" {
		t.Fatalf("normalizeKeys = %#v", got)
	}
}

func TestMajorityGroupAndUniqueIDs(t *testing.T) {
	tokens := []ResolvedToken{
		{ID: 3, Group: "vip"},
		{ID: 1, Group: "vip"},
		{ID: 3, Group: "default"},
		{ID: 2},
	}
	if got := majorityGroup(tokens); got != "vip" {
		t.Fatalf("majorityGroup = %q", got)
	}
	ids := uniqueIDs(tokens)
	if len(ids) != 3 || ids[0] != 3 || ids[1] != 1 || ids[2] != 2 {
		t.Fatalf("uniqueIDs = %#v", ids)
	}
}
