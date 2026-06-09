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
