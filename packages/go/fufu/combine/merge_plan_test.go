package combine

import (
	"strings"
	"testing"
)

func TestBuildMergeTargetPlanUsesVerifiedQuotaAndGeneratedName(t *testing.T) {
	plan, err := buildMergeTargetPlan(
		[]ResolvedToken{
			{RemainQuota: 500000, Group: "vip"},
			{RemainQuota: 1000000, Group: "vip"},
			{RemainQuota: 1000000, Group: "default"},
		},
		nil,
		" ",
		500000,
	)
	if err != nil {
		t.Fatalf("buildMergeTargetPlan: %v", err)
	}
	if plan.Quota != 2500000 || plan.Name != "5" || plan.Group != "vip" {
		t.Fatalf("plan = %#v", plan)
	}
}

func TestBuildMergeTargetPlanUsesCustomQuotaAndName(t *testing.T) {
	quota := int64(42)
	plan, err := buildMergeTargetPlan(
		[]ResolvedToken{{RemainQuota: 1, Group: "default"}},
		&quota,
		" custom name ",
		500000,
	)
	if err != nil {
		t.Fatalf("buildMergeTargetPlan: %v", err)
	}
	if plan.Quota != 42 || plan.Name != "custom name" || plan.Group != "default" {
		t.Fatalf("plan = %#v", plan)
	}
}

func TestBuildMergeTargetPlanRejectsInvalidQuota(t *testing.T) {
	quota := int64(0)
	_, err := buildMergeTargetPlan([]ResolvedToken{{RemainQuota: 1}}, &quota, "", 500000)
	if err == nil || !strings.Contains(err.Error(), "合并额度无效") {
		t.Fatalf("invalid quota err = %v", err)
	}
}

func TestBuildMergeTargetPlanRejectsInvalidQuotaUnit(t *testing.T) {
	_, err := buildMergeTargetPlan([]ResolvedToken{{RemainQuota: 500000}}, nil, "", 0)
	if err == nil || !strings.Contains(err.Error(), "额度单位无效") {
		t.Fatalf("invalid quota unit err = %v", err)
	}
}
