package activityapp

import (
	"encoding/json"
	"fufu/newapi"
	"fufu/tokens"
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
	oldTokenSvc := tokenSvc
	oldTokenConfigErr := tokenConfigErr
	oldUnknownLoginLimiter := unknownLoginLimiter
	testDB, err := initDB(filepath.Join(t.TempDir(), "slot.db"))
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := "sk-" + r.URL.Query().Get("token")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data": []any{map[string]any{
				"id":             30,
				"key":            key,
				"name":           "100-act-test",
				"interval_quota": newapi.DefaultQuotaUnit * 100,
				"remain_quota":   newapi.DefaultQuotaUnit * 100,
				"status":         1,
				"created_time":   actStartTS + 1,
			}},
		})
	}))
	db = testDB
	cardLocks = &cardLockRegistry{}
	unknownLoginLimiter = newLoginUnknownRateLimiter()
	tokenSvc = tokens.NewService(newapi.NewClient(newapi.Site{URL: server.URL, Token: "token", UserID: "1"}))
	tokenConfigErr = nil
	t.Cleanup(func() {
		server.Close()
		_ = testDB.Close()
		db = oldDB
		cardLocks = oldLocks
		unknownLoginLimiter = oldUnknownLoginLimiter
		tokenSvc = oldTokenSvc
		tokenConfigErr = oldTokenConfigErr
	})
}

func seedScratchCard(t *testing.T, key string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO cards (card_key, card_name, dollars, total_spins) VALUES (?,?,?,?)`, key, "scratch-test", 55, 1); err != nil {
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

func TestScratchStartRequiresExact55Dollars(t *testing.T) {
	setupScratchLockTestDB(t)
	if _, err := db.Exec(`INSERT INTO cards (card_key, card_name, dollars, total_spins) VALUES (?,?,?,?)`, "near-scratch-card", "near scratch", 55.4, 0); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/scratch/start", strings.NewReader(`{"cardKey":"near-scratch-card"}`))
	w := httptest.NewRecorder()
	handleScratchStart(w, req)

	if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "不参与刮刮乐") {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	if _, ok := getScratch("near-scratch-card"); ok {
		t.Fatal("near-55 card should not create a scratch game")
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

func TestScratchResetAllowsCaseInsensitiveTestCard(t *testing.T) {
	setupScratchLockTestDB(t)
	if _, err := db.Exec(`INSERT INTO cards (card_key, card_name, dollars, total_spins) VALUES (?,?,?,?)`, "scratch-card", "55-act-TEST", 55, 0); err != nil {
		t.Fatal(err)
	}
	seedScratchGame(t, "scratch-card", "[1,2]", "[0]", 1, "cashout")

	req := httptest.NewRequest(http.MethodPost, "/api/scratch/reset", strings.NewReader(`{"cardKey":"scratch-card"}`))
	w := httptest.NewRecorder()

	handleScratchReset(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	if _, ok := getScratch("scratch-card"); ok {
		t.Fatalf("scratch game should be deleted after reset")
	}
}

func TestScratchResetInvalidatesPriorDoneCreditForTestCard(t *testing.T) {
	setupScratchLockTestDB(t)
	if _, err := db.Exec(`INSERT INTO cards (card_key, card_name, dollars, total_spins) VALUES (?,?,?,?)`, "scratch-card", "55-act-test", 55, 0); err != nil {
		t.Fatal(err)
	}
	seedScratchGame(t, "scratch-card", "[1,2]", "[0,3,4,5,6]", 12, "won")
	if _, err := db.Exec(`INSERT INTO credit_queue (card_key, prize_dollars, status, processed_at) VALUES (?,?,?,datetime('now'))`, "scratch-card", 12, "done"); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/scratch/reset", strings.NewReader(`{"cardKey":"scratch-card"}`))
	w := httptest.NewRecorder()

	handleScratchReset(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	if err := enqueueCredit("scratch-card", 4); err != nil {
		t.Fatalf("reset should allow a future scratch prize to enqueue: %v", err)
	}
	var pending, archived int
	if err := db.QueryRow(`SELECT COUNT(*) FROM credit_queue WHERE card_key=? AND status='pending'`, "scratch-card").Scan(&pending); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM credit_queue WHERE card_key=? AND status='archived'`, "scratch-card").Scan(&archived); err != nil {
		t.Fatal(err)
	}
	if pending != 1 || archived != 1 {
		t.Fatalf("pending=%d archived=%d, want one new pending and one archived old row", pending, archived)
	}
}

func TestScratchResetRejectsTestSubstringInNonTestCard(t *testing.T) {
	setupScratchLockTestDB(t)
	if _, err := db.Exec(`INSERT INTO cards (card_key, card_name, dollars, total_spins) VALUES (?,?,?,?)`, "scratch-card", "55-act-contest", 55, 0); err != nil {
		t.Fatal(err)
	}
	seedScratchGame(t, "scratch-card", "[1,2]", "[0]", 1, "cashout")

	req := httptest.NewRequest(http.MethodPost, "/api/scratch/reset", strings.NewReader(`{"cardKey":"scratch-card"}`))
	w := httptest.NewRecorder()

	handleScratchReset(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	if _, ok := getScratch("scratch-card"); !ok {
		t.Fatalf("scratch game should not be deleted")
	}
}
