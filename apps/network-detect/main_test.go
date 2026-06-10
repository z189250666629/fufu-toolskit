package main

import (
	"fufu/combine"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHealth(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()
	handleAPI(w, req)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"ok":true`) {
		t.Fatalf("health code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestCombinePageServed(t *testing.T) {
	tmp := t.TempDir()
	rootDir = tmp
	frontendDir = filepath.Join(tmp, "frontend")
	combineDir = filepath.Join(tmp, "combine")
	if err := os.MkdirAll(combineDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(combineDir, "index.html"), []byte("combine page"), 0644); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/combine", nil)
	w := httptest.NewRecorder()
	route(w, req)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "combine page") {
		t.Fatalf("combine page code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestCombineAPIRoutes(t *testing.T) {
	for _, path := range []string{
		"/api/auth",
		"/api/session",
		"/api/search-keys",
		"/api/merge",
		"/api/public-merge",
		"/api/generate",
		"/api/merge-status/job-1",
		"/api/token/1",
	} {
		if !isCombineAPI(path) {
			t.Fatalf("%s should be routed to combine app", path)
		}
	}
	if isCombineAPI("/api/health") {
		t.Fatalf("network health endpoint should not be routed to combine app")
	}
	for _, path := range []string{"/api/auth", "/api/session", "/api/search-keys", "/api/merge", "/api/public-merge", "/api/generate", "/api/merge-status/job-1", "/api/token/1", "/api/health"} {
		if isCombineAPI(path) != combine.IsAPIPath(path) {
			t.Fatalf("%s route mismatch: network=%v combine=%v", path, isCombineAPI(path), combine.IsAPIPath(path))
		}
	}
}

func TestNetworkAPIRouteMethods(t *testing.T) {
	cases := []struct {
		path   string
		method string
	}{
		{path: "/api/health", method: http.MethodGet},
		{path: "/api/client", method: http.MethodGet},
		{path: "/api/connectivity/targets", method: http.MethodGet},
		{path: "/api/newapi/sites", method: http.MethodGet},
		{path: "/api/newapi/model-status", method: http.MethodGet},
		{path: "/api/newapi/overview", method: http.MethodGet},
		{path: "/api/newapi/model-status/test", method: http.MethodPost},
	}

	for _, tc := range cases {
		route, ok := findNetworkAPIPath(tc.path)
		if !ok {
			t.Fatalf("%s should be a network API route", tc.path)
		}
		if route.Method != tc.method {
			t.Fatalf("%s method=%s, want %s", tc.path, route.Method, tc.method)
		}
	}
	for _, path := range []string{"/api/auth", "/api/merge-status/job-1", "/api/unknown"} {
		if _, ok := findNetworkAPIPath(path); ok {
			t.Fatalf("%s should not be a network-owned API route", path)
		}
	}
}

func TestHandleAPIRejectsWrongNetworkMethodBeforeHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/newapi/sites", nil)
	w := httptest.NewRecorder()

	handleAPI(w, req)

	if w.Code != http.StatusMethodNotAllowed || strings.TrimSpace(w.Body.String()) != `{"error":"Only GET is supported"}` {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestWriteJSONErrorUsesStablePayload(t *testing.T) {
	w := httptest.NewRecorder()

	writeJSONError(w, http.StatusTeapot, "bad request")

	if w.Code != http.StatusTeapot || strings.TrimSpace(w.Body.String()) != `{"error":"bad request"}` {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestRequireMethodUsesNetworkMethodMessage(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/newapi/model-status/test", nil)

	if requireMethod(w, req, http.MethodPost) {
		t.Fatal("GET should be rejected when POST is required")
	}
	if w.Code != http.StatusMethodNotAllowed || strings.TrimSpace(w.Body.String()) != `{"error":"Only POST is supported"}` {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestParseListCleansSortsAndDedupes(t *testing.T) {
	got := parseList(`["beta","alpha","beta",""]`)
	if strings.Join(got, ",") != "alpha,beta" {
		t.Fatalf("json parseList = %#v", got)
	}

	got = parseList(" beta,alpha beta|gamma ")
	if strings.Join(got, ",") != "alpha,beta,gamma" {
		t.Fatalf("text parseList = %#v", got)
	}
}

func TestModelStatusClassifiers(t *testing.T) {
	if got := statusFromCounts(0, 0); got != "unknown" {
		t.Fatalf("empty status = %s", got)
	}
	if got := statusFromCounts(4, 1); got != "operational" {
		t.Fatalf("mostly successful status = %s", got)
	}
	if got := statusFromCounts(1, 3); got != "degraded" {
		t.Fatalf("mixed status = %s", got)
	}
	if got := statusFromCounts(0, 3); got != "down" {
		t.Fatalf("failed status = %s", got)
	}

	rowStatus := modelRowStatus([]*ModelCell{{Configured: true, Status: "operational"}, {Configured: true, Status: "degraded"}})
	if rowStatus != "degraded" {
		t.Fatalf("row status = %s", rowStatus)
	}
}
