package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"fufu/newapi"
	"fufu/tokens"
)

func TestHandleLoginDoesNotTreatContestNameAsActTest(t *testing.T) {
	setupScratchLockTestDB(t)
	t.Setenv("MCY_BASE_URL", "")
	t.Setenv("SHOP_BASE_URL", "")

	key := "sk-contest-card-123"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/token/search" {
			t.Fatalf("unexpected token request %s %s", r.Method, r.URL.String())
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data": []any{map[string]any{
				"id":             7,
				"key":            key,
				"name":           "100-act-contest",
				"interval_quota": newapi.DefaultQuotaUnit * 100,
				"status":         1,
				"created_time":   actStartTS - 1,
			}},
		})
	}))
	t.Cleanup(server.Close)

	oldTokenSvc := tokenSvc
	tokenSvc = tokens.NewService(newapi.NewClient(newapi.Site{URL: server.URL, Token: "token", UserID: "1"}))
	t.Cleanup(func() { tokenSvc = oldTokenSvc })

	req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"cardKey":"`+key+`"}`))
	w := httptest.NewRecorder()
	handleLogin(w, req)

	if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "不在活动期间") {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestHandleLoginRejectsDisabledToken(t *testing.T) {
	setupScratchLockTestDB(t)
	t.Setenv("MCY_BASE_URL", "")
	t.Setenv("SHOP_BASE_URL", "")

	key := "sk-disabled-card-123"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/token/search" {
			t.Fatalf("unexpected token request %s %s", r.Method, r.URL.String())
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data": []any{map[string]any{
				"id":             8,
				"key":            key,
				"name":           "100-act-disabled",
				"interval_quota": newapi.DefaultQuotaUnit * 100,
				"status":         2,
				"created_time":   actStartTS + 1,
			}},
		})
	}))
	t.Cleanup(server.Close)

	oldTokenSvc := tokenSvc
	tokenSvc = tokens.NewService(newapi.NewClient(newapi.Site{URL: server.URL, Token: "token", UserID: "1"}))
	t.Cleanup(func() { tokenSvc = oldTokenSvc })

	req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"cardKey":"`+key+`"}`))
	w := httptest.NewRecorder()
	handleLogin(w, req)

	if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "已被禁用") {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	if _, ok := getCard(key); ok {
		t.Fatal("disabled token should not be inserted as an activity card")
	}
}

func TestHandleLoginRequiresExactScratchDollarTier(t *testing.T) {
	setupScratchLockTestDB(t)
	t.Setenv("MCY_BASE_URL", "")
	t.Setenv("SHOP_BASE_URL", "")

	key := "sk-near-scratch-card-123"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/token/search" {
			t.Fatalf("unexpected token request %s %s", r.Method, r.URL.String())
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data": []any{map[string]any{
				"id":             9,
				"key":            key,
				"name":           "55.4-act-test",
				"interval_quota": newapi.DefaultQuotaUnit * 55,
				"status":         1,
				"created_time":   actStartTS + 1,
			}},
		})
	}))
	t.Cleanup(server.Close)

	oldTokenSvc := tokenSvc
	tokenSvc = tokens.NewService(newapi.NewClient(newapi.Site{URL: server.URL, Token: "token", UserID: "1"}))
	t.Cleanup(func() { tokenSvc = oldTokenSvc })

	req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"cardKey":"`+key+`"}`))
	w := httptest.NewRecorder()
	handleLogin(w, req)

	if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "额度不参与活动") {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	if _, ok := getCard(key); ok {
		t.Fatal("near-55 token should not be inserted as a scratch activity card")
	}
}

func TestHandleLoginMasksTokenConfigErrorDetails(t *testing.T) {
	setupScratchLockTestDB(t)
	oldTokenSvc := tokenSvc
	oldTokenConfigErr := tokenConfigErr
	tokenSvc = nil
	tokenConfigErr = errors.New("NEWAPI_MANAGED_API_SITES 不是有效 JSON: secret-dsn")
	t.Cleanup(func() {
		tokenSvc = oldTokenSvc
		tokenConfigErr = oldTokenConfigErr
	})

	req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"cardKey":"sk-config-card-123"}`))
	w := httptest.NewRecorder()
	handleLogin(w, req)

	body := w.Body.String()
	if w.Code != http.StatusServiceUnavailable || !strings.Contains(body, "NewAPI 未配置") {
		t.Fatalf("code=%d body=%s", w.Code, body)
	}
	if strings.Contains(body, "secret-dsn") || strings.Contains(body, "NEWAPI_MANAGED_API_SITES") {
		t.Fatalf("login response leaked config error details: %s", body)
	}
}

func TestHandleLoginMasksInternalTokenLookupError(t *testing.T) {
	setupScratchLockTestDB(t)
	t.Setenv("MCY_BASE_URL", "")
	t.Setenv("SHOP_BASE_URL", "")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "message": "postgres password=secret-token"})
	}))
	t.Cleanup(server.Close)

	oldTokenSvc := tokenSvc
	tokenSvc = tokens.NewService(newapi.NewClient(newapi.Site{URL: server.URL, Token: "token", UserID: "1"}))
	t.Cleanup(func() { tokenSvc = oldTokenSvc })

	req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"cardKey":"sk-lookup-card-123"}`))
	w := httptest.NewRecorder()
	handleLogin(w, req)

	body := w.Body.String()
	if w.Code != http.StatusInternalServerError || !strings.Contains(body, "服务器错误") {
		t.Fatalf("code=%d body=%s", w.Code, body)
	}
	if strings.Contains(body, "postgres") || strings.Contains(body, "secret-token") {
		t.Fatalf("login response leaked token lookup details: %s", body)
	}
}

func TestHandleLoginMasksCardInsertErrorDetails(t *testing.T) {
	setupScratchLockTestDB(t)
	t.Setenv("MCY_BASE_URL", "")
	t.Setenv("SHOP_BASE_URL", "")
	if _, err := db.Exec(`CREATE TRIGGER reject_cards_insert BEFORE INSERT ON cards BEGIN SELECT RAISE(ABORT, 'internal insert secret'); END;`); err != nil {
		t.Fatal(err)
	}

	key := "sk-insert-card-123"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data": []any{map[string]any{
				"id":             10,
				"key":            key,
				"name":           "100-act-test",
				"interval_quota": newapi.DefaultQuotaUnit * 100,
				"status":         1,
				"created_time":   actStartTS + 1,
			}},
		})
	}))
	t.Cleanup(server.Close)

	oldTokenSvc := tokenSvc
	tokenSvc = tokens.NewService(newapi.NewClient(newapi.Site{URL: server.URL, Token: "token", UserID: "1"}))
	t.Cleanup(func() { tokenSvc = oldTokenSvc })

	req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"cardKey":"`+key+`"}`))
	w := httptest.NewRecorder()
	handleLogin(w, req)

	body := w.Body.String()
	if w.Code != http.StatusInternalServerError || !strings.Contains(body, "服务器错误") {
		t.Fatalf("code=%d body=%s", w.Code, body)
	}
	if strings.Contains(body, "internal insert secret") || strings.Contains(body, "constraint") {
		t.Fatalf("login response leaked insert error details: %s", body)
	}
}

func TestParseActTestTokenNameRequiresIndependentTestSegment(t *testing.T) {
	dollars, ok := parseActTestTokenName("100-act-test")
	if !ok || dollars != 100 {
		t.Fatalf("valid act test parse = %v/%v", dollars, ok)
	}

	for _, name := range []string{"100-act-contest", "100-act-latest", "bad-act-test"} {
		if dollars, ok := parseActTestTokenName(name); ok || dollars != 0 {
			t.Fatalf("%q should not parse as act test: %v/%v", name, dollars, ok)
		}
	}
}
