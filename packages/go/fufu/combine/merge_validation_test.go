package combine

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
		IntervalUnit: publicTargetUnit,
	}
	if err := validateExecuteMergeRequest(guest, []ResolvedToken{}); err == nil || !strings.Contains(err.Error(), "普通免登录合卡不支持指定额度") {
		t.Fatalf("guest custom quota err = %v", err)
	}

	userCustom := ExecuteMergeParams{Role: RoleUser, CustomQuota: true, IntervalUnit: 8}
	if err := validateExecuteMergeRequest(userCustom, []ResolvedToken{{IntervalUnit: 8}}); err == nil || !strings.Contains(err.Error(), "无权指定额度") {
		t.Fatalf("user custom quota err = %v", err)
	}
}

func TestValidateExecuteMergeRequestUserIntervalRules(t *testing.T) {
	oneCard := ExecuteMergeParams{Role: RoleUser, IntervalUnit: 8}
	if err := validateExecuteMergeRequest(oneCard, []ResolvedToken{{IntervalUnit: 3}}); err == nil || !strings.Contains(err.Error(), "单卡续卡") {
		t.Fatalf("one card interval err = %v", err)
	}

	multiCard := ExecuteMergeParams{Role: RoleUser, IntervalUnit: 9}
	if err := validateExecuteMergeRequest(multiCard, []ResolvedToken{{IntervalUnit: 3}, {IntervalUnit: 3}}); err == nil || !strings.Contains(err.Error(), "当前卡组合") {
		t.Fatalf("multi-card invalid interval err = %v", err)
	}

	allowed := ExecuteMergeParams{Role: RoleUser, IntervalUnit: 8}
	if err := validateExecuteMergeRequest(allowed, []ResolvedToken{{IntervalUnit: 3}, {IntervalUnit: 8}}); err != nil {
		t.Fatalf("allowed multi-card interval err = %v", err)
	}
}

func TestValidateExecuteMergeRequestGuestPublicEligibility(t *testing.T) {
	params := ExecuteMergeParams{Role: RoleGuest, IntervalUnit: publicTargetUnit}
	tokens := []ResolvedToken{
		{RemainQuota: 10, IntervalUnit: publicSourceUnit, Status: 1},
		{RemainQuota: 11, IntervalUnit: publicSourceUnit, Status: 1},
	}
	if err := validateExecuteMergeRequest(params, tokens); err != nil {
		t.Fatalf("eligible guest merge err = %v", err)
	}

	tokens[0].UsedQuota = 1
	if err := validateExecuteMergeRequest(params, tokens); err == nil || !strings.Contains(err.Error(), "普通免登录合卡仅支持") {
		t.Fatalf("ineligible guest merge err = %v", err)
	}
}
