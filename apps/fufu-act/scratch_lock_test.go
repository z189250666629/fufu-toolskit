package main

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func setupScratchLockTestDB(t *testing.T) {
	t.Helper()
	oldDB := db
	oldLocks := cardLocks
	testDB, err := initDB(filepath.Join(t.TempDir(), "slot.db"))
	if err != nil {
		t.Fatal(err)
	}
	db = testDB
	cardLocks = &cardLockRegistry{}
	t.Cleanup(func() {
		_ = testDB.Close()
		db = oldDB
		cardLocks = oldLocks
	})
}

func seedScratchCard(t *testing.T, key string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO cards (card_key, card_name, dollars, total_spins) VALUES (?,?,?,?)`, key, "scratch-test", 55, 0); err != nil {
		t.Fatal(err)
	}
}

func lockCardForTest(key string) func() {
	entry := cardLocks.acquire(key)
	return func() {
		cardLocks.release(key, entry)
	}
}

func assertHandlerWaitsForCardLock(t *testing.T, key string, call func() *httptest.ResponseRecorder) *httptest.ResponseRecorder {
	t.Helper()
	unlock := lockCardForTest(key)
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		done <- call()
	}()

	select {
	case rec := <-done:
		t.Fatalf("handler completed before card lock was released: code=%d body=%s", rec.Code, rec.Body.String())
	case <-time.After(80 * time.Millisecond):
	}

	unlock()
	select {
	case rec := <-done:
		return rec
	case <-time.After(time.Second):
		t.Fatal("handler did not complete after card lock was released")
	}
	return nil
}

func TestScratchStartUsesCardLock(t *testing.T) {
	setupScratchLockTestDB(t)
	seedScratchCard(t, "scratch-card")

	rec := assertHandlerWaitsForCardLock(t, "scratch-card", func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/scratch/start", strings.NewReader(`{"cardKey":"scratch-card"}`))
		w := httptest.NewRecorder()
		handleScratchStart(w, req)
		return w
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestWithCardLockReleasesIdleLockEntry(t *testing.T) {
	setupScratchLockTestDB(t)

	if _, err := withCardLock("idle-card", func() (any, error) {
		return "ok", nil
	}); err != nil {
		t.Fatal(err)
	}
	if cardLocks.has("idle-card") {
		t.Fatal("idle card lock entry should be removed after use")
	}
}

func TestScratchResetUsesCardLock(t *testing.T) {
	setupScratchLockTestDB(t)
	seedScratchCard(t, "scratch-card")
	if _, err := db.Exec(`INSERT INTO scratch_games (card_key, mine_pos, revealed, prize_dollars, status) VALUES (?,?,?,?,?)`, "scratch-card", "[1,2]", "[0]", 1, "cashout"); err != nil {
		t.Fatal(err)
	}

	rec := assertHandlerWaitsForCardLock(t, "scratch-card", func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/scratch/reset", strings.NewReader(`{"cardKey":"scratch-card"}`))
		w := httptest.NewRecorder()
		handleScratchReset(w, req)
		return w
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	if _, ok := getScratch("scratch-card"); ok {
		t.Fatalf("scratch game should be deleted after reset")
	}
}
