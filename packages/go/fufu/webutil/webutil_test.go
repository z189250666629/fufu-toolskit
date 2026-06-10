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
