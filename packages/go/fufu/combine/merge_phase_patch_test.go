package combine

import "testing"

func TestBuildMergePhasePatchStartsAtZero(t *testing.T) {
	patch := buildMergePhasePatch("verifying", "校验额度中...", 3)

	if patch.Status == nil || *patch.Status != "verifying" {
		t.Fatalf("status = %#v", patch.Status)
	}
	if patch.StepText == nil || *patch.StepText != "校验额度中..." {
		t.Fatalf("step = %#v", patch.StepText)
	}
	if patch.Total == nil || *patch.Total != 3 {
		t.Fatalf("total = %#v", patch.Total)
	}
	if patch.Current == nil || *patch.Current != 0 {
		t.Fatalf("current = %#v", patch.Current)
	}
}
