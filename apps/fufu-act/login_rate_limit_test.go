package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"fufu/newapi"
	"fufu/tokens"
)

func TestHandleLoginRateLimitsRepeatedUnknownCardKey(t *testing.T) {
	setupScratchLockTestDB(t)
	t.Setenv("MCY_BASE_URL", "")
	t.Setenv("SHOP_BASE_URL", "")

	key := "sk-unknown-login-card-123"
	var searchHits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/token/search" {
			t.Fatalf("unexpected token request %s %s", r.Method, r.URL.String())
		}
		searchHits.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": []any{}})
	}))
	t.Cleanup(server.Close)
	oldTokenSvc := tokenSvc
	tokenSvc = tokens.NewService(newapi.NewClient(newapi.Site{URL: server.URL, Token: "token", UserID: "1"}))
	t.Cleanup(func() { tokenSvc = oldTokenSvc })

	for i := 0; i < unknownLoginFailureLimit; i++ {
		rec := postLoginCardForRateLimitTest(key, "203.0.113.70:5000")
		if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "卡密不存在") {
			t.Fatalf("attempt %d code=%d body=%s", i+1, rec.Code, rec.Body.String())
		}
	}
	blocked := postLoginCardForRateLimitTest(key, "203.0.113.70:5000")

	if blocked.Code != http.StatusTooManyRequests {
		t.Fatalf("blocked code=%d body=%s", blocked.Code, blocked.Body.String())
	}
	if got := blocked.Header().Get("Retry-After"); got == "" {
		t.Fatal("rate-limited login should include Retry-After")
	}
	if !strings.Contains(blocked.Body.String(), "登录尝试过多") {
		t.Fatalf("unexpected rate-limit body: %s", blocked.Body.String())
	}
	if got := searchHits.Load(); got != int32(unknownLoginFailureLimit) {
		t.Fatalf("rate-limited attempt should not hit upstream token search, got %d searches", got)
	}
	if _, ok := getCard(key); ok {
		t.Fatal("unknown card key should not be inserted")
	}
}

func TestHandleLoginUnknownLimitScopesByClientAndCardKey(t *testing.T) {
	setupScratchLockTestDB(t)
	t.Setenv("MCY_BASE_URL", "")
	t.Setenv("SHOP_BASE_URL", "")

	var searchHits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/token/search" {
			t.Fatalf("unexpected token request %s %s", r.Method, r.URL.String())
		}
		searchHits.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": []any{}})
	}))
	t.Cleanup(server.Close)
	oldTokenSvc := tokenSvc
	tokenSvc = tokens.NewService(newapi.NewClient(newapi.Site{URL: server.URL, Token: "token", UserID: "1"}))
	t.Cleanup(func() { tokenSvc = oldTokenSvc })

	keyA := "sk-unknown-scope-a-123"
	keyB := "sk-unknown-scope-b-123"
	for i := 0; i < unknownLoginFailureLimit; i++ {
		rec := postLoginCardForRateLimitTest(keyA, "203.0.113.71:5000")
		if rec.Code != http.StatusNotFound {
			t.Fatalf("attempt %d code=%d body=%s", i+1, rec.Code, rec.Body.String())
		}
	}
	if rec := postLoginCardForRateLimitTest(keyA, "203.0.113.71:5000"); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("key A should be limited for first client, code=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec := postLoginCardForRateLimitTest(keyB, "203.0.113.71:5000"); rec.Code != http.StatusNotFound {
		t.Fatalf("key B should not be limited by key A, code=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec := postLoginCardForRateLimitTest(keyA, "198.51.100.44:5000"); rec.Code != http.StatusNotFound {
		t.Fatalf("another client should not be limited by first client, code=%d body=%s", rec.Code, rec.Body.String())
	}
	wantSearches := int32(unknownLoginFailureLimit + 2)
	if got := searchHits.Load(); got != wantSearches {
		t.Fatalf("upstream searches = %d, want %d", got, wantSearches)
	}
}

func TestHandleLoginLocalCardBypassesUnknownLimit(t *testing.T) {
	setupScratchLockTestDB(t)
	t.Setenv("MCY_BASE_URL", "")
	t.Setenv("SHOP_BASE_URL", "")

	key := "sk-local-known-login-card-123"
	var searchHits atomic.Int32
	var returnKnown atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		searchHits.Add(1)
		if returnKnown.Load() {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"data": []any{map[string]any{
					"id":             41,
					"key":            key,
					"name":           "100-act-test",
					"interval_quota": newapi.DefaultQuotaUnit * 100,
					"status":         1,
					"created_time":   actStartTS + 1,
				}},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": []any{}})
	}))
	t.Cleanup(server.Close)
	oldTokenSvc := tokenSvc
	tokenSvc = tokens.NewService(newapi.NewClient(newapi.Site{URL: server.URL, Token: "token", UserID: "1"}))
	t.Cleanup(func() { tokenSvc = oldTokenSvc })

	for i := 0; i < unknownLoginFailureLimit; i++ {
		rec := postLoginCardForRateLimitTest(key, "203.0.113.72:5000")
		if rec.Code != http.StatusNotFound {
			t.Fatalf("attempt %d code=%d body=%s", i+1, rec.Code, rec.Body.String())
		}
	}
	if _, err := db.Exec(`INSERT INTO cards (card_key, card_name, dollars, total_spins) VALUES (?,?,?,?)`, key, "local-card", 100, 3); err != nil {
		t.Fatal(err)
	}
	returnKnown.Store(true)

	rec := postLoginCardForRateLimitTest(key, "203.0.113.72:5000")

	if rec.Code != http.StatusOK {
		t.Fatalf("local card should bypass unknown-card limiter, code=%d body=%s", rec.Code, rec.Body.String())
	}
	wantSearches := int32(unknownLoginFailureLimit + 1)
	if got := searchHits.Load(); got != wantSearches {
		t.Fatalf("local card login should bypass unknown-card limiter and only revalidate once, got %d searches", got)
	}
}

func postLoginCardForRateLimitTest(key, remoteAddr string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"cardKey":"`+key+`"}`))
	req.RemoteAddr = remoteAddr
	rec := httptest.NewRecorder()
	handleLogin(rec, req)
	return rec
}
