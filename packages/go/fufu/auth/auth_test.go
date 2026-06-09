package auth

import "testing"

func TestCheckAdminToken(t *testing.T) {
	if !CheckAdminToken("secret", "secret", "fallback") {
		t.Fatal("configured token should pass")
	}
	if CheckAdminToken("fallback", "secret", "fallback") {
		t.Fatal("fallback should not pass when configured token exists")
	}
	if !CheckAdminToken("fallback", "", "fallback") {
		t.Fatal("fallback should pass when no configured token exists")
	}
}
