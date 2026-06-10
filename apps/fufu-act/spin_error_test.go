package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func seedSpinCard(t *testing.T, key string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO cards (card_key, card_name, dollars, total_spins, rigged) VALUES (?,?,?,?,?)`, key, "spin-test", 100, 1, `{"1":5}`); err != nil {
		t.Fatal(err)
	}
}

func TestHandleSpinReturnsServerErrorWhenSpinWriteFails(t *testing.T) {
	setupScratchLockTestDB(t)
	seedSpinCard(t, "spin-card")
	makeScratchDBReadOnlyForTest(t)

	req := httptest.NewRequest(http.MethodPost, "/api/spin", strings.NewReader(`{"cardKey":"spin-card"}`))
	w := httptest.NewRecorder()

	handleSpin(w, req)

	if w.Code != http.StatusInternalServerError || !strings.Contains(w.Body.String(), "服务器错误") {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestHandleSpinRejectsUnsupportedStoredDollarTier(t *testing.T) {
	setupScratchLockTestDB(t)
	if _, err := db.Exec(`INSERT INTO cards (card_key, card_name, dollars, total_spins, rigged) VALUES (?,?,?,?,?)`, "odd-tier-card", "odd-tier", 42, 1, `{"1":5}`); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/spin", strings.NewReader(`{"cardKey":"odd-tier-card"}`))
	w := httptest.NewRecorder()

	handleSpin(w, req)

	if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "额度不参与活动") {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	var logs int
	if err := db.QueryRow(`SELECT COUNT(*) FROM spin_log WHERE card_key=?`, "odd-tier-card").Scan(&logs); err != nil {
		t.Fatal(err)
	}
	if logs != 0 {
		t.Fatalf("spin_log rows = %d", logs)
	}
}
