package combine

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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
	got := normalizeKeys([]string{" abcdefghijkl ", "sk-abcdefghijkl", "", "sk-", "defghijklmno"})
	if len(got) != 2 || got[0] != "sk-abcdefghijkl" || got[1] != "sk-defghijklmno" {
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
		if !IsAPIPath(path) {
			t.Fatalf("%s should be a combine API path", path)
		}
	}
	if isPublicAPI("/api/session") {
		t.Fatalf("session endpoint should require authentication")
	}
	if IsAPIPath("/api/health") {
		t.Fatalf("network health endpoint should not be a combine API path")
	}
}

func TestMergeStatusRequiresSessionForNonGuestJobs(t *testing.T) {
	app := NewApp(Config{}, nil)
	guestRole := RoleGuest
	userRole := RoleUser
	app.setMergeJob("job-guest", MergeJobPatch{Status: strp("done"), Role: &guestRole, Result: map[string]any{"ok": true}, HasResult: true})
	app.setMergeJob("job-user", MergeJobPatch{Status: strp("done"), Role: &userRole, Result: map[string]any{"newCard": map[string]string{"key": "sk-secret"}}, HasResult: true})
	app.sessions["user-session"] = SessionInfo{Expiry: time.Now().Add(time.Hour), Role: RoleUser}

	guestReq := httptest.NewRequest(http.MethodGet, "/api/merge-status/job-guest", nil)
	guestRec := httptest.NewRecorder()
	app.handleAPI(guestRec, guestReq)
	if guestRec.Code != http.StatusOK {
		t.Fatalf("guest status code=%d body=%s", guestRec.Code, guestRec.Body.String())
	}

	anonReq := httptest.NewRequest(http.MethodGet, "/api/merge-status/job-user", nil)
	anonRec := httptest.NewRecorder()
	app.handleAPI(anonRec, anonReq)
	if anonRec.Code != http.StatusUnauthorized || strings.Contains(anonRec.Body.String(), "sk-secret") {
		t.Fatalf("anonymous status code=%d body=%s", anonRec.Code, anonRec.Body.String())
	}

	authReq := httptest.NewRequest(http.MethodGet, "/api/merge-status/job-user", nil)
	authReq.Header.Set("X-Session-Token", "user-session")
	authRec := httptest.NewRecorder()
	app.handleAPI(authRec, authReq)
	if authRec.Code != http.StatusOK || !strings.Contains(authRec.Body.String(), "sk-secret") {
		t.Fatalf("authenticated status code=%d body=%s", authRec.Code, authRec.Body.String())
	}
}

func TestGuestMergeStatusRedactsSourceKeysAndRawErrors(t *testing.T) {
	app := NewApp(Config{}, nil)
	guestRole := RoleGuest
	app.setMergeJob("job-guest-secret", MergeJobPatch{
		Status: strp("error"),
		Role:   &guestRole,
		Result: MergeResult{
			Success: false,
			NewCard: NewCardResult{
				Key: "sk-new-card-secret-1234567890",
			},
			DeleteResults: []DeleteResult{{ID: 1, Key: "sk-source-secret-1234567890", OK: false}},
		},
		HasResult: true,
		Error:     strp("upstream panic: sk-source-secret-1234567890"),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/merge-status/job-guest-secret", nil)
	rec := httptest.NewRecorder()
	app.handleAPI(rec, req)

	body := rec.Body.String()
	if rec.Code != http.StatusOK {
		t.Fatalf("status code=%d body=%s", rec.Code, body)
	}
	if strings.Contains(body, "sk-source-secret-1234567890") || strings.Contains(body, "upstream panic") {
		t.Fatalf("guest status leaked source key or raw error: %s", body)
	}
	if !strings.Contains(body, "sk-sour…7890") || !strings.Contains(body, "合并失败，请稍后重试") {
		t.Fatalf("guest status should keep safe diagnostics: %s", body)
	}
	if !strings.Contains(body, "sk-new-card-secret-1234567890") {
		t.Fatalf("guest status should still return the generated card key: %s", body)
	}
}

func TestHandleAPIRejectsWrongMethodWithMethodNotAllowed(t *testing.T) {
	app := NewApp(Config{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/auth", nil)
	w := httptest.NewRecorder()

	app.handleAPI(w, req)

	if w.Code != http.StatusMethodNotAllowed || strings.TrimSpace(w.Body.String()) != `{"error":"Only POST"}` {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
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

func TestHandleSearchKeysRejectsBlankOnlyKeys(t *testing.T) {
	app := NewApp(Config{}, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/search-keys", strings.NewReader(`{"keys":["  ","sk-"]}`))
	w := httptest.NewRecorder()

	app.handleSearchKeys(w, req)

	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "No keys provided") {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestHandleSearchKeysRedactsLookupErrorsFromPublicAPI(t *testing.T) {
	app := NewApp(Config{URL: "http://internal.example.local/%zz", Token: "secret-token", UserID: "1"}, nil)
	secretKey := "sk-public-search-secret-1234567890"
	req := httptest.NewRequest(http.MethodPost, "/api/search-keys", strings.NewReader(`{"keys":["`+secretKey+`"]}`))
	w := httptest.NewRecorder()

	app.handleSearchKeys(w, req)

	body := strings.TrimSpace(w.Body.String())
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("code=%d body=%s", w.Code, body)
	}
	if body != `{"error":"查询失败，请稍后重试"}` {
		t.Fatalf("expected safe public error, got %s", body)
	}
	for _, leaked := range []string{"internal.example.local", "%zz", "parse", "invalid URL", "secret-token", secretKey} {
		if strings.Contains(body, leaked) {
			t.Fatalf("search-keys error leaked %q in %s", leaked, body)
		}
	}
}

func TestHandleDeleteTokenRedactsUpstreamErrors(t *testing.T) {
	app := NewApp(Config{URL: "http://internal.example.local/%zz", Token: "secret-token", UserID: "1"}, nil)
	app.sessions["admin-session"] = SessionInfo{Expiry: time.Now().Add(time.Hour), Role: RoleAdmin}
	req := httptest.NewRequest(http.MethodDelete, "/api/token/7", nil)
	req.Header.Set("X-Session-Token", "admin-session")
	w := httptest.NewRecorder()

	app.handleAPI(w, req)

	body := strings.TrimSpace(w.Body.String())
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("code=%d body=%s", w.Code, body)
	}
	if body != `{"error":"删除失败，请稍后重试"}` {
		t.Fatalf("expected safe delete error, got %s", body)
	}
	for _, leaked := range []string{"internal.example.local", "%zz", "parse", "invalid URL", "secret-token"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("delete error leaked %q in %s", leaked, body)
		}
	}
}

func TestHandleMergeRejectsEmptyKeysBeforeQueuing(t *testing.T) {
	app := NewApp(Config{}, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/merge", strings.NewReader(`{"keys":["  ","sk-"]}`))
	w := httptest.NewRecorder()

	app.handleMerge(w, req)

	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "No keys provided") {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	if len(app.mergeJobs) != 0 {
		t.Fatalf("blank keys should not queue merge jobs: %#v", app.mergeJobs)
	}
}

func TestHandlePublicMergeRejectsEmptyKeysBeforeQueuing(t *testing.T) {
	app := NewApp(Config{}, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/public-merge", strings.NewReader(`{"keys":["  ","sk-"]}`))
	w := httptest.NewRecorder()

	app.handlePublicMerge(w, req)

	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "No keys provided") {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	if len(app.mergeJobs) != 0 {
		t.Fatalf("blank keys should not queue public merge jobs: %#v", app.mergeJobs)
	}
}
