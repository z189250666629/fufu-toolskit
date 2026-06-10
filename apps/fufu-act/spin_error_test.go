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
