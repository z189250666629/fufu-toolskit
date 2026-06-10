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

func TestWriteJSONErrorUsesStablePayload(t *testing.T) {
	w := httptest.NewRecorder()

	writeJSONError(w, http.StatusTeapot, "bad card")

	if w.Code != http.StatusTeapot || strings.TrimSpace(w.Body.String()) != `{"error":"bad card"}` {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestReadCardKeyRequestTrimsAndRejectsInvalidInput(t *testing.T) {
	var body struct {
		CardKey string `json:"cardKey"`
	}
	req := httptest.NewRequest(http.MethodPost, "/api/spin", strings.NewReader(`{"cardKey":"  sk-card  "}`))

	key, ok := readCardKeyRequest(req, &body, func() string { return body.CardKey })
	if !ok || key != "sk-card" {
		t.Fatalf("card key = %q/%v", key, ok)
	}

	blankReq := httptest.NewRequest(http.MethodPost, "/api/spin", strings.NewReader(`{"cardKey":"  "}`))
	key, ok = readCardKeyRequest(blankReq, &body, func() string { return body.CardKey })
	if ok || key != "" {
		t.Fatalf("blank card key = %q/%v", key, ok)
	}

	badReq := httptest.NewRequest(http.MethodPost, "/api/spin", strings.NewReader(`{"cardKey"`))
	key, ok = readCardKeyRequest(badReq, &body, func() string { return body.CardKey })
	if ok || key != "" {
		t.Fatalf("bad body card key = %q/%v", key, ok)
	}
}

func TestReadCardKeyRequestRejectsTrailingJSON(t *testing.T) {
	var body struct {
		CardKey string `json:"cardKey"`
	}

	trailingReq := httptest.NewRequest(http.MethodPost, "/api/spin", strings.NewReader(`{"cardKey":"sk-card"} {}`))
	key, ok := readCardKeyRequest(trailingReq, &body, func() string { return body.CardKey })
	if ok || key != "" {
		t.Fatalf("trailing JSON card key = %q/%v", key, ok)
	}

	whitespaceReq := httptest.NewRequest(http.MethodPost, "/api/spin", strings.NewReader("{\"cardKey\":\"sk-card\"}\n\t "))
	key, ok = readCardKeyRequest(whitespaceReq, &body, func() string { return body.CardKey })
	if !ok || key != "sk-card" {
		t.Fatalf("trailing whitespace card key = %q/%v", key, ok)
	}
}

func TestCardKeyWhitespaceRejectedAcrossHandlers(t *testing.T) {
	cases := []struct {
		name    string
		handler func(http.ResponseWriter, *http.Request)
		body    string
	}{
		{name: "login", handler: handleLogin, body: `{"cardKey":"   "}`},
		{name: "spin", handler: handleSpin, body: `{"cardKey":"   "}`},
		{name: "scratch start", handler: handleScratchStart, body: `{"cardKey":"   "}`},
		{name: "scratch reveal", handler: handleScratchReveal, body: `{"cardKey":"   ","cellIndex":1}`},
		{name: "scratch cashout", handler: handleScratchCashout, body: `{"cardKey":"   "}`},
		{name: "scratch reset", handler: handleScratchReset, body: `{"cardKey":"   "}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/test", strings.NewReader(tc.body))
			w := httptest.NewRecorder()

			tc.handler(w, req)

			if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "请输入卡密") {
				t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
			}
		})
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
