package webutil

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSafePathStaysInsideRoot(t *testing.T) {
	root := t.TempDir()

	got, ok := SafePath(root, "/assets/app.js")
	if !ok {
		t.Fatalf("SafePath rejected a normal path")
	}
	want := filepath.Join(root, "assets", "app.js")
	if got != want {
		t.Fatalf("SafePath = %q, want %q", got, want)
	}

	if _, ok := SafePath(root, "/../secret.txt"); ok {
		t.Fatalf("SafePath allowed traversal outside root")
	}
}

func TestIsPublicStaticPathRejectsTestArtifactsAndDotfiles(t *testing.T) {
	for _, path := range []string{
		"/app.test.mjs",
		"/api.spec.mjs",
		"/__tests__/fixture.js",
		"/tests/snapshot.txt",
		"/testdata/secret.json",
		"/coverage/lcov.info",
		"/.env.local",
		"/assets/.secret/config.json",
	} {
		if IsPublicStaticPath(path) {
			t.Fatalf("%s should not be public static content", path)
		}
	}
	for _, path := range []string{
		"/index.html",
		"/assets/app.js",
		"/styles/models/results.css",
	} {
		if !IsPublicStaticPath(path) {
			t.Fatalf("%s should be public static content", path)
		}
	}
}

func TestReferencedBrowserAssetPathsFollowsLocalHTMLJSCSSReferences(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"index.html":      `<link rel="stylesheet" href="/styles.css"><script type="module" src="/app.js"></script><a href="https://example.test/ignored.css">x</a>`,
		"app.js":          `import { boot } from "./boot.js"; import "./side-effect.js"; const lazy = import("./lazy.js"); export { thing } from "./exported.js";`,
		"boot.js":         `export function boot() {}`,
		"side-effect.js":  `export const ok = true;`,
		"lazy.js":         `export const lazy = true;`,
		"exported.js":     `export const thing = true;`,
		"styles.css":      `@import url("./styles/base.css");`,
		"styles/base.css": `.app { color: green; }`,
		"debug.js":        `window.__debug = true;`,
	}
	for name, body := range files {
		file := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}

	allowed := ReferencedBrowserAssetPaths(root, []string{"/index.html"})

	for _, path := range []string{"/index.html", "/app.js", "/boot.js", "/side-effect.js", "/lazy.js", "/exported.js", "/styles.css", "/styles/base.css"} {
		if _, ok := allowed[path]; !ok {
			t.Fatalf("%s should be included in referenced browser asset allowlist: %#v", path, allowed)
		}
	}
	for _, path := range []string{"/debug.js", "https://example.test/ignored.css"} {
		if _, ok := allowed[path]; ok {
			t.Fatalf("%s should not be included in referenced browser asset allowlist", path)
		}
	}
}

func TestReferencedBrowserAssetPathsFollowsLocalCSSUrlAssets(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"index.html":          `<link rel="stylesheet" href="/styles/main.css">`,
		"styles/main.css":     `.logo { background: url("./img/logo.svg?v=1"); } .font { src: url('../fonts/app.woff2') format("woff2"); } .remote { background: url("https://example.test/logo.svg"); } .inline { background: url(data:image/svg+xml;base64,aaaa); } .custom { background: url(var(--brand-logo)); }`,
		"styles/img/logo.svg": `<svg></svg>`,
		"fonts/app.woff2":     "font bytes",
		"unlinked.svg":        `<svg></svg>`,
	}
	for name, body := range files {
		file := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}

	allowed := ReferencedBrowserAssetPaths(root, []string{"/index.html"})

	for _, path := range []string{"/styles/main.css", "/styles/img/logo.svg", "/fonts/app.woff2"} {
		if _, ok := allowed[path]; !ok {
			t.Fatalf("%s should be included in referenced browser asset allowlist: %#v", path, allowed)
		}
	}
	for _, path := range []string{"/unlinked.svg", "https://example.test/logo.svg", "data:image/svg+xml;base64,aaaa", "/styles/var(--brand-logo"} {
		if _, ok := allowed[path]; ok {
			t.Fatalf("%s should not be included in referenced browser asset allowlist: %#v", path, allowed)
		}
	}
}

func TestWriteJSONSetsNoStoreAndContentType(t *testing.T) {
	rec := httptest.NewRecorder()

	WriteJSON(rec, http.StatusAccepted, map[string]any{"ok": true})

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "application/json") {
		t.Fatalf("Content-Type = %q", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q", got)
	}
	if strings.TrimSpace(rec.Body.String()) != `{"ok":true}` {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestWriteJSONErrorUsesStableEnvelope(t *testing.T) {
	rec := httptest.NewRecorder()

	WriteJSONError(rec, http.StatusTeapot, "bad card")

	if rec.Code != http.StatusTeapot {
		t.Fatalf("status = %d", rec.Code)
	}
	if strings.TrimSpace(rec.Body.String()) != `{"error":"bad card"}` {
		t.Fatalf("body = %q", rec.Body.String())
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q", got)
	}
}

func TestRequireMethodRejectsUnexpectedVerb(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)

	if RequireMethod(rec, req, http.MethodPost) {
		t.Fatal("RequireMethod should reject GET when POST is required")
	}
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d", rec.Code)
	}
	if strings.TrimSpace(rec.Body.String()) != `{"error":"Only POST"}` {
		t.Fatalf("body = %q", rec.Body.String())
	}
	if got := rec.Header().Get("Allow"); got != http.MethodPost {
		t.Fatalf("Allow = %q", got)
	}

	okRec := httptest.NewRecorder()
	okReq := httptest.NewRequest(http.MethodPost, "/api/test", nil)
	if !RequireMethod(okRec, okReq, http.MethodPost) {
		t.Fatal("RequireMethod should allow POST")
	}
	if okRec.Code != http.StatusOK {
		t.Fatalf("allowed request should not write response, code=%d", okRec.Code)
	}
}

func TestRequireMethodMessageUsesCustomMessage(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)

	if RequireMethodMessage(rec, req, http.MethodPost, "Only POST is supported") {
		t.Fatal("RequireMethodMessage should reject GET when POST is required")
	}
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d", rec.Code)
	}
	if strings.TrimSpace(rec.Body.String()) != `{"error":"Only POST is supported"}` {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestResolvePortUsesDefaultTrimsAndRejectsInvalidValues(t *testing.T) {
	port, err := ResolvePort(" \t ", "8080")
	if err != nil {
		t.Fatalf("ResolvePort default returned error: %v", err)
	}
	if port != "8080" {
		t.Fatalf("default port = %q, want 8080", port)
	}

	port, err = ResolvePort(" 18820 ", "8080")
	if err != nil {
		t.Fatalf("ResolvePort explicit returned error: %v", err)
	}
	if port != "18820" {
		t.Fatalf("explicit port = %q, want 18820", port)
	}

	for _, value := range []string{"abc", "0", "-1", "65536", "80/tcp"} {
		t.Run(value, func(t *testing.T) {
			if port, err := ResolvePort(value, "8080"); err == nil {
				t.Fatalf("ResolvePort(%q) = %q, nil; want error", value, port)
			}
		})
	}
}

func TestServeFileSupportsHeadAndCachePolicy(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "index.html")
	if err := os.WriteFile(file, []byte("<h1>Hello</h1>"), 0644); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodHead, "/", nil)
	ServeFile(rec, req, file, true)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("HEAD response should not include a body")
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q", got)
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "text/html") {
		t.Fatalf("Content-Type = %q", got)
	}
}

func TestStaticHandlerErrorsAreNoStore(t *testing.T) {
	root := t.TempDir()
	handler := NewStaticHandler(root)
	for _, tc := range []struct {
		name   string
		method string
		path   string
		status int
	}{
		{name: "wrong method", method: http.MethodPost, path: "/", status: http.StatusMethodNotAllowed},
		{name: "blocked path", method: http.MethodGet, path: "/app.test.mjs", status: http.StatusNotFound},
		{name: "missing file", method: http.MethodGet, path: "/missing.js", status: http.StatusNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(tc.method, tc.path, nil)

			handler.ServeHTTP(rec, req)

			if rec.Code != tc.status {
				t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
			}
			if got := rec.Header().Get("Cache-Control"); got != "no-store" {
				t.Fatalf("Cache-Control = %q", got)
			}
		})
	}
}
