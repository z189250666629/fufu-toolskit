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
