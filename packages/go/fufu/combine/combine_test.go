package combine

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEvaluatePublicMergeEligibility(t *testing.T) {
	ok := evaluatePublicMergeEligibility([]ResolvedToken{{RemainQuota: 1, UsedQuota: 0, IntervalUnit: publicSourceUnit, Status: 1}, {RemainQuota: 1, UsedQuota: 0, IntervalUnit: publicSourceUnit, Status: 1}})
	if !ok.Eligible {
		t.Fatalf("expected eligible: %#v", ok.Reasons)
	}
	bad := evaluatePublicMergeEligibility([]ResolvedToken{{RemainQuota: 1, UsedQuota: 10, IntervalUnit: publicSourceUnit, Status: 1}, {RemainQuota: 1, UsedQuota: 0, IntervalUnit: publicSourceUnit, Status: 1}})
	if bad.Eligible {
		t.Fatalf("expected ineligible")
	}
}

func TestNewAppSharesNewAPIClientWithTokenService(t *testing.T) {
	app := newApp(Config{
		Name:      "test",
		URL:       "https://newapi.example.test",
		Token:     "site-token",
		UserID:    "user-1",
		QuotaUnit: 123,
	}, nil)

	if app.apiClient == nil {
		t.Fatalf("apiClient is nil")
	}
	if app.tokenSvc == nil {
		t.Fatalf("tokenSvc is nil")
	}
	if app.tokenSvc.Client != app.apiClient {
		t.Fatalf("tokenSvc.Client and app.apiClient should share the same instance")
	}
}

func TestMergeLockRejectsConcurrentIDs(t *testing.T) {
	app := &App{mergeLocks: map[int]struct{}{}}
	if !app.acquireMergeLock([]int{1, 2}) {
		t.Fatalf("first lock failed")
	}
	if app.acquireMergeLock([]int{2}) {
		t.Fatalf("overlapping lock should fail")
	}
	app.releaseMergeLock([]int{1, 2})
	if !app.acquireMergeLock([]int{2}) {
		t.Fatalf("lock after release failed")
	}
}

func TestKeyHelpers(t *testing.T) {
	if got := ensureFullKey("abc123"); got != "sk-abc123" {
		t.Fatalf("ensureFullKey = %q", got)
	}
	if got := ensureFullKey(" sk-existing "); got != "sk-existing" {
		t.Fatalf("ensureFullKey trims/preserves prefix = %q", got)
	}
	if got := displayKey("sk-abcdefghijkl"); got != "sk-abcd…ijkl" {
		t.Fatalf("displayKey = %q", got)
	}
	if got := keyMask("abcdefghijkl"); got != "sk-abcd…ijkl" {
		t.Fatalf("keyMask = %q", got)
	}
}

func TestNormalizeKeysDedupesAndSkipsBlankValues(t *testing.T) {
	got := normalizeKeys([]string{" abc ", "sk-abc", "", "sk-", "def"})
	if len(got) != 2 || got[0] != "sk-abc" || got[1] != "sk-def" {
		t.Fatalf("normalizeKeys = %#v", got)
	}
}

func TestMajorityGroupAndUniqueIDs(t *testing.T) {
	tokens := []ResolvedToken{
		{ID: 3, Group: "vip"},
		{ID: 1, Group: "vip"},
		{ID: 3, Group: "default"},
		{ID: 2},
	}
	if got := majorityGroup(tokens); got != "vip" {
		t.Fatalf("majorityGroup = %q", got)
	}
	ids := uniqueIDs(tokens)
	if len(ids) != 3 || ids[0] != 3 || ids[1] != 1 || ids[2] != 2 {
		t.Fatalf("uniqueIDs = %#v", ids)
	}
}

func TestTraceSchemaContainsMergeAndGeneratedTokenTables(t *testing.T) {
	for _, fragment := range []string{
		"CREATE TABLE IF NOT EXISTS merge_records",
		"CREATE TABLE IF NOT EXISTS merge_tokens",
		"CREATE INDEX IF NOT EXISTS idx_merge_tokens_key_hash",
		"CREATE TABLE IF NOT EXISTS generated_tokens",
		"CREATE INDEX IF NOT EXISTS idx_generated_tokens_key_hash",
	} {
		if !strings.Contains(traceSchemaSQL, fragment) {
			t.Fatalf("trace schema missing %q", fragment)
		}
	}
}

func TestPublicAPIRoutes(t *testing.T) {
	for _, path := range []string{"/api/auth", "/api/search-keys", "/api/public-merge", "/api/merge-status/job-1"} {
		if !isPublicAPI(path) {
			t.Fatalf("%s should be public", path)
		}
	}
	if isPublicAPI("/api/session") {
		t.Fatalf("session endpoint should require authentication")
	}
}

func TestHandleAuthCreatesSession(t *testing.T) {
	app := NewApp(Config{}, nil)
	app.passwords = map[string]struct {
		Hash string
		Role Role
	}{
		"admin": {Hash: sha256Hex("test-admin"), Role: RoleAdmin},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/auth", strings.NewReader(`{"password":"test-admin"}`))
	w := httptest.NewRecorder()

	app.handleAuth(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"role":"admin"`) {
		t.Fatalf("auth body=%s", w.Body.String())
	}
	if len(app.sessions) != 1 {
		t.Fatalf("sessions = %#v", app.sessions)
	}
}
