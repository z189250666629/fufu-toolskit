package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSpinGuaranteeForThousandCard(t *testing.T) {
	got := spin(1000, false, 9, 10, 0, 0)
	if got.Type != "win" || got.Dollars != 100 {
		t.Fatalf("unexpected guarantee result: %#v", got)
	}
}

func TestScratchJSONHelpers(t *testing.T) {
	arr := jsonArr(`[1,2]`)
	if len(arr) != 2 || !intContains(arr, 2) || intContains(arr, 3) {
		t.Fatalf("bad json helpers: %#v", arr)
	}
}

func TestPostRejectsNonPOST(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/spin", nil)
	w := httptest.NewRecorder()

	post(w, req, func(http.ResponseWriter, *http.Request) {
		t.Fatal("handler should not be called")
	})

	if w.Code != http.StatusMethodNotAllowed || !strings.Contains(w.Body.String(), "Only POST") {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestStaticRouteServesPublicIndex(t *testing.T) {
	oldRoot := rootDir
	defer func() { rootDir = oldRoot }()

	tmp := t.TempDir()
	rootDir = tmp
	publicDir := filepath.Join(tmp, "public")
	if err := os.MkdirAll(publicDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(publicDir, "index.html"), []byte("activity home"), 0644); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	staticRoute(w, req)

	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "activity home") {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestWriteHTTPErrorMapsKnownError(t *testing.T) {
	w := httptest.NewRecorder()
	writeHTTPError(w, httpErr{Status: http.StatusForbidden, Message: "denied"})

	if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "denied") {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestMCYLoginSendsAllSetCookieValuesOnPost(t *testing.T) {
	oldCookie := mcyCookie
	t.Cleanup(func() { mcyCookie = oldCookie })
	mcyCookie = ""

	var gotCookie string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			http.SetCookie(w, &http.Cookie{Name: "mcy_session", Value: "s1"})
			http.SetCookie(w, &http.Cookie{Name: "mcy_csrf", Value: "c1"})
		case "/check":
			gotCookie = r.Header.Get("Cookie")
			_, _ = w.Write([]byte(`{}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	t.Setenv("MCY_BASE_URL", srv.URL)
	t.Setenv("MCY_USERNAME", "u")
	t.Setenv("MCY_PASSWORD", "p")
	t.Setenv("MCY_LOGIN_ENDPOINT", "/login")

	if err := mcyLogin(); err != nil {
		t.Fatal(err)
	}
	if _, err := mcyPost("/check", map[string]any{"ok": true}); err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{"mcy_session=s1", "mcy_csrf=c1"} {
		if !strings.Contains(gotCookie, want) {
			t.Fatalf("Cookie header %q missing %q; mcyCookie=%q", gotCookie, want, mcyCookie)
		}
	}
}
