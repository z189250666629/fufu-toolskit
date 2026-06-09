package main

import "testing"

func TestModelManualKeySeparatesSiteAndModel(t *testing.T) {
	if modelManualKey("ab", "c") == modelManualKey("a", "bc") {
		t.Fatalf("manual test key should preserve site/model tuple boundaries")
	}
	if got := modelManualKey("site-a", "model-b"); got != "site-a\x00model-b" {
		t.Fatalf("manual test key = %q", got)
	}
}
