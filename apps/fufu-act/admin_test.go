package activityapp

import (
	"encoding/json"
	"fufu/activity"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleAdminStatsReturns500OnStatsQueryError(t *testing.T) {
	setupScratchLockTestDB(t)
	t.Setenv("ADMIN_TOKEN", "test-admin-token")
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/admin/stats", nil)
	req.Header.Set("Authorization", "Bearer test-admin-token")
	w := httptest.NewRecorder()

	handleAdminStats(w, req)

	if w.Code != http.StatusInternalServerError || !strings.Contains(w.Body.String(), "服务器错误") {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestHandleAdminStatsRejectsQueryToken(t *testing.T) {
	setupScratchLockTestDB(t)
	t.Setenv("ADMIN_TOKEN", "test-admin-token")

	req := httptest.NewRequest(http.MethodGet, "/api/admin/stats?token=test-admin-token", nil)
	w := httptest.NewRecorder()

	handleAdminStats(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("query token should be rejected: code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestHandleAdminStatsRejectsDefaultTokenWhenAdminTokenUnset(t *testing.T) {
	setupScratchLockTestDB(t)
	t.Setenv("ADMIN_TOKEN", "")

	req := httptest.NewRequest(http.MethodGet, "/api/admin/stats?token=Chukayu98", nil)
	w := httptest.NewRecorder()

	handleAdminStats(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("default token should be rejected when ADMIN_TOKEN is unset: code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestHandlePrizesReturnsActivityPoolWeights(t *testing.T) {
	original := SnapshotRuntimeConfig()
	t.Cleanup(func() { SetRuntimeConfig(original) })

	req := httptest.NewRequest(http.MethodGet, "/api/prizes", nil)
	w := httptest.NewRecorder()

	handlePrizes(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	if _, ok := raw["tierPools"]; ok {
		t.Fatalf("/api/prizes should expose one unified prize pool only, got %s", w.Body.String())
	}
	var body struct {
		Prizes []prizeWeightRow `json:"prizes"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if _, ok := raw["postJackpotPrizes"]; ok {
		t.Fatalf("/api/prizes should not expose a second post-jackpot pool, got %s", w.Body.String())
	}

	defaultPrizePool := activity.DefaultPrizePool()
	defaultDollar := findPrizeRow(t, body.Prizes, 1)
	if defaultDollar.Weight != defaultPrizePool[2].Weight || defaultDollar.TotalWeight != sumPrizeWeights(defaultPrizePool) {
		t.Fatalf("default $1 row=%+v", defaultDollar)
	}
	jackpot := findPrizeRow(t, body.Prizes, 1000)
	if jackpot.Rank != "jackpot" || jackpot.Label != "大奖" || !jackpot.Advertised {
		t.Fatalf("jackpot row should expose prompt metadata, got %+v", jackpot)
	}
}

func TestHandlePrizesReturnsBalancedActivityPoolWeights(t *testing.T) {
	original := SnapshotRuntimeConfig()
	t.Cleanup(func() { SetRuntimeConfig(original) })

	cfg := activity.DefaultConfig()
	cfg.GameConfigs = []activity.GameConfig{{Game: activity.GameSlot, TargetExpectedValue: 4.5, ActualExpectedValue: 4.5}}
	cfg.PrizePool = []activity.Prize{
		{Type: "miss", Weight: 100},
		{Type: "win", Dollars: 9, Weight: 1},
	}
	SetRuntimeConfig(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/prizes", nil)
	w := httptest.NewRecorder()
	handlePrizes(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Prizes []prizeWeightRow `json:"prizes"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	row := findPrizeRow(t, body.Prizes, 9)
	if row.Weight != 1 || row.TotalWeight != 2 {
		t.Fatalf("/api/prizes should expose balanced total weight 2, got %+v body=%s", row, w.Body.String())
	}
}

type prizeWeightRow struct {
	Dollars     int    `json:"dollars"`
	Weight      int    `json:"weight"`
	TotalWeight int    `json:"totalWeight"`
	Rank        string `json:"rank"`
	Label       string `json:"label"`
	Advertised  bool   `json:"advertised"`
}

func findPrizeRow(t *testing.T, rows []prizeWeightRow, dollars int) prizeWeightRow {
	t.Helper()
	for _, row := range rows {
		if row.Dollars == dollars {
			return row
		}
	}
	t.Fatalf("missing prize row for $%d in %+v", dollars, rows)
	return prizeWeightRow{}
}

func sumPrizeWeights(pool []activity.Prize) int {
	total := 0
	for _, prize := range pool {
		total += prize.Weight
	}
	return total
}
