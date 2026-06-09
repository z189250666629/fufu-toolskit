package combine

import "testing"

func TestBuildExecuteMergeCardParamsAppliesAdminCustomQuotaAndName(t *testing.T) {
	quota := int64(42)
	patches := []MergeJobPatch{}

	params := buildExecuteMergeCardParams(ExecuteMergeParams{
		Keys:         []string{"sk-a"},
		IntervalUnit: 60,
		TotalQuota:   &quota,
		Name:         " custom name ",
		CustomQuota:  true,
		Role:         RoleAdmin,
		JobID:        "job-1",
	}, func(patch MergeJobPatch) {
		patches = append(patches, patch)
	})

	if params.Quota == nil || *params.Quota != 42 {
		t.Fatalf("quota = %#v", params.Quota)
	}
	if params.Name != "custom name" || params.Role != RoleAdmin || params.JobID != "job-1" {
		t.Fatalf("params = %#v", params)
	}
	params.OnProgress(MergeJobPatch{Current: intp(1)})
	if len(patches) != 1 || patches[0].Current == nil || *patches[0].Current != 1 {
		t.Fatalf("patches = %#v", patches)
	}
}

func TestBuildExecuteMergeCardParamsClearsGuestOverrides(t *testing.T) {
	quota := int64(42)
	params := buildExecuteMergeCardParams(ExecuteMergeParams{
		TotalQuota:  &quota,
		Name:        " guest name ",
		CustomQuota: true,
		Role:        RoleGuest,
	}, nil)

	if params.Quota != nil || params.Name != "" {
		t.Fatalf("guest params = %#v", params)
	}
}
