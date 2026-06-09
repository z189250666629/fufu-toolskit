package main

import (
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
