package main

import "testing"

func TestModelManualKeySeparatesSiteAndModel(t *testing.T) {
	if modelManualKey("ab", "c", "") == modelManualKey("a", "bc", "") {
		t.Fatalf("manual test key should preserve site/model tuple boundaries")
	}
	if got := modelManualKey("site-a", "model-b", ""); got != "site-a\x00model-b\x00" {
		t.Fatalf("manual test key = %q", got)
	}
}

func TestModelManualKeyIncludesGroupBoundary(t *testing.T) {
	if modelManualKey("site", "model", "vip") == modelManualKey("site", "model", "default") {
		t.Fatal("manual test key should separate token groups")
	}
	if modelManualKey("site", "model", "") == modelManualKey("site", "model", "default") {
		t.Fatal("empty group key should not collide with a concrete group")
	}
	if got := modelManualKey("site", "model", "vip"); got != "site\x00model\x00vip" {
		t.Fatalf("manual group key = %q", got)
	}
}
