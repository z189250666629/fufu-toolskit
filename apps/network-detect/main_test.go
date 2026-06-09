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
