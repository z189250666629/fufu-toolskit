package prizepoolcore

import "testing"

func TestBalancePoolKeepsAdvertisedPrizeMetadataSeparateFromProbability(t *testing.T) {
	result := BalancePoolForPlan(BalanceInput{
		Pool: []Prize{
			{Type: "miss", Weight: 1},
			{Type: "win", Dollars: 1000, Weight: 1, Rank: "jackpot", Label: "大奖", Advertised: true},
			{Type: "win", Dollars: 300, Weight: 1, Rank: "second", Label: "二等奖", Advertised: true},
			{Type: "win", Dollars: 100, Weight: 1, Rank: "third", Label: "三等奖", Advertised: true},
		},
		TargetExpectedValue: 100,
		ActualExpectedValue: 100,
		DrawCount:           10,
	})

	if result.TargetPerDrawExpectedValue != 10 {
		t.Fatalf("target per draw EV = %v, want 10", result.TargetPerDrawExpectedValue)
	}
	for _, rank := range []string{"jackpot", "second", "third"} {
		if !hasAdvertisedRank(result.Pool, rank) {
			t.Fatalf("balanced pool should preserve advertised rank %q: %#v", rank, result.Pool)
		}
	}
	if result.ActualExpectedValue != 10 || !result.Reached {
		t.Fatalf("balanced result = %#v, want reached EV 10", result)
	}
}

func TestApplyWeightsCanKeepAdvertisedZeroProbabilityPrizesVisible(t *testing.T) {
	pool := ApplyWeights([]Prize{
		{Type: "miss", Weight: 1},
		{Type: "win", Dollars: 1000, Weight: 1, Rank: "jackpot", Advertised: true},
		{Type: "win", Dollars: 20, Weight: 1},
	}, []Weight{
		{ID: "0", Weight: 1},
		{ID: "1", Weight: 0},
		{ID: "2", Weight: 0},
	})

	if !hasAdvertisedRank(pool, "jackpot") {
		t.Fatalf("advertised jackpot should remain visible with zero probability: %#v", pool)
	}
	if got := ExpectedValue(pool); got != 0 {
		t.Fatalf("ExpectedValue = %v, want 0 after zeroing winning probability", got)
	}
	got := Roll(pool, func(max int) int { return max - 1 })
	if got.Type != "miss" {
		t.Fatalf("zero-probability advertised jackpot must not be rolled, got %#v", got)
	}
}

func TestOutcomesExposeOnlyValueAndWeightToProbabilityCore(t *testing.T) {
	outcomes := Outcomes([]Prize{
		{Type: "miss", Weight: 2, Label: "未中奖"},
		{Type: "win", Dollars: 10, Weight: 1, Rank: "jackpot", Label: "大奖", Advertised: true},
	})

	if len(outcomes) != 2 {
		t.Fatalf("outcomes length = %d, want 2", len(outcomes))
	}
	if outcomes[1].ID != "1" || outcomes[1].Value != 10 || outcomes[1].Weight != 1 {
		t.Fatalf("winning outcome = %#v, want id/value/weight only", outcomes[1])
	}
}

func hasAdvertisedRank(pool []Prize, rank string) bool {
	for _, prize := range pool {
		if prize.Rank == rank && prize.Advertised {
			return true
		}
	}
	return false
}
