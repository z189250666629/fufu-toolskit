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
	if err := validateExecuteMergeRequest(userCustom, []ResolvedToken{{ID: 1, IntervalUnit: 8}}); err == nil || !strings.Contains(err.Error(), "无权指定额度") {
		t.Fatalf("user custom quota err = %v", err)
	}
}

func TestValidateExecuteMergeRequestUserIntervalRules(t *testing.T) {
	oneCard := ExecuteMergeParams{Role: RoleUser, IntervalUnit: 8}
	if err := validateExecuteMergeRequest(oneCard, []ResolvedToken{{ID: 1, IntervalUnit: 3}}); err == nil || !strings.Contains(err.Error(), "单卡续卡") {
		t.Fatalf("one card interval err = %v", err)
	}

	multiCard := ExecuteMergeParams{Role: RoleUser, IntervalUnit: 9}
	if err := validateExecuteMergeRequest(multiCard, []ResolvedToken{{ID: 1, IntervalUnit: 3}, {ID: 2, IntervalUnit: 3}}); err == nil || !strings.Contains(err.Error(), "当前卡组合") {
		t.Fatalf("multi-card invalid interval err = %v", err)
	}

	allowed := ExecuteMergeParams{Role: RoleUser, IntervalUnit: 8}
	if err := validateExecuteMergeRequest(allowed, []ResolvedToken{{ID: 1, IntervalUnit: 3}, {ID: 2, IntervalUnit: 8}}); err != nil {
		t.Fatalf("allowed multi-card interval err = %v", err)
	}
}

func TestValidateExecuteMergeRequestGuestPublicEligibility(t *testing.T) {
	params := ExecuteMergeParams{Role: RoleGuest, IntervalUnit: publicTargetUnit}
	tokens := []ResolvedToken{
		{ID: 1, RemainQuota: 10, IntervalUnit: publicSourceUnit, Status: 1},
		{ID: 2, RemainQuota: 11, IntervalUnit: publicSourceUnit, Status: 1},
	}
	if err := validateExecuteMergeRequest(params, tokens); err != nil {
		t.Fatalf("eligible guest merge err = %v", err)
	}

	tokens[0].UsedQuota = 1
	if err := validateExecuteMergeRequest(params, tokens); err == nil || !strings.Contains(err.Error(), "普通免登录合卡仅支持") {
		t.Fatalf("ineligible guest merge err = %v", err)
	}
}

func TestValidateExecuteMergeRequestRejectsTokensWithoutPositiveIDs(t *testing.T) {
	params := ExecuteMergeParams{Role: RoleAdmin, IntervalUnit: 8}
	tokens := []ResolvedToken{
		{ID: 101, Key: "sk-valid"},
		{ID: 0, Key: "sk-missing-id"},
	}

	err := validateExecuteMergeRequest(params, tokens)
	if err == nil || !strings.Contains(err.Error(), "Token ID 无效") || strings.Contains(err.Error(), "Token 0") {
		t.Fatalf("missing positive token ID err = %v", err)
	}
}
