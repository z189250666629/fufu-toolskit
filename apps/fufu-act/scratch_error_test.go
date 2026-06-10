package main

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func makeScratchDBReadOnlyForTest(t *testing.T) {
	t.Helper()
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA query_only = ON`); err != nil {
		t.Fatal(err)
	}
}

func seedScratchGame(t *testing.T, key string, minePos string, revealed string, prize int, status string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO scratch_games (card_key, mine_pos, revealed, prize_dollars, status) VALUES (?,?,?,?,?)`, key, minePos, revealed, prize, status); err != nil {
		t.Fatal(err)
	}
}

func TestScratchRevealReturnsServerErrorWhenUpdateFails(t *testing.T) {
	setupScratchLockTestDB(t)
	seedScratchGame(t, "scratch-card", "[7,8]", "[]", 0, "playing")
	makeScratchDBReadOnlyForTest(t)

	req := httptest.NewRequest(http.MethodPost, "/api/scratch/reveal", strings.NewReader(`{"cardKey":"scratch-card","cellIndex":0}`))
	w := httptest.NewRecorder()

	handleScratchReveal(w, req)

	if w.Code != http.StatusInternalServerError || !strings.Contains(w.Body.String(), "服务器错误") {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestScratchRevealRequiresCellIndex(t *testing.T) {
	setupScratchLockTestDB(t)
	seedScratchGame(t, "scratch-card", "[7,8]", "[]", 0, "playing")

	req := httptest.NewRequest(http.MethodPost, "/api/scratch/reveal", strings.NewReader(`{"cardKey":"scratch-card"}`))
	w := httptest.NewRecorder()

	handleScratchReveal(w, req)

	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "无效的格子") {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	var revealed sql.NullString
	if err := db.QueryRow(`SELECT revealed FROM scratch_games WHERE card_key=?`, "scratch-card").Scan(&revealed); err != nil {
		t.Fatal(err)
	}
	if !revealed.Valid || revealed.String != "[]" {
		t.Fatalf("revealed = %#v", revealed)
	}
}

func TestScratchCashoutReturnsServerErrorWhenUpdateFails(t *testing.T) {
	setupScratchLockTestDB(t)
	seedScratchGame(t, "scratch-card", "[7,8]", "[0]", 2, "playing")
	makeScratchDBReadOnlyForTest(t)

	req := httptest.NewRequest(http.MethodPost, "/api/scratch/cashout", strings.NewReader(`{"cardKey":"scratch-card"}`))
	w := httptest.NewRecorder()

	handleScratchCashout(w, req)

	if w.Code != http.StatusInternalServerError || !strings.Contains(w.Body.String(), "服务器错误") {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
}
