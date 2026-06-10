package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"fufu/newapi"
	"fufu/tokens"
)

func useTokenStatusServer(t *testing.T, key string, status int) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/token/search" {
			t.Fatalf("unexpected token request %s %s", r.Method, r.URL.String())
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data": []any{map[string]any{
				"id":             30,
				"key":            key,
				"name":           "100-act-test",
				"interval_quota": newapi.DefaultQuotaUnit * 100,
				"remain_quota":   newapi.DefaultQuotaUnit * 100,
				"status":         status,
				"created_time":   actStartTS + 1,
			}},
		})
	}))
	t.Cleanup(server.Close)

	oldTokenSvc := tokenSvc
	tokenSvc = tokens.NewService(newapi.NewClient(newapi.Site{URL: server.URL, Token: "token", UserID: "1"}))
	t.Cleanup(func() { tokenSvc = oldTokenSvc })
}

func TestHandleLoginRevalidatesCachedDisabledToken(t *testing.T) {
	setupScratchLockTestDB(t)
	key := "sk-cached-disabled-login"
	if _, err := db.Exec(`INSERT INTO cards (card_key, card_name, dollars, total_spins) VALUES (?,?,?,?)`, key, "cached", 100, 1); err != nil {
		t.Fatal(err)
	}
	useTokenStatusServer(t, key, 2)

	req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"cardKey":"`+key+`"}`))
	w := httptest.NewRecorder()

	handleLogin(w, req)

	if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "已被禁用") {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestHandleSpinRejectsCachedDisabledToken(t *testing.T) {
	setupScratchLockTestDB(t)
	key := "sk-cached-disabled-spin"
	seedSpinCard(t, key)
	useTokenStatusServer(t, key, 2)

	req := httptest.NewRequest(http.MethodPost, "/api/spin", strings.NewReader(`{"cardKey":"`+key+`"}`))
	w := httptest.NewRecorder()

	handleSpin(w, req)

	if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "已被禁用") {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	var usedSpins, logs int
	if err := db.QueryRow(`SELECT used_spins FROM cards WHERE card_key=?`, key).Scan(&usedSpins); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM spin_log WHERE card_key=?`, key).Scan(&logs); err != nil {
		t.Fatal(err)
	}
	if usedSpins != 0 || logs != 0 {
		t.Fatalf("disabled token should not mutate spin state, used_spins=%d logs=%d", usedSpins, logs)
	}
}

func TestScratchStartRejectsCachedDisabledToken(t *testing.T) {
	setupScratchLockTestDB(t)
	key := "sk-cached-disabled-scratch"
	seedScratchCard(t, key)
	useTokenStatusServer(t, key, 2)

	req := httptest.NewRequest(http.MethodPost, "/api/scratch/start", strings.NewReader(`{"cardKey":"`+key+`"}`))
	w := httptest.NewRecorder()

	handleScratchStart(w, req)

	if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "已被禁用") {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	if _, ok := getScratch(key); ok {
		t.Fatal("disabled token should not start a scratch game")
	}
}
