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
	if _, err := db.Exec(`INSERT OR IGNORE INTO cards (card_key, card_name, dollars, total_spins) VALUES (?,?,?,?)`, key, "scratch-test", 55, 0); err != nil {
		t.Fatal(err)
	}
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

func TestScratchRevealRejectsCorruptMinePositions(t *testing.T) {
	setupScratchLockTestDB(t)
	seedScratchGame(t, "scratch-card", "not-json", "[]", 0, "playing")

	req := httptest.NewRequest(http.MethodPost, "/api/scratch/reveal", strings.NewReader(`{"cardKey":"scratch-card","cellIndex":0}`))
	w := httptest.NewRecorder()

	handleScratchReveal(w, req)

	if w.Code != http.StatusInternalServerError || !strings.Contains(w.Body.String(), "服务器错误") {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	var status string
	var revealed string
	if err := db.QueryRow(`SELECT status,revealed FROM scratch_games WHERE card_key=?`, "scratch-card").Scan(&status, &revealed); err != nil {
		t.Fatal(err)
	}
	if status != "playing" || revealed != "[]" {
		t.Fatalf("corrupt mine positions should not mutate game, status=%q revealed=%s", status, revealed)
	}
}

func TestScratchRevealRejectsInvalidPersistedScratchArrays(t *testing.T) {
	for _, tc := range []struct {
		name     string
		minePos  string
		revealed string
	}{
		{name: "duplicate mines", minePos: "[7,7]", revealed: "[]"},
		{name: "mine out of range", minePos: "[7,9]", revealed: "[]"},
		{name: "wrong mine count", minePos: "[7]", revealed: "[]"},
		{name: "duplicate revealed", minePos: "[7,8]", revealed: "[0,0]"},
		{name: "revealed out of range", minePos: "[7,8]", revealed: "[9]"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setupScratchLockTestDB(t)
			seedScratchGame(t, "scratch-card", tc.minePos, tc.revealed, 0, "playing")

			req := httptest.NewRequest(http.MethodPost, "/api/scratch/reveal", strings.NewReader(`{"cardKey":"scratch-card","cellIndex":0}`))
			w := httptest.NewRecorder()

			handleScratchReveal(w, req)

			if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "进度异常") {
				t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
			}
			var status, revealed string
			if err := db.QueryRow(`SELECT status,revealed FROM scratch_games WHERE card_key=?`, "scratch-card").Scan(&status, &revealed); err != nil {
				t.Fatal(err)
			}
			if status != "playing" || revealed != tc.revealed {
				t.Fatalf("invalid scratch state should not mutate game, status=%q revealed=%s", status, revealed)
			}
		})
	}
}

func TestScratchRevealRequiresEligibleScratchCard(t *testing.T) {
	setupScratchLockTestDB(t)
	if _, err := db.Exec(`INSERT INTO scratch_games (card_key, mine_pos, revealed, prize_dollars, status) VALUES (?,?,?,?,?)`, "orphan-scratch", "[7,8]", "[]", 0, "playing"); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/scratch/reveal", strings.NewReader(`{"cardKey":"orphan-scratch","cellIndex":0}`))
	w := httptest.NewRecorder()

	handleScratchReveal(w, req)

	if w.Code != http.StatusNotFound || !strings.Contains(w.Body.String(), "请先登录") {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	var status, revealed string
	if err := db.QueryRow(`SELECT status,revealed FROM scratch_games WHERE card_key=?`, "orphan-scratch").Scan(&status, &revealed); err != nil {
		t.Fatal(err)
	}
	if status != "playing" || revealed != "[]" {
		t.Fatalf("ineligible scratch should not mutate game, status=%q revealed=%s", status, revealed)
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

func TestScratchCashoutRejectsCorruptRevealedJSON(t *testing.T) {
	setupScratchLockTestDB(t)
	seedScratchGame(t, "scratch-card", "[7,8]", "not-json", 2, "playing")

	req := httptest.NewRequest(http.MethodPost, "/api/scratch/cashout", strings.NewReader(`{"cardKey":"scratch-card"}`))
	w := httptest.NewRecorder()

	handleScratchCashout(w, req)

	if w.Code != http.StatusInternalServerError || !strings.Contains(w.Body.String(), "服务器错误") {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	var status string
	if err := db.QueryRow(`SELECT status FROM scratch_games WHERE card_key=?`, "scratch-card").Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "playing" {
		t.Fatalf("corrupt revealed JSON should not cash out, status=%q", status)
	}
}

func TestScratchCashoutRequiresEligibleScratchCard(t *testing.T) {
	setupScratchLockTestDB(t)
	if _, err := db.Exec(`INSERT INTO cards (card_key, card_name, dollars, total_spins) VALUES (?,?,?,?)`, "near-scratch-card", "near scratch", 55.4, 0); err != nil {
		t.Fatal(err)
	}
	seedScratchGame(t, "near-scratch-card", "[7,8]", "[0]", 2, "playing")

	req := httptest.NewRequest(http.MethodPost, "/api/scratch/cashout", strings.NewReader(`{"cardKey":"near-scratch-card"}`))
	w := httptest.NewRecorder()

	handleScratchCashout(w, req)

	if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "不参与刮刮乐") {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	var status string
	var prize int
	if err := db.QueryRow(`SELECT status,prize_dollars FROM scratch_games WHERE card_key=?`, "near-scratch-card").Scan(&status, &prize); err != nil {
		t.Fatal(err)
	}
	if status != "playing" || prize != 2 {
		t.Fatalf("ineligible scratch should not cash out, status=%q prize=%d", status, prize)
	}
}

func TestScratchRevealDoesNotMarkWonWhenCreditEnqueueFails(t *testing.T) {
	setupScratchLockTestDB(t)
	seedScratchGame(t, "scratch-card", "[7,8]", "[0,1,2,3,4]", 12, "playing")
	if _, err := db.Exec(`DROP TABLE credit_queue`); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/scratch/reveal", strings.NewReader(`{"cardKey":"scratch-card","cellIndex":5}`))
	w := httptest.NewRecorder()

	handleScratchReveal(w, req)

	if w.Code != http.StatusInternalServerError || !strings.Contains(w.Body.String(), "服务器错误") {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	var status string
	var prize int
	var revealed string
	if err := db.QueryRow(`SELECT status,prize_dollars,revealed FROM scratch_games WHERE card_key=?`, "scratch-card").Scan(&status, &prize, &revealed); err != nil {
		t.Fatal(err)
	}
	if status != "playing" || prize != 12 || revealed != "[0,1,2,3,4]" {
		t.Fatalf("scratch game should be rolled back, status=%q prize=%d revealed=%s", status, prize, revealed)
	}
}

func TestScratchCashoutDoesNotMarkCashoutWhenCreditEnqueueFails(t *testing.T) {
	setupScratchLockTestDB(t)
	seedScratchGame(t, "scratch-card", "[7,8]", "[0]", 2, "playing")
	if _, err := db.Exec(`DROP TABLE credit_queue`); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/scratch/cashout", strings.NewReader(`{"cardKey":"scratch-card"}`))
	w := httptest.NewRecorder()

	handleScratchCashout(w, req)

	if w.Code != http.StatusInternalServerError || !strings.Contains(w.Body.String(), "服务器错误") {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	var status string
	var prize int
	if err := db.QueryRow(`SELECT status,prize_dollars FROM scratch_games WHERE card_key=?`, "scratch-card").Scan(&status, &prize); err != nil {
		t.Fatal(err)
	}
	if status != "playing" || prize != 2 {
		t.Fatalf("scratch cashout should be rolled back, status=%q prize=%d", status, prize)
	}
}

func TestScratchCashoutRejectsCorruptRevealedCountWithoutPanic(t *testing.T) {
	setupScratchLockTestDB(t)
	seedScratchGame(t, "scratch-card", "[7,8]", "[0,1,2,3,4,5,6]", 15, "playing")

	req := httptest.NewRequest(http.MethodPost, "/api/scratch/cashout", strings.NewReader(`{"cardKey":"scratch-card"}`))
	w := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("cashout should reject corrupt progress, not panic: %v", r)
		}
	}()

	handleScratchCashout(w, req)

	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "进度异常") {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestScratchRevealReturnsServerErrorWhenScratchLookupFails(t *testing.T) {
	setupScratchLockTestDB(t)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/scratch/reveal", strings.NewReader(`{"cardKey":"scratch-card","cellIndex":0}`))
	w := httptest.NewRecorder()

	handleScratchReveal(w, req)

	if w.Code != http.StatusInternalServerError || !strings.Contains(w.Body.String(), "服务器错误") {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestScratchCashoutReturnsServerErrorWhenScratchLookupFails(t *testing.T) {
	setupScratchLockTestDB(t)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/scratch/cashout", strings.NewReader(`{"cardKey":"scratch-card"}`))
	w := httptest.NewRecorder()

	handleScratchCashout(w, req)

	if w.Code != http.StatusInternalServerError || !strings.Contains(w.Body.String(), "服务器错误") {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
}
