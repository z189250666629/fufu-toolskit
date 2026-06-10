package main

import (
	"fufu/tokens"
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
	if got := w.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q", got)
	}
}

func TestInitAllResetsTokenConfigStateOnPrimarySiteError(t *testing.T) {
	oldRoot := rootDir
	oldDB := db
	oldTokenSvc := tokenSvc
	oldTokenConfigErr := tokenConfigErr
	tempRoot := t.TempDir()
	t.Cleanup(func() {
		if db != nil && db != oldDB {
			_ = db.Close()
		}
		rootDir = oldRoot
		db = oldDB
		tokenSvc = oldTokenSvc
		tokenConfigErr = oldTokenConfigErr
	})
	clearActPrimaryEnv(t)
	t.Setenv("NEWAPI_MANAGED_API_SITES", `not-json`)
	rootDir = tempRoot
	tokenSvc = &tokens.Service{}
	tokenConfigErr = nil

	if err := initAll(); err != nil {
		t.Fatal(err)
	}
	if tokenSvc != nil {
		t.Fatalf("tokenSvc should be reset on config error, got %#v", tokenSvc)
	}
	if tokenConfigErr == nil || !strings.Contains(tokenConfigErr.Error(), "NEWAPI_MANAGED_API_SITES 不是有效 JSON") {
		t.Fatalf("tokenConfigErr = %v", tokenConfigErr)
	}
}

func clearActPrimaryEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		"FUFU_COMBINE_API_URL",
		"FUFU_COMBINE_API_TOKEN",
		"FUFU_COMBINE_USER_ID",
		"FUFU_COMBINE_QUOTA_UNIT",
		"FUFU_COMBINE_NAME",
		"FUFU_API_BASE_URL",
		"FUFU_API_TOKEN",
		"FUFU_API_USER_ID",
		"FUFU_QUOTA_UNIT",
		"NEWAPI_API_SITE_URL",
		"NEWAPI_API_SITE_TOKEN",
		"NEWAPI_API_SITE_ACCESS_TOKEN",
		"NEWAPI_TOKEN_SITE_URL",
		"NEWAPI_TOKEN_SITE_TOKEN",
		"NEWAPI_TOKEN_SITE_ACCESS_TOKEN",
		"NEWAPI_MANAGED_API_SITES",
		"NEWAPI_MANAGED_API_CONFIG",
	} {
		t.Setenv(name, "")
	}
	for i := 1; i <= 10; i++ {
		prefix := "NEWAPI_MANAGED_SITE_" + string(rune('0'+i))
		if i == 10 {
			prefix = "NEWAPI_MANAGED_SITE_10"
		}
		t.Setenv(prefix+"_URL", "")
		t.Setenv(prefix+"_TOKEN", "")
		t.Setenv(prefix+"_ACCESS_TOKEN", "")
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

	key, ok, err := readCardKeyRequest(req, &body, func() string { return body.CardKey })
	if err != nil || !ok || key != "sk-card" {
		t.Fatalf("card key = %q/%v err=%v", key, ok, err)
	}

	blankReq := httptest.NewRequest(http.MethodPost, "/api/spin", strings.NewReader(`{"cardKey":"  "}`))
	key, ok, err = readCardKeyRequest(blankReq, &body, func() string { return body.CardKey })
	if err != nil || ok || key != "" {
		t.Fatalf("blank card key = %q/%v err=%v", key, ok, err)
	}

	badReq := httptest.NewRequest(http.MethodPost, "/api/spin", strings.NewReader(`{"cardKey"`))
	key, ok, err = readCardKeyRequest(badReq, &body, func() string { return body.CardKey })
	if err == nil || ok || key != "" {
		t.Fatalf("bad body card key = %q/%v err=%v", key, ok, err)
	}
}

func TestReadCardKeyRequestRejectsTrailingJSON(t *testing.T) {
	var body struct {
		CardKey string `json:"cardKey"`
	}

	trailingReq := httptest.NewRequest(http.MethodPost, "/api/spin", strings.NewReader(`{"cardKey":"sk-card"} {}`))
	key, ok, err := readCardKeyRequest(trailingReq, &body, func() string { return body.CardKey })
	if err == nil || ok || key != "" {
		t.Fatalf("trailing JSON card key = %q/%v err=%v", key, ok, err)
	}

	whitespaceReq := httptest.NewRequest(http.MethodPost, "/api/spin", strings.NewReader("{\"cardKey\":\"sk-card\"}\n\t "))
	key, ok, err = readCardKeyRequest(whitespaceReq, &body, func() string { return body.CardKey })
	if err != nil || !ok || key != "sk-card" {
		t.Fatalf("trailing whitespace card key = %q/%v err=%v", key, ok, err)
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

func TestCardKeyHandlersReportMalformedJSON(t *testing.T) {
	cases := []struct {
		name    string
		handler func(http.ResponseWriter, *http.Request)
	}{
		{name: "login", handler: handleLogin},
		{name: "spin", handler: handleSpin},
		{name: "scratch start", handler: handleScratchStart},
		{name: "scratch reveal", handler: handleScratchReveal},
		{name: "scratch cashout", handler: handleScratchCashout},
		{name: "scratch reset", handler: handleScratchReset},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/test", strings.NewReader(`{"cardKey"`))
			w := httptest.NewRecorder()

			tc.handler(w, req)

			body := w.Body.String()
			if w.Code != http.StatusBadRequest || !strings.Contains(body, "请求格式错误") || strings.Contains(body, "请输入卡密") {
				t.Fatalf("code=%d body=%s", w.Code, body)
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
