package main

import (
	"fufu/webutil"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestDefaultPortConstant(t *testing.T) {
	if defaultPort == "" {
		t.Fatal("default port should be set")
	}
}

func TestResolvePortUsesDefaultWhenEmpty(t *testing.T) {
	port, err := webutil.ResolvePort(" \t ", defaultPort)
	if err != nil {
		t.Fatalf("resolvePort returned error: %v", err)
	}
	if port != defaultPort {
		t.Fatalf("port = %q, want default %q", port, defaultPort)
	}
}

func TestResolvePortTrimsValidPort(t *testing.T) {
	port, err := webutil.ResolvePort(" 18820 ", defaultPort)
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
			if port, err := webutil.ResolvePort(value, defaultPort); err == nil {
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

func TestHTTPServerHasTimeouts(t *testing.T) {
	server := newHTTPServer("33148", http.NewServeMux())
	if server.Addr != "0.0.0.0:33148" {
		t.Fatalf("Addr = %q", server.Addr)
	}
	if server.ReadHeaderTimeout <= 0 || server.ReadTimeout <= 0 || server.WriteTimeout <= 0 || server.IdleTimeout <= 0 {
		t.Fatalf("server timeouts should be set: %#v", server)
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

func TestStaticHandlerServesOnlyBrowserAssets(t *testing.T) {
	root := t.TempDir()
	allowed := map[string]string{
		"/":            "index marker",
		"/index.html":  "index marker",
		"/theme.mjs":   "theme marker",
		"/latency.mjs": "latency marker",
	}
	for path, marker := range map[string]string{
		"index.html":             "index marker",
		"theme.mjs":              "theme marker",
		"latency.mjs":            "latency marker",
		"main.go":                "secret source marker",
		"main_test.go":           "secret test marker",
		"Dockerfile":             "secret docker marker",
		"package.json":           "secret package marker",
		"go.mod":                 "secret gomod marker",
		"docker_assets.test.mjs": "secret docker test marker",
		"theme.test.mjs":         "secret theme test marker",
		"latency.test.mjs":       "secret latency test marker",
		"y2k-nav":                "secret binary marker",
		"y2k-nav.exe":            "secret windows binary marker",
		"tests/snapshot.txt":     "secret nested test marker",
		"src/debug.txt":          "secret nested source marker",
		".env.local":             "secret env marker",
	} {
		fullPath := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte(marker), 0644); err != nil {
			t.Fatal(err)
		}
	}
	handler := newStaticHandler(root)

	for path, marker := range allowed {
		t.Run("allow "+path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), marker) {
				t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
			}
		})
	}
	for _, path := range []string{"/main.go", "/main_test.go", "/Dockerfile", "/package.json", "/go.mod", "/docker_assets.test.mjs", "/theme.test.mjs", "/latency.test.mjs", "/y2k-nav", "/y2k-nav.exe", "/tests/snapshot.txt", "/src/debug.txt", "/.env.local"} {
		t.Run("block "+path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			if w.Code == http.StatusOK {
				t.Fatalf("%s should not be served from runtime root: code=%d body=%s", path, w.Code, w.Body.String())
			}
			if strings.Contains(w.Body.String(), "secret ") {
				t.Fatalf("%s leaked blocked file content: %s", path, w.Body.String())
			}
		})
	}
}

func TestPublicBrowserAssetAllowlistIsDataDriven(t *testing.T) {
	raw, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "switch path") {
		t.Fatal("browser asset allowlist should be a data table so tests can cover it without duplicating switch cases")
	}
}

func TestPublicBrowserAssetAllowlistCoversIndexModules(t *testing.T) {
	raw, err := os.ReadFile("index.html")
	if err != nil {
		t.Fatal(err)
	}
	matches := regexp.MustCompile(`(?:src|from)=?\s*["']\.?/([^"']+\.mjs)["']`).FindAllStringSubmatch(string(raw), -1)
	if len(matches) == 0 {
		t.Fatal("index.html should reference module scripts")
	}
	for _, match := range matches {
		path := "/" + strings.TrimPrefix(match[1], "/")
		if !isPublicBrowserAsset(path) {
			t.Fatalf("index module %s is not allowed by the Go static handler", path)
		}
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
			if got := w.Header().Get("Cache-Control"); got != "no-store" {
				t.Fatalf("Cache-Control = %q", got)
			}
			if tc.status == http.StatusMethodNotAllowed {
				if got := w.Header().Get("Allow"); got != "GET, HEAD" {
					t.Fatalf("Allow = %q", got)
				}
			}
		})
	}
}
