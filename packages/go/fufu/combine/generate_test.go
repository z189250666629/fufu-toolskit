package combine

import "testing"

func TestNormalizeGenerateGroupDefaultsBlank(t *testing.T) {
	if got := normalizeGenerateGroup("  "); got != "mix" {
		t.Fatalf("blank group = %q", got)
	}
	if got := normalizeGenerateGroup(" vip "); got != "vip" {
		t.Fatalf("trimmed group = %q", got)
	}
}

func TestBuildGeneratedTokenCreateBody(t *testing.T) {
	body := buildGeneratedTokenCreateBody("gen-temp", 500000, "vip", 60)

	if body["name"] != "gen-temp" || body["remain_quota"] != int64(500000) || body["interval_quota"] != int64(500000) {
		t.Fatalf("quota fields = %#v", body)
	}
	if body["group"] != "vip" || body["interval_unit"] != 60 || body["unlimited_quota"] != false {
		t.Fatalf("settings fields = %#v", body)
	}
	if body["expired_time"] != -1 || body["interval_time"] != -1 || body["trigger_last_time"] != 0 {
		t.Fatalf("time fields = %#v", body)
	}
}

func TestValidateGenerateParams(t *testing.T) {
	validCases := []struct {
		count        int
		quota        float64
		intervalUnit int
	}{
		{count: 1, quota: 0.1, intervalUnit: 60},
		{count: 100, quota: 1, intervalUnit: -1},
	}
	for _, tc := range validCases {
		if !validateGenerateParams(tc.count, tc.quota, tc.intervalUnit, 500000) {
			t.Fatalf("expected valid params: %#v", tc)
		}
	}

	invalidCases := []struct {
		count        int
		quota        float64
		intervalUnit int
	}{
		{count: 0, quota: 1, intervalUnit: 60},
		{count: 101, quota: 1, intervalUnit: 60},
		{count: 1, quota: 0, intervalUnit: 60},
		{count: 1, quota: 1, intervalUnit: 0},
	}
	for _, tc := range invalidCases {
		if validateGenerateParams(tc.count, tc.quota, tc.intervalUnit, 500000) {
			t.Fatalf("expected invalid params: %#v", tc)
		}
	}
}

func TestValidateGenerateParamsRejectsQuotaThatRoundsToZero(t *testing.T) {
	if validateGenerateParams(1, 0.0000001, 60, 500000) {
		t.Fatal("quota that rounds to zero should be invalid")
	}
}

func TestGenerateQuotaAndFinalName(t *testing.T) {
	if got := generateTotalQuota(1.25, 500000); got != 625000 {
		t.Fatalf("generateTotalQuota = %d", got)
	}
	if got := generateTokenFinalName(1.2500); got != "1.25" {
		t.Fatalf("generateTokenFinalName = %q", got)
	}
}
