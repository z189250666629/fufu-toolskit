package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCardKeyRejectsOverlongOrControlCharactersAcrossHandlers(t *testing.T) {
	setupScratchLockTestDB(t)
	overlong := "sk-" + strings.Repeat("a", 300)
	withControl := "sk-valid\\u0001key"

	for _, tc := range []struct {
		name    string
		path    string
		handler func(http.ResponseWriter, *http.Request)
		body    func(string) string
	}{
		{name: "login", path: "/api/login", handler: handleLogin, body: func(key string) string {
			return `{"cardKey":"` + key + `"}`
		}},
		{name: "spin", path: "/api/spin", handler: handleSpin, body: func(key string) string {
			return `{"cardKey":"` + key + `"}`
		}},
		{name: "scratch start", path: "/api/scratch/start", handler: handleScratchStart, body: func(key string) string {
			return `{"cardKey":"` + key + `"}`
		}},
		{name: "scratch reveal", path: "/api/scratch/reveal", handler: handleScratchReveal, body: func(key string) string {
			return `{"cardKey":"` + key + `","cellIndex":0}`
		}},
		{name: "scratch cashout", path: "/api/scratch/cashout", handler: handleScratchCashout, body: func(key string) string {
			return `{"cardKey":"` + key + `"}`
		}},
		{name: "scratch reset", path: "/api/scratch/reset", handler: handleScratchReset, body: func(key string) string {
			return `{"cardKey":"` + key + `"}`
		}},
	} {
		for _, key := range []string{overlong, withControl} {
			t.Run(tc.name, func(t *testing.T) {
				req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body(key)))
				w := httptest.NewRecorder()

				tc.handler(w, req)

				if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "卡密格式错误") {
					t.Fatalf("%s should reject invalid cardKey, code=%d body=%s", tc.name, w.Code, w.Body.String())
				}
			})
		}
	}
}
