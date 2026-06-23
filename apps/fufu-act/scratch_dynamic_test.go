package activityapp

import (
	"encoding/json"
	"net/http"
	"testing"

	"fufu/activity"
	"fufu/poolfundcore"
)

func TestScratchDynamicRewardsUseSeparatePoolAndConfiguredSteps(t *testing.T) {
	setupScratchLockTestDB(t)
	restoreRuntimeConfig(t)
	cfg := activity.DefaultConfig()
	cfg.DynamicPrizePool = poolfundcore.Config{Enabled: true}
	cfg.ScratchMaxReveals = 4
	SetRuntimeConfig(cfg)
	insertPrizePoolLedgerForTest(t, "sk-main-pool-seed", "deposit", 1000, "", "")
	insertScratchPrizePoolLedgerForTest(t, "sk-scratch-pool-seed", "deposit", 1000, "", "")

	rewards, err := scratchRewardsForCurrentPool()
	if err != nil {
		t.Fatal(err)
	}
	wantRewards := []int{25, 50, 75, 100}
	if !sameScratchRewardInts(rewards, wantRewards) {
		t.Fatalf("dynamic scratch rewards=%+v, want %+v", rewards, wantRewards)
	}

	key := "scratch-dynamic-card"
	seedScratchCardWithRounds(t, key, 1, 0, 0)
	seedScratchGame(t, key, "[7,8]", "[]", 0, "playing")

	reveal := postScratch(t, "/api/scratch/reveal", `{"cardKey":"scratch-dynamic-card","cellIndex":0}`)
	if reveal.Code != http.StatusOK {
		t.Fatalf("reveal code=%d body=%s", reveal.Code, reveal.Body.String())
	}
	var revealBody struct {
		MaxReveals int `json:"maxReveals"`
		Prize      int `json:"prize"`
	}
	if err := json.Unmarshal(reveal.Body.Bytes(), &revealBody); err != nil {
		t.Fatal(err)
	}
	if revealBody.MaxReveals != 4 {
		t.Fatalf("maxReveals=%d, want 4", revealBody.MaxReveals)
	}
	if revealBody.Prize != 25 {
		t.Fatalf("first safe prize=%d, want 25", revealBody.Prize)
	}

	cashout := postScratch(t, "/api/scratch/cashout", `{"cardKey":"scratch-dynamic-card"}`)
	if cashout.Code != http.StatusOK {
		t.Fatalf("cashout code=%d body=%s", cashout.Code, cashout.Body.String())
	}

	balance, err := currentScratchPrizePoolBalance()
	if err != nil {
		t.Fatal(err)
	}
	if balance != 975 {
		t.Fatalf("scratch pool balance=%v, want 975 after $25 scratch payout", balance)
	}
	mainBalance, err := currentPrizePoolBalance()
	if err != nil {
		t.Fatal(err)
	}
	if mainBalance != 1000 {
		t.Fatalf("main pool balance=%v, want unchanged 1000 after scratch payout", mainBalance)
	}
	var rank, label string
	if err := db.QueryRow(`SELECT prize_rank,prize_label FROM scratch_prize_pool_ledger WHERE card_key=? AND kind=?`, key, prizePoolLedgerPayout).Scan(&rank, &label); err != nil {
		t.Fatal(err)
	}
	if rank != "scratch" || label != "刮刮乐" {
		t.Fatalf("scratch payout ledger metadata=(%q,%q)", rank, label)
	}
}

func sameScratchRewardInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
