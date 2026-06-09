package combine

import "testing"

func TestMatchRoleByPasswordFindsConfiguredHash(t *testing.T) {
	passwords := map[string]struct {
		Hash string
		Role Role
	}{
		"admin": {Hash: sha256Hex("admin-pass"), Role: RoleAdmin},
		"user":  {Hash: sha256Hex("user-pass"), Role: RoleUser},
	}

	role, ok := matchRoleByPassword(passwords, "user-pass")
	if !ok || role != RoleUser {
		t.Fatalf("matchRoleByPassword = %q/%v", role, ok)
	}

	role, ok = matchRoleByPassword(passwords, "missing")
	if ok || role != "" {
		t.Fatalf("missing match = %q/%v", role, ok)
	}
}
