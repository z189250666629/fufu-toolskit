package main

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
	t.Setenv("ADMIN_TOKEN", "")
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/admin/stats?token=Chukayu98", nil)
	w := httptest.NewRecorder()

	handleAdminStats(w, req)

	if w.Code != http.StatusInternalServerError || !strings.Contains(w.Body.String(), "服务器错误") {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestHandlePrizesReturnsActivityPoolWeights(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/prizes", nil)
	w := httptest.NewRecorder()

	handlePrizes(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Prizes            []prizeWeightRow            `json:"prizes"`
		TierPools         map[string][]prizeWeightRow `json:"tierPools"`
		PostJackpotPrizes []prizeWeightRow            `json:"postJackpotPrizes"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}

	defaultDollar := findPrizeRow(t, body.Prizes, 1)
	if defaultDollar.Weight != activity.PrizePool[2].Weight || defaultDollar.TotalWeight != sumPrizeWeights(activity.PrizePool) {
		t.Fatalf("default $1 row=%+v", defaultDollar)
	}
	tierDollar := findPrizeRow(t, body.TierPools["100"], 1)
	if tierDollar.Weight != activity.TierPools[100][2].Weight || tierDollar.TotalWeight != sumPrizeWeights(activity.TierPools[100]) {
		t.Fatalf("tier $100 $1 row=%+v", tierDollar)
	}
	postJackpotDollar := findPrizeRow(t, body.PostJackpotPrizes, 20)
	if postJackpotDollar.Weight != activity.PostJackpotPool[5].Weight || postJackpotDollar.TotalWeight != sumPrizeWeights(activity.PostJackpotPool) {
		t.Fatalf("post-jackpot $20 row=%+v", postJackpotDollar)
	}
}

type prizeWeightRow struct {
	Dollars     int `json:"dollars"`
	Weight      int `json:"weight"`
	TotalWeight int `json:"totalWeight"`
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
