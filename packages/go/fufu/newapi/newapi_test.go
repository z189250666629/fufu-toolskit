package newapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestAddsAuthHeadersAndParsesJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Fatalf("Authorization = %q", got)
		}
		if got := r.Header.Get("New-Api-User"); got != "7" {
			t.Fatalf("New-Api-User = %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": []any{map[string]any{"ok": true}}})
	}))
	defer server.Close()
	client := NewClient(Site{URL: server.URL, Token: "secret", UserID: "7"})
	res, data, err := client.Get(context.Background(), "/api/test")
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK() {
		t.Fatalf("response not OK: %+v", res)
	}
	if data["success"] != true {
		t.Fatalf("decoded success = %#v", data["success"])
	}
}

func TestPublicSiteHidesRawURLAndToken(t *testing.T) {
	public := Site{Name: "private-site", URL: "http://10.0.0.5:3000/admin", Token: "sk-private", UserID: "1"}.Public()
	raw, err := json.Marshal(public)
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)

	for _, leaked := range []string{"http://10.0.0.5:3000/admin", "10.0.0.5", "3000", "sk-private"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("public site leaked %q in %s", leaked, body)
		}
	}
	if public.DisplayURL == "" {
		t.Fatalf("public site should expose a safe display label: %#v", public)
	}
}

func TestRequestKeepsHTTPErrorBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"forbidden"}`))
	}))
	defer server.Close()
	client := NewClient(Site{URL: server.URL, Token: "secret", UserID: "1"})
	res, data, err := client.Get(context.Background(), "/api/test")
	if err != nil {
		t.Fatal(err)
	}
	if res.OK() {
		t.Fatalf("expected non-OK")
	}
	if data["error"] != "forbidden" {
		t.Fatalf("decoded error = %#v", data["error"])
	}
}

func TestRequestDoesNotTreatInvalidJSONAsSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html>bad gateway</html>`))
	}))
	defer server.Close()

	client := NewClient(Site{URL: server.URL, Token: "secret", UserID: "1"})
	res, data, err := client.Get(context.Background(), "/api/test")

	if err == nil {
		t.Fatalf("expected invalid JSON error, got response=%+v data=%#v", res, data)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status code = %d", res.StatusCode)
	}
	if data != nil {
		t.Fatalf("invalid JSON should not return success payload: %#v", data)
	}
}

func TestRequestRejectsOversizedResponseBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":"`))
		_, _ = w.Write([]byte(strings.Repeat("x", (16<<20)+1)))
		_, _ = w.Write([]byte(`"}`))
	}))
	defer server.Close()

	client := NewClient(Site{URL: server.URL, Token: "secret", UserID: "1"})
	res, data, err := client.Get(context.Background(), "/api/test")

	if err == nil {
		t.Fatalf("expected oversized response error, got status=%d bodyBytes=%d data=%#v", res.StatusCode, len(res.Body), data)
	}
	if !strings.Contains(err.Error(), "response body too large") {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status code = %d", res.StatusCode)
	}
	if res.Body != "" {
		t.Fatalf("oversized body should not be retained, got %d bytes", len(res.Body))
	}
	if data != nil {
		t.Fatalf("oversized response should not return payload: %#v", data)
	}
}

func TestSuccessFalseErrorMessage(t *testing.T) {
	data := map[string]any{"success": false, "message": "bad token"}
	if IsSuccess(data) {
		t.Fatalf("success=false should be parsed as failed")
	}
	if got := ErrorMessage(data, 200, "fallback"); got != "bad token" {
		t.Fatalf("ErrorMessage = %q", got)
	}
}

func TestErrorMessageUsesFallbackForHTTP200WithoutPayloadMessage(t *testing.T) {
	data := map[string]any{"success": false}

	if got := ErrorMessage(data, 200, "upstream rejected request"); got != "upstream rejected request" {
		t.Fatalf("ErrorMessage = %q", got)
	}
}
