package newapi

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestBuildHTTPRequestAppliesURLHeadersAndJSONBody(t *testing.T) {
	client := NewClient(Site{URL: "https://api.example.test/", Token: "secret", UserID: "7"})

	req, err := buildHTTPRequest(context.Background(), client, "post", "api/token", map[string]any{"name": "card"})
	if err != nil {
		t.Fatalf("buildHTTPRequest: %v", err)
	}

	if req.Method != http.MethodPost || req.URL.String() != "https://api.example.test/api/token" {
		t.Fatalf("request target = %s %s", req.Method, req.URL.String())
	}
	if req.Header.Get("Authorization") != "Bearer secret" || req.Header.Get("New-Api-User") != "7" {
		t.Fatalf("headers = %#v", req.Header)
	}
	if req.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("content type = %q", req.Header.Get("Content-Type"))
	}
	body, _ := io.ReadAll(req.Body)
	if !strings.Contains(string(body), `"name":"card"`) {
		t.Fatalf("body = %s", body)
	}
}

func TestBuildHTTPRequestSkipsJSONContentTypeForGet(t *testing.T) {
	client := NewClient(Site{URL: "https://api.example.test", SkipUserHeader: true})

	req, err := buildHTTPRequest(context.Background(), client, http.MethodGet, "/api/status", nil)
	if err != nil {
		t.Fatalf("buildHTTPRequest: %v", err)
	}

	if req.Header.Get("Content-Type") != "" || req.Header.Get("New-Api-User") != "" {
		t.Fatalf("headers = %#v", req.Header)
	}
}

func TestBuildHTTPRequestHonorsConnectionCloseContext(t *testing.T) {
	client := NewClient(Site{URL: "https://api.example.test"})

	req, err := buildHTTPRequest(WithConnectionClose(context.Background()), client, http.MethodGet, "/api/status", nil)
	if err != nil {
		t.Fatalf("buildHTTPRequest: %v", err)
	}

	if !req.Close || req.Header.Get("Connection") != "close" {
		t.Fatalf("request should close connection, close=%v headers=%#v", req.Close, req.Header)
	}
}
