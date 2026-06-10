package newapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
