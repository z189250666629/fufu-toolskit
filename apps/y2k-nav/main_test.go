package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultPortConstant(t *testing.T) {
	if defaultPort == "" {
		t.Fatal("default port should be set")
	}
}

func TestStaticHandlerServesRootIndex(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "index.html"), []byte("y2k home"), 0644); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	newStaticHandler(root).ServeHTTP(w, req)

	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "y2k home") {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestStaticHandlerRejectsUnsafeOrWrongMethodRequests(t *testing.T) {
	root := t.TempDir()
	for _, tc := range []struct {
		name   string
		method string
		path   string
		status int
	}{
		{name: "post", method: http.MethodPost, path: "/", status: http.StatusMethodNotAllowed},
		{name: "traversal", method: http.MethodGet, path: "/..%2fsecret.txt", status: http.StatusForbidden},
		{name: "missing", method: http.MethodGet, path: "/missing.html", status: http.StatusNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			w := httptest.NewRecorder()
			newStaticHandler(root).ServeHTTP(w, req)
			if w.Code != tc.status {
				t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
			}
		})
	}
}
