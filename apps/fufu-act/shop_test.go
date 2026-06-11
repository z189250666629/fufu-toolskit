package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func newJSONServer(t *testing.T, payload any) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewEncoder(w).Encode(payload); err != nil {
			t.Fatalf("encode payload: %v", err)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestFindShopPurchaseReturnsBlankWhenPurchaseTimeMissing(t *testing.T) {
	oldCookie := mcyCookie
	t.Cleanup(func() { mcyCookie = oldCookie })
	mcyCookie = "session=ok"

	srv := newJSONServer(t, map[string]any{"data": map[string]any{"list": []any{map[string]any{}}}})
	t.Setenv("MCY_BASE_URL", srv.URL)

	if got, err := findShopPurchase(context.Background(), "card-1"); err != nil || got.PurchaseTime != "" {
		t.Fatalf("missing purchase_time = %#v err=%v, want blank", got, err)
	}
}

func TestFindShopPurchaseClassifiesLoginFailure(t *testing.T) {
	oldCookie := mcyCookie
	t.Cleanup(func() { mcyCookie = oldCookie })
	mcyCookie = ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/admin/login" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`login failed`))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("MCY_BASE_URL", srv.URL)
	t.Setenv("MCY_USERNAME", "u")
	t.Setenv("MCY_PASSWORD", "p")

	purchase, err := findShopPurchase(context.Background(), "card-1")
	if err == nil || !errors.Is(err, ErrShopLoginFailed) {
		t.Fatalf("purchase=%#v err=%v, want ErrShopLoginFailed", purchase, err)
	}
	if purchase.PurchaseTime != "" {
		t.Fatalf("purchase time = %q, want empty on login failure", purchase.PurchaseTime)
	}
}

func TestFindShopPurchaseClassifiesQueryHTTPFailure(t *testing.T) {
	oldCookie := mcyCookie
	t.Cleanup(func() { mcyCookie = oldCookie })
	mcyCookie = "session=ok"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/plugin/virtual-card-ship/card/get" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "upstream down"})
	}))
	t.Cleanup(srv.Close)
	t.Setenv("MCY_BASE_URL", srv.URL)

	purchase, err := findShopPurchase(context.Background(), "card-1")
	if err == nil || !errors.Is(err, ErrShopRequestFailed) {
		t.Fatalf("purchase=%#v err=%v, want ErrShopRequestFailed", purchase, err)
	}
	if purchase.PurchaseTime != "" {
		t.Fatalf("purchase time = %q, want empty on query failure", purchase.PurchaseTime)
	}
}

func TestFindShopPurchaseClassifiesInvalidJSONResponse(t *testing.T) {
	oldCookie := mcyCookie
	t.Cleanup(func() { mcyCookie = oldCookie })
	mcyCookie = "session=ok"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/plugin/virtual-card-ship/card/get" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
		_, _ = w.Write([]byte(`not-json`))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("MCY_BASE_URL", srv.URL)

	purchase, err := findShopPurchase(context.Background(), "card-1")
	if err == nil || !errors.Is(err, ErrShopInvalidResponse) {
		t.Fatalf("purchase=%#v err=%v, want ErrShopInvalidResponse", purchase, err)
	}
	if purchase.PurchaseTime != "" {
		t.Fatalf("purchase time = %q, want empty on invalid JSON", purchase.PurchaseTime)
	}
}

func TestExtractPurchaseTimeTrimsAndDropsBlankValues(t *testing.T) {
	if got := extractPurchaseTime(map[string]any{"list": []any{map[string]any{"purchase_time": " 2026-06-10 10:00:00 "}}}); got != "2026-06-10 10:00:00" {
		t.Fatalf("trimmed purchase_time = %q", got)
	}
	if got := extractPurchaseTime(map[string]any{"list": []any{map[string]any{"purchase_time": "   "}}}); got != "" {
		t.Fatalf("blank purchase_time = %q, want blank", got)
	}
}

func TestFindShopPurchaseRefreshesExpiredCookie(t *testing.T) {
	oldCookie := mcyCookie
	t.Cleanup(func() { mcyCookie = oldCookie })
	mcyCookie = "mcy_session=expired"

	postAttempts := 0
	loginAttempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/admin/login":
			loginAttempts++
			http.SetCookie(w, &http.Cookie{Name: "mcy_session", Value: "fresh"})
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
		case r.Method == http.MethodPost && r.URL.Path == "/plugin/virtual-card-ship/card/get":
			postAttempts++
			if r.Header.Get("Cookie") != "mcy_session=fresh" {
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]any{"error": "expired"})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"list": []any{map[string]any{"purchase_time": "2026-06-10 12:00:00"}}}})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("MCY_BASE_URL", srv.URL)
	t.Setenv("MCY_USERNAME", "u")
	t.Setenv("MCY_PASSWORD", "p")

	got, err := findShopPurchase(context.Background(), "card-1")

	if err != nil || got.PurchaseTime != "2026-06-10 12:00:00" {
		t.Fatalf("purchase = %#v err=%v", got, err)
	}
	if postAttempts != 2 || loginAttempts != 1 {
		t.Fatalf("postAttempts=%d loginAttempts=%d", postAttempts, loginAttempts)
	}
}

func TestFindShopPurchaseConcurrentAuthRefreshIsRaceFree(t *testing.T) {
	oldCookie := mcyCookie
	t.Cleanup(func() { mcyCookie = oldCookie })
	mcyCookie = ""

	var loginAttempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/admin/login":
			loginAttempts.Add(1)
			time.Sleep(5 * time.Millisecond)
			http.SetCookie(w, &http.Cookie{Name: "mcy_session", Value: "fresh"})
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
		case r.Method == http.MethodPost && r.URL.Path == "/plugin/virtual-card-ship/card/get":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"list": []any{map[string]any{"purchase_time": "2026-06-10 12:00:00"}}}})
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.String())
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("MCY_BASE_URL", srv.URL)
	t.Setenv("MCY_USERNAME", "u")
	t.Setenv("MCY_PASSWORD", "p")

	var wg sync.WaitGroup
	errs := make(chan error, 12)
	for i := 0; i < cap(errs); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := findShopPurchase(context.Background(), "card-1")
			if err != nil {
				errs <- err
				return
			}
			if got.PurchaseTime != "2026-06-10 12:00:00" {
				errs <- errors.New("unexpected purchase time: " + got.PurchaseTime)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	if got := loginAttempts.Load(); got != 1 {
		t.Fatalf("concurrent empty-cookie lookups should share one auth refresh, got %d login attempts", got)
	}
}

func TestMCYPostReturnsErrorForInvalidRequestURL(t *testing.T) {
	oldCookie := mcyCookie
	t.Cleanup(func() { mcyCookie = oldCookie })
	mcyCookie = "session=ok"
	t.Setenv("MCY_BASE_URL", ":// bad-url")

	defer func() {
		if x := recover(); x != nil {
			t.Fatalf("mcyPost should return an error, not panic: %v", x)
		}
	}()
	if _, err := mcyPost(context.Background(), "/check", map[string]any{"ok": true}); err == nil {
		t.Fatal("mcyPost should reject invalid request URLs")
	}
}

func TestMCYPostReturnsErrorForInvalidJSONResponse(t *testing.T) {
	oldCookie := mcyCookie
	t.Cleanup(func() { mcyCookie = oldCookie })
	mcyCookie = "session=ok"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`not-json`))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("MCY_BASE_URL", srv.URL)

	data, err := mcyPost(context.Background(), "/check", map[string]any{"ok": true})
	if err == nil || !errors.Is(err, ErrShopInvalidResponse) {
		t.Fatalf("data=%#v err=%v", data, err)
	}
}

func TestDecodeMCYResponseRejectsTrailingJSON(t *testing.T) {
	var data map[string]any
	err := decodeMCYResponse(strings.NewReader(`{"data":{"list":[]}}{"extra":true}`), &data)
	if err == nil || !errors.Is(err, ErrShopInvalidResponse) {
		t.Fatalf("data=%#v err=%v, want ErrShopInvalidResponse", data, err)
	}
}

func TestMCYPostRejectsOversizedResponseBody(t *testing.T) {
	oldCookie := mcyCookie
	t.Cleanup(func() { mcyCookie = oldCookie })
	mcyCookie = "session=ok"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":"`))
		_, _ = w.Write([]byte(strings.Repeat("x", (16<<20)+1)))
		_, _ = w.Write([]byte(`"}`))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("MCY_BASE_URL", srv.URL)

	data, err := mcyPost(context.Background(), "/check", map[string]any{"ok": true})
	if err == nil || !errors.Is(err, ErrShopInvalidResponse) {
		t.Fatalf("dataPresent=%v err=%v, want ErrShopInvalidResponse", data != nil, err)
	}
	if !strings.Contains(err.Error(), "response body too large") {
		t.Fatalf("unexpected oversized response error: %v", err)
	}
}

func TestMCYLoginReturnsErrorForHTTPFailure(t *testing.T) {
	oldCookie := mcyCookie
	t.Cleanup(func() { mcyCookie = oldCookie })
	mcyCookie = ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "mcy_session", Value: "bad"})
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`login failed`))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("MCY_BASE_URL", srv.URL)
	t.Setenv("MCY_USERNAME", "u")
	t.Setenv("MCY_PASSWORD", "p")

	err := mcyLogin(context.Background())
	if err == nil || !strings.Contains(err.Error(), "500") {
		t.Fatalf("expected HTTP 500 login error, got %v", err)
	}
	if mcyCookie != "" {
		t.Fatalf("mcyCookie should stay empty on login failure, got %q", mcyCookie)
	}
}

func TestMCYLoginFallsBackToEncryptedAdminEndpointToken(t *testing.T) {
	oldCookie := mcyCookie
	t.Cleanup(func() { mcyCookie = oldCookie })
	mcyCookie = ""
	var jsonLoginHits, encryptedLoginHits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/admin/login":
			jsonLoginHits++
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 404, "msg": "not found"})
		case r.Method == http.MethodPost && r.URL.Path == "/admin":
			encryptedLoginHits++
			payload := testDecodeMCYRequest(t, r.Body, r.Header.Get("Secret"))
			if payload["email"] != "user@example.test" || payload["password"] != "pass" {
				t.Fatalf("encrypted login payload=%#v", payload)
			}
			testWriteEncryptedMCYResponse(t, w, map[string]any{"code": 200, "data": map[string]any{"token": "fresh-token"}})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("MCY_BASE_URL", srv.URL)
	t.Setenv("MCY_USERNAME", "user@example.test")
	t.Setenv("MCY_PASSWORD", "pass")
	t.Setenv("MCY_LOGIN_ENDPOINT", "/admin/login")

	if err := mcyLogin(context.Background()); err != nil {
		t.Fatalf("mcyLogin error: %v", err)
	}
	if got := getMCYCookie(); got != "manage_token=fresh-token" {
		t.Fatalf("mcyCookie=%q", got)
	}
	if jsonLoginHits != 1 || encryptedLoginHits != 1 {
		t.Fatalf("jsonLoginHits=%d encryptedLoginHits=%d", jsonLoginHits, encryptedLoginHits)
	}
}

func TestMCYHTTPClientUsesFiniteTimeout(t *testing.T) {
	if mcyHTTPClient == nil {
		t.Fatal("mcyHTTPClient is nil")
	}
	if mcyHTTPClient.Timeout <= 0 || mcyHTTPClient.Timeout > 30*time.Second {
		t.Fatalf("mcyHTTPClient timeout = %s", mcyHTTPClient.Timeout)
	}
}
