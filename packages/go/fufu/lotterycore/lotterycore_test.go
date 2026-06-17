package lotterycore

import "testing"

func TestBalancePoolHitsTargetExpectedValueByAdjustingNeutralWeights(t *testing.T) {
	result := BalancePool([]Prize{
		{Type: "miss", Weight: 100},
		{Type: "retry", Weight: 100},
		{Type: "win", Dollars: 9, Weight: 1},
	}, 4.5)

	if !result.Reached || result.ActualExpectedValue != 4.5 {
		t.Fatalf("balance result = %#v, want reached EV 4.5", result)
	}
	if got := totalWeight(result.Pool); got != 2 {
		t.Fatalf("balanced total weight = %d, want 2", got)
	}
	if got := winWeight(result.Pool); got != 1 {
		t.Fatalf("win weight should stay unchanged, got %d", got)
	}
	if got := neutralWeight(result.Pool); got != 1 {
		t.Fatalf("neutral weight should be adjusted to 1, got %d", got)
	}
}

func TestBalancePoolForPlanUsesGameExpectedValuesToCalculatePerDrawTarget(t *testing.T) {
	result := BalancePoolForPlan(BalanceInput{
		Pool: []Prize{
			{Type: "miss", Weight: 100},
			{Type: "win", Dollars: 9, Weight: 1},
		},
		TargetExpectedValue: 4,
		ActualExpectedValue: 2,
		DrawCount:           4,
	})

	if result.TargetPerDrawExpectedValue != 1.5 {
		t.Fatalf("target per draw EV = %v, want 1.5", result.TargetPerDrawExpectedValue)
	}
	if result.ActualExpectedValue != 1.5 {
		t.Fatalf("balanced EV = %v, want 1.5", result.ActualExpectedValue)
	}
	if got := totalWeight(result.Pool); got != 6 {
		t.Fatalf("balanced total weight = %d, want 6", got)
	}
}

func TestBalancePoolForPlanLowersExpectedValueWhenActualIsAboveTarget(t *testing.T) {
	result := BalancePoolForPlan(BalanceInput{
		Pool: []Prize{
			{Type: "miss", Weight: 1},
			{Type: "win", Dollars: 9, Weight: 1},
		},
		TargetExpectedValue: 4,
		ActualExpectedValue: 8,
		DrawCount:           4,
	})

	if result.TargetPerDrawExpectedValue != 0 {
		t.Fatalf("target per draw EV = %v, want 0", result.TargetPerDrawExpectedValue)
	}
	if result.ActualExpectedValue != 0 {
		t.Fatalf("balanced EV = %v, want 0", result.ActualExpectedValue)
	}
	if got := winWeight(result.Pool); got != 0 {
		t.Fatalf("balanced win weight = %d, want 0", got)
	}
}

func TestBalancePoolPreservesWinningPrizeRatio(t *testing.T) {
	result := BalancePool([]Prize{
		{Type: "miss", Weight: 10},
		{Type: "win", Dollars: 2, Weight: 1},
		{Type: "win", Dollars: 10, Weight: 3},
	}, 4)

	if !result.Reached || result.ActualExpectedValue != 4 {
		t.Fatalf("balance result = %#v, want reached EV 4", result)
	}
	for _, prize := range result.Pool {
		if prize.Type == "win" && prize.Dollars == 2 && prize.Weight != 1 {
			t.Fatalf("$2 winning weight changed: %#v", result.Pool)
		}
		if prize.Type == "win" && prize.Dollars == 10 && prize.Weight != 3 {
			t.Fatalf("$10 winning weight changed: %#v", result.Pool)
		}
	}
}

func TestBalancePoolReportsUnreachableHighTarget(t *testing.T) {
	result := BalancePool([]Prize{
		{Type: "miss", Weight: 10},
		{Type: "win", Dollars: 10, Weight: 1},
	}, 20)

	if result.Reached {
		t.Fatalf("target above win-only EV should be unreachable: %#v", result)
	}
	if result.ActualExpectedValue != 10 {
		t.Fatalf("unreachable high target should clamp to win-only EV 10, got %#v", result)
	}
	if got := neutralWeight(result.Pool); got != 0 {
		t.Fatalf("unreachable high target should remove neutral weight, got %d", got)
	}
}

func TestRollUsesBalancedWeightRange(t *testing.T) {
	result := BalancePool([]Prize{
		{Type: "miss", Weight: 100},
		{Type: "win", Dollars: 9, Weight: 1},
	}, 4.5)

	got := Roll(result.Pool, func(max int) int {
		if max != 2 {
			t.Fatalf("roll total = %d, want balanced total 2", max)
		}
		return 1
	})
	if got.Type != "win" || got.Dollars != 9 {
		t.Fatalf("roll = %#v, want $9 win", got)
	}
}

func totalWeight(pool []Prize) int {
	total := 0
	for _, prize := range pool {
		total += prize.Weight
	}
	return total
}

func winWeight(pool []Prize) int {
	total := 0
	for _, prize := range pool {
		if prize.Type == "win" {
			total += prize.Weight
		}
	}
	return total
}

func neutralWeight(pool []Prize) int {
	total := 0
	for _, prize := range pool {
		if prize.Type != "win" {
			total += prize.Weight
		}
	}
	return total
}
