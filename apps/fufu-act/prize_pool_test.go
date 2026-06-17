package activityapp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"fufu/activity"
	"fufu/newapi"
	"fufu/poolfundcore"
	"fufu/tokens"
)

func TestHandleLoginFundsDynamicPrizePoolOnce(t *testing.T) {
	setupScratchLockTestDB(t)
	t.Setenv("MCY_BASE_URL", "")
	t.Setenv("SHOP_BASE_URL", "")
	restoreRuntimeConfig(t)

	cfg := activity.DefaultConfig()
	cfg.DynamicPrizePool = poolfundcore.Config{
		Enabled:          true,
		ContributionRate: 0.3,
		TierEconomics: []poolfundcore.TierEconomics{
			{Dollars: 100, Revenue: 55, Cost: 20},
		},
	}
	SetRuntimeConfig(cfg)

	key := "sk-dynamic-pool-login"
	var searches atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/token/search" {
			t.Fatalf("unexpected token request %s %s", r.Method, r.URL.String())
		}
		searches.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data": []any{map[string]any{
				"id":             77,
				"key":            key,
				"name":           "100-act-test",
				"interval_quota": newapi.DefaultQuotaUnit * 100,
				"status":         1,
				"created_time":   actStartTS + 1,
			}},
		})
	}))
	t.Cleanup(server.Close)
	oldTokenSvc := tokenSvc
	tokenSvc = tokens.NewService(newapi.NewClient(newapi.Site{URL: server.URL, Token: "token", UserID: "1"}))
	t.Cleanup(func() { tokenSvc = oldTokenSvc })

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"cardKey":"`+key+`"}`))
		w := httptest.NewRecorder()
		handleLogin(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("login #%d code=%d body=%s", i+1, w.Code, w.Body.String())
		}
	}

	if searches.Load() != 2 {
		t.Fatalf("cached login should still revalidate token, searches=%d", searches.Load())
	}
	balance, err := currentPrizePoolBalance()
	if err != nil {
		t.Fatal(err)
	}
	if balance != 10.5 {
		t.Fatalf("pool balance = %v, want one contribution 10.5", balance)
	}
	var deposits int
	if err := db.QueryRow(`SELECT COUNT(*) FROM prize_pool_ledger WHERE card_key=? AND kind='deposit'`, key).Scan(&deposits); err != nil {
		t.Fatal(err)
	}
	if deposits != 1 {
		t.Fatalf("deposit rows = %d, want 1", deposits)
	}
}

func TestHandlePrizesReflectsDynamicPrizePoolBalance(t *testing.T) {
	setupScratchLockTestDB(t)
	restoreRuntimeConfig(t)
	cfg := activity.DefaultConfig()
	cfg.DynamicPrizePool = poolfundcore.Config{
		Enabled:     true,
		JackpotRate: 0.5,
		SecondRate:  0.3,
		ThirdRate:   0.2,
	}
	SetRuntimeConfig(cfg)
	insertPrizePoolLedgerForTest(t, "sk-pool-seed", "deposit", 1000, "", "")

	req := httptest.NewRequest(http.MethodGet, "/api/prizes", nil)
	w := httptest.NewRecorder()
	handlePrizes(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	var body struct {
		PoolBalance float64 `json:"poolBalance"`
		Prizes      []struct {
			Dollars int    `json:"dollars"`
			Rank    string `json:"rank"`
		} `json:"prizes"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v; body=%s", err, w.Body.String())
	}
	if body.PoolBalance != 1000 {
		t.Fatalf("poolBalance=%v, want 1000", body.PoolBalance)
	}
	if dollarsForResponseRank(body.Prizes, "jackpot") != 500 || dollarsForResponseRank(body.Prizes, "second") != 300 || dollarsForResponseRank(body.Prizes, "third") != 200 {
		t.Fatalf("dynamic advertised prizes = %#v, want jackpot/second/third 500/300/200", body.Prizes)
	}
}

func TestSpinUsesDynamicPrizePoolBalanceForAdvertisedAward(t *testing.T) {
	setupScratchLockTestDB(t)
	restoreRuntimeConfig(t)
	cfg := activity.DefaultConfig()
	cfg.PrizePool = []activity.Prize{{Type: "win", Dollars: 1000, Weight: 1, Rank: "jackpot", Label: "大奖", Advertised: true}}
	cfg.SpinGuarantees = nil
	cfg.JackpotEligibleDollars = []float64{100}
	cfg.GameConfigs = []activity.GameConfig{{Game: activity.GameSlot, TargetExpectedValue: 500, ActualExpectedValue: 500}}
	cfg.DynamicPrizePool = poolfundcore.Config{Enabled: true, JackpotRate: 0.5}
	SetRuntimeConfig(cfg)
	insertPrizePoolLedgerForTest(t, "sk-pool-seed", "deposit", 1000, "", "")

	got := spin(100, false, 0, 1, 0, 0)

	if got.Type != "win" || got.Dollars != 500 || got.Rank != "jackpot" {
		t.Fatalf("spin() = %#v, want dynamic jackpot $500", got)
	}
}

func TestRecordSpinResultDebitsAdvertisedPrizeFromDynamicPool(t *testing.T) {
	setupScratchLockTestDB(t)
	restoreRuntimeConfig(t)
	cfg := activity.DefaultConfig()
	cfg.DynamicPrizePool = poolfundcore.Config{Enabled: true}
	SetRuntimeConfig(cfg)
	key := "sk-dynamic-payout"
	if _, err := db.Exec(`INSERT INTO cards (card_key, card_name, dollars, total_spins) VALUES (?,?,?,?)`, key, "dynamic", 100, 1); err != nil {
		t.Fatal(err)
	}
	insertPrizePoolLedgerForTest(t, "sk-pool-seed", "deposit", 1000, "", "")

	err := recordSpinResult(key, Card{CardKey: key, Dollars: 100, TotalSpins: 1}, spinResult{Type: "win", Dollars: 500, Rank: "jackpot", Label: "大奖", Advertised: true}, 1)
	if err != nil {
		t.Fatal(err)
	}

	balance, err := currentPrizePoolBalance()
	if err != nil {
		t.Fatal(err)
	}
	if balance != 500 {
		t.Fatalf("pool balance = %v, want 500 after payout", balance)
	}
	var payouts int
	if err := db.QueryRow(`SELECT COUNT(*) FROM prize_pool_ledger WHERE card_key=? AND kind='payout' AND amount=-500 AND prize_rank='jackpot'`, key).Scan(&payouts); err != nil {
		t.Fatal(err)
	}
	if payouts != 1 {
		t.Fatalf("payout rows = %d, want 1", payouts)
	}
}

func restoreRuntimeConfig(t *testing.T) {
	t.Helper()
	original := SnapshotRuntimeConfig()
	t.Cleanup(func() { SetRuntimeConfig(original) })
}

func insertPrizePoolLedgerForTest(t *testing.T, key, kind string, amount float64, rank, label string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO prize_pool_ledger (card_key,kind,amount,prize_rank,prize_label) VALUES (?,?,?,?,?)`, key, kind, amount, rank, label); err != nil {
		t.Fatal(err)
	}
}

func dollarsForResponseRank(rows []struct {
	Dollars int    `json:"dollars"`
	Rank    string `json:"rank"`
}, rank string) int {
	for _, row := range rows {
		if row.Rank == rank {
			return row.Dollars
		}
	}
	return 0
}
