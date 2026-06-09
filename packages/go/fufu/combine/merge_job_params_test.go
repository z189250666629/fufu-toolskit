package combine

import "testing"

func TestBuildRunMergeJobParamsMapsPayloadAndRole(t *testing.T) {
	quota := int64(500)
	params := buildRunMergeJobParams("job-1", MergePayload{
		Keys:         []string{"sk-a", "sk-b"},
		IntervalUnit: 60,
		TotalQuota:   &quota,
		Name:         "merged",
		CustomQuota:  true,
	}, RoleAdmin)

	if params.JobID != "job-1" || params.Role != RoleAdmin {
		t.Fatalf("job identity = %#v", params)
	}
	if len(params.Keys) != 2 || params.IntervalUnit != 60 || params.TotalQuota == nil || *params.TotalQuota != 500 {
		t.Fatalf("payload fields = %#v", params)
	}
	if params.Name != "merged" || !params.CustomQuota {
		t.Fatalf("custom fields = %#v", params)
	}
}
