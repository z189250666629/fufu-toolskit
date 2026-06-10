package main

import (
	"net"
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

func TestResolvePortUsesDefaultWhenEmpty(t *testing.T) {
	port, err := resolvePort(" \t ")
	if err != nil {
		t.Fatalf("resolvePort returned error: %v", err)
	}
	if port != defaultPort {
		t.Fatalf("port = %q, want default %q", port, defaultPort)
	}
}

func TestResolvePortTrimsValidPort(t *testing.T) {
	port, err := resolvePort(" 18820 ")
	if err != nil {
		t.Fatalf("resolvePort returned error: %v", err)
	}
	if port != "18820" {
		t.Fatalf("port = %q, want 18820", port)
	}
}

func TestResolvePortRejectsInvalidPort(t *testing.T) {
	for _, value := range []string{"abc", "0", "-1", "65536", "80/tcp"} {
		t.Run(value, func(t *testing.T) {
			if port, err := resolvePort(value); err == nil {
				t.Fatalf("resolvePort(%q) = %q, nil; want error", value, port)
			}
		})
	}
}

func TestServeReturnsListenerErrors(t *testing.T) {
	listener, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	_, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	err = serve(t.TempDir(), port)

	if err == nil {
		t.Fatal("serve should return bind errors instead of panicking")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "bind") && !strings.Contains(strings.ToLower(err.Error()), "address already in use") {
		t.Fatalf("serve returned unexpected error: %v", err)
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

func TestStaticHandlerDisablesHTMLCaching(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "index.html"), []byte("y2k home"), 0644); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	newStaticHandler(root).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q", got)
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
