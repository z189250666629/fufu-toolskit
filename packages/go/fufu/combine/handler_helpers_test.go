package combine

import "testing"

func TestMergeStatusJobIDFromPath(t *testing.T) {
	if got := mergeStatusJobIDFromPath("/api/merge-status/job-123"); got != "job-123" {
		t.Fatalf("job id = %q", got)
	}
	if got := mergeStatusJobIDFromPath("/api/merge-status/"); got != "" {
		t.Fatalf("empty job id = %q", got)
	}
}

func TestBuildQueuedMergeJobPatch(t *testing.T) {
	patch := buildQueuedMergeJobPatch(3, RoleUser, "准备合并...")
	if patch.Status == nil || *patch.Status != "queued" {
		t.Fatalf("status = %#v", patch.Status)
	}
	if patch.StepText == nil || *patch.StepText != "准备合并..." {
		t.Fatalf("step = %#v", patch.StepText)
	}
	if patch.Current == nil || *patch.Current != 0 || patch.Total == nil || *patch.Total != 3 {
		t.Fatalf("progress = %#v/%#v", patch.Current, patch.Total)
	}
	if patch.Role == nil || *patch.Role != RoleUser {
		t.Fatalf("role = %#v", patch.Role)
	}
}

func TestBuildMergeAcceptedResponse(t *testing.T) {
	resp := buildMergeAcceptedResponse("job-1")
	if resp["ok"] != true || resp["jobId"] != "job-1" {
		t.Fatalf("response = %#v", resp)
	}
}

func TestCanDeleteTokenRole(t *testing.T) {
	if !canDeleteTokenRole(RoleAdmin) || !canDeleteTokenRole(RoleUser) {
		t.Fatalf("admin/user should delete")
	}
	if canDeleteTokenRole(RoleGuest) || canDeleteTokenRole("") {
		t.Fatalf("guest/empty should not delete")
	}
}

func TestDeleteTokenIDFromPath(t *testing.T) {
	id, ok := deleteTokenIDFromPath("/api/token/42")
	if !ok || id != 42 {
		t.Fatalf("valid id = %d/%v", id, ok)
	}
	for _, path := range []string{"/api/token/", "/api/token/0", "/api/token/not-a-number"} {
		if id, ok := deleteTokenIDFromPath(path); ok || id != 0 {
			t.Fatalf("invalid %q = %d/%v", path, id, ok)
		}
	}
}

func TestBuildSearchKeysResponseIncludesEligibilityAndTrace(t *testing.T) {
	keys := []string{"sk-a", "sk-b"}
	found := []ResolvedToken{
		{RemainQuota: 1, UsedQuota: 0, IntervalUnit: publicSourceUnit, Status: 1},
		{RemainQuota: 1, UsedQuota: 0, IntervalUnit: publicSourceUnit, Status: 1},
	}
	missing := []string{"sk-missing"}
	traces := []TraceResult{{MergeID: 7}}

	resp := buildSearchKeysResponse(keys, found, missing, 500000, 123, traces)
	if resp["searched"] != 2 || resp["quotaUnit"] != int64(500000) || resp["elapsedMs"] != int64(123) {
		t.Fatalf("basic response = %#v", resp)
	}
	if resp["concurrency"] != 2 {
		t.Fatalf("concurrency = %#v", resp["concurrency"])
	}
	elig, ok := resp["publicMergeEligibility"].(map[string]any)
	if !ok || elig["eligible"] != true || elig["targetUnit"] != publicTargetUnit {
		t.Fatalf("eligibility = %#v", resp["publicMergeEligibility"])
	}
	if got := resp["traceResults"].([]TraceResult); len(got) != 1 || got[0].MergeID != 7 {
		t.Fatalf("trace results = %#v", resp["traceResults"])
	}
}
