package activityapp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func seedScratchCardWithRounds(t *testing.T, key string, total, used, totalWon int) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO cards (card_key, card_name, dollars, total_spins, used_spins, total_won) VALUES (?,?,?,?,?,?)`,
		key,
		"scratch-rounds",
		55,
		total,
		used,
		totalWon,
	); err != nil {
		t.Fatal(err)
	}
}

func postScratch(t *testing.T, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	w := httptest.NewRecorder()
	switch path {
	case "/api/scratch/start":
		handleScratchStart(w, req)
	case "/api/scratch/reveal":
		handleScratchReveal(w, req)
	case "/api/scratch/cashout":
		handleScratchCashout(w, req)
	default:
		t.Fatalf("unsupported path %s", path)
	}
	return w
}

func setScratchMinesForTest(t *testing.T, key string) {
	t.Helper()
	if _, err := db.Exec(`UPDATE scratch_games SET mine_pos='[7,8]', revealed='[]', prize_dollars=0, status='playing' WHERE card_key=?`, key); err != nil {
		t.Fatal(err)
	}
}

func TestScratchRoundConsumesOneDrawOnCompletionNotPerReveal(t *testing.T) {
	setupScratchLockTestDB(t)
	key := "scratch-round-card"
	seedScratchCardWithRounds(t, key, 2, 0, 0)
	seedScratchGame(t, key, "[7,8]", "[]", 0, "playing")

	reveal := postScratch(t, "/api/scratch/reveal", `{"cardKey":"scratch-round-card","cellIndex":0}`)
	if reveal.Code != http.StatusOK {
		t.Fatalf("reveal code=%d body=%s", reveal.Code, reveal.Body.String())
	}
	var usedSpins, totalWon, logs int
	if err := db.QueryRow(`SELECT used_spins,total_won FROM cards WHERE card_key=?`, key).Scan(&usedSpins, &totalWon); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM spin_log WHERE card_key=?`, key).Scan(&logs); err != nil {
		t.Fatal(err)
	}
	if usedSpins != 0 || totalWon != 0 || logs != 0 {
		t.Fatalf("revealing one cell should not consume a draw: used=%d won=%d logs=%d", usedSpins, totalWon, logs)
	}

	cashout := postScratch(t, "/api/scratch/cashout", `{"cardKey":"scratch-round-card"}`)
	if cashout.Code != http.StatusOK {
		t.Fatalf("cashout code=%d body=%s", cashout.Code, cashout.Body.String())
	}
	if err := db.QueryRow(`SELECT used_spins,total_won FROM cards WHERE card_key=?`, key).Scan(&usedSpins, &totalWon); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM spin_log WHERE card_key=?`, key).Scan(&logs); err != nil {
		t.Fatal(err)
	}
	var queued int
	if err := db.QueryRow(`SELECT COUNT(*) FROM credit_queue WHERE card_key=?`, key).Scan(&queued); err != nil {
		t.Fatal(err)
	}
	if usedSpins != 1 || totalWon != 2 || logs != 1 || queued != 0 {
		t.Fatalf("first completed scratch should consume one draw without final credit: used=%d won=%d logs=%d queued=%d", usedSpins, totalWon, logs, queued)
	}
}

func TestScratchCanStartNextRoundUntilConfiguredDrawsAreUsed(t *testing.T) {
	setupScratchLockTestDB(t)
	key := "scratch-two-round-card"
	seedScratchCardWithRounds(t, key, 2, 0, 0)
	seedScratchGame(t, key, "[7,8]", "[0]", 2, "cashout")
	if _, err := db.Exec(`UPDATE cards SET used_spins=1,total_won=2 WHERE card_key=?`, key); err != nil {
		t.Fatal(err)
	}

	start := postScratch(t, "/api/scratch/start", `{"cardKey":"scratch-two-round-card"}`)
	if start.Code != http.StatusOK {
		t.Fatalf("start next round code=%d body=%s", start.Code, start.Body.String())
	}
	setScratchMinesForTest(t, key)
	reveal := postScratch(t, "/api/scratch/reveal", `{"cardKey":"scratch-two-round-card","cellIndex":0}`)
	if reveal.Code != http.StatusOK {
		t.Fatalf("second reveal code=%d body=%s", reveal.Code, reveal.Body.String())
	}
	cashout := postScratch(t, "/api/scratch/cashout", `{"cardKey":"scratch-two-round-card"}`)
	if cashout.Code != http.StatusOK {
		t.Fatalf("second cashout code=%d body=%s", cashout.Code, cashout.Body.String())
	}

	var usedSpins, totalWon, queuedPrize int
	if err := db.QueryRow(`SELECT used_spins,total_won FROM cards WHERE card_key=?`, key).Scan(&usedSpins, &totalWon); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT prize_dollars FROM credit_queue WHERE card_key=?`, key).Scan(&queuedPrize); err != nil {
		t.Fatal(err)
	}
	if usedSpins != 2 || totalWon != 4 || queuedPrize != 4 {
		t.Fatalf("second completed scratch should finish card and enqueue cumulative prize: used=%d won=%d queued=%d", usedSpins, totalWon, queuedPrize)
	}
}
