package combinecore

import (
	"strings"
	"testing"
)

func TestValidateExecuteMergeRequestRoleRules(t *testing.T) {
	customQuota := int64(100)
	guest := ExecuteMergeParams{
		Role:         RoleGuest,
		CustomQuota:  true,
		TotalQuota:   &customQuota,
		Name:         "custom",
		IntervalUnit: PublicTargetUnit,
	}
	if err := ValidateExecuteMergeRequest(guest, []ResolvedToken{}); err == nil || !strings.Contains(err.Error(), "普通免登录合卡不支持指定额度") {
		t.Fatalf("guest custom quota err = %v", err)
	}

	userCustom := ExecuteMergeParams{Role: RoleUser, CustomQuota: true, IntervalUnit: 8}
	if err := ValidateExecuteMergeRequest(userCustom, []ResolvedToken{{ID: 1, IntervalUnit: 8}}); err == nil || !strings.Contains(err.Error(), "无权指定额度") {
		t.Fatalf("user custom quota err = %v", err)
	}
}

func TestValidateExecuteMergeRequestUserIntervalRules(t *testing.T) {
	oneCard := ExecuteMergeParams{Role: RoleUser, IntervalUnit: 8}
	if err := ValidateExecuteMergeRequest(oneCard, []ResolvedToken{{ID: 1, IntervalUnit: 3}}); err == nil || !strings.Contains(err.Error(), "单卡续卡") {
		t.Fatalf("one card interval err = %v", err)
	}

	multiCard := ExecuteMergeParams{Role: RoleUser, IntervalUnit: 9}
	if err := ValidateExecuteMergeRequest(multiCard, []ResolvedToken{{ID: 1, IntervalUnit: 3}, {ID: 2, IntervalUnit: 3}}); err == nil || !strings.Contains(err.Error(), "当前卡组合") {
		t.Fatalf("multi-card invalid interval err = %v", err)
	}

	allowed := ExecuteMergeParams{Role: RoleUser, IntervalUnit: 8}
	if err := ValidateExecuteMergeRequest(allowed, []ResolvedToken{{ID: 1, IntervalUnit: 3}, {ID: 2, IntervalUnit: 8}}); err != nil {
		t.Fatalf("allowed multi-card interval err = %v", err)
	}
}

func TestValidateExecuteMergeRequestGuestPublicEligibility(t *testing.T) {
	params := ExecuteMergeParams{Role: RoleGuest, IntervalUnit: PublicTargetUnit}
	tokens := []ResolvedToken{
		{ID: 1, RemainQuota: 10, IntervalUnit: PublicSourceUnit, Status: 1},
		{ID: 2, RemainQuota: 11, IntervalUnit: PublicSourceUnit, Status: 1},
	}
	if err := ValidateExecuteMergeRequest(params, tokens); err != nil {
		t.Fatalf("eligible guest merge err = %v", err)
	}

	tokens[0].UsedQuota = 1
	if err := ValidateExecuteMergeRequest(params, tokens); err == nil || !strings.Contains(err.Error(), "普通免登录合卡仅支持") {
		t.Fatalf("ineligible guest merge err = %v", err)
	}
}

func TestValidateExecuteMergeRequestRejectsUnsupportedIntervalUnits(t *testing.T) {
	for _, unit := range []int{0, 60, -1} {
		params := ExecuteMergeParams{Role: RoleAdmin, IntervalUnit: unit}
		err := ValidateExecuteMergeRequest(params, []ResolvedToken{{ID: 1, IntervalUnit: PublicSourceUnit}})
		if err == nil || !strings.Contains(err.Error(), "卡类型无效") {
			t.Fatalf("unit %d err = %v", unit, err)
		}
	}

	for _, unit := range []int{3, 8, 9} {
		params := ExecuteMergeParams{Role: RoleAdmin, IntervalUnit: unit}
		if err := ValidateExecuteMergeRequest(params, []ResolvedToken{{ID: 1, IntervalUnit: PublicSourceUnit}}); err != nil {
			t.Fatalf("allowed unit %d err = %v", unit, err)
		}
	}
}

func TestValidateExecuteMergeRequestRejectsTokensWithoutPositiveIDs(t *testing.T) {
	params := ExecuteMergeParams{Role: RoleAdmin, IntervalUnit: 8}
	tokens := []ResolvedToken{
		{ID: 101, Key: "sk-valid"},
		{ID: 0, Key: "sk-missing-id", DisplayKey: "sk-miss...g-id"},
	}

	err := ValidateExecuteMergeRequest(params, tokens)
	if err == nil || !strings.Contains(err.Error(), "Token ID 无效") || strings.Contains(err.Error(), "Token 0") {
		t.Fatalf("missing positive token ID err = %v", err)
	}
}

func TestBuildMergeTargetPlanUsesQuotaNameAndMajorityGroup(t *testing.T) {
	plan, err := BuildMergeTargetPlan(
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
		t.Fatalf("BuildMergeTargetPlan: %v", err)
	}
	if plan.Quota != 2500000 || plan.Name != "5" || plan.Group != "vip" {
		t.Fatalf("plan = %#v", plan)
	}

	quota := int64(42)
	plan, err = BuildMergeTargetPlan([]ResolvedToken{{RemainQuota: 1, Group: "default"}}, &quota, " custom name ", 500000)
	if err != nil {
		t.Fatalf("BuildMergeTargetPlan custom: %v", err)
	}
	if plan.Quota != 42 || plan.Name != "custom name" || plan.Group != "default" {
		t.Fatalf("custom plan = %#v", plan)
	}
}

func TestBuildExecuteMergeCardPlanAppliesRolePolicy(t *testing.T) {
	quota := int64(42)
	admin := BuildExecuteMergeCardPlan(ExecuteMergeParams{Keys: []string{"sk-a"}, Role: RoleAdmin, CustomQuota: true, TotalQuota: &quota, Name: " custom ", IntervalUnit: 8})
	if admin.Quota == nil || *admin.Quota != 42 || admin.Name != "custom" {
		t.Fatalf("admin card plan = %#v", admin)
	}

	guest := BuildExecuteMergeCardPlan(ExecuteMergeParams{Keys: []string{"sk-a"}, Role: RoleGuest, CustomQuota: true, TotalQuota: &quota, Name: " custom ", IntervalUnit: 8})
	if guest.Quota != nil || guest.Name != "" {
		t.Fatalf("guest card plan should ignore custom quota/name, got %#v", guest)
	}
}
