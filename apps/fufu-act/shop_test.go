package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

	if got := findShopPurchase("card-1"); got != "" {
		t.Fatalf("missing purchase_time = %q, want blank", got)
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
	if _, err := mcyPost("/check", map[string]any{"ok": true}); err == nil {
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

	data, err := mcyPost("/check", map[string]any{"ok": true})
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "json") {
		t.Fatalf("data=%#v err=%v", data, err)
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

	err := mcyLogin()
	if err == nil || !strings.Contains(err.Error(), "500") {
		t.Fatalf("expected HTTP 500 login error, got %v", err)
	}
	if mcyCookie != "" {
		t.Fatalf("mcyCookie should stay empty on login failure, got %q", mcyCookie)
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
