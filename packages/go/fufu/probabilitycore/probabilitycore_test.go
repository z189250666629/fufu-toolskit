package probabilitycore

import "testing"

func TestBalanceWeightsUsesGameExpectedValuesAndDrawCount(t *testing.T) {
	result := BalanceWeights(BalanceInput{
		Outcomes: []Outcome{
			{ID: "miss", Value: 0, Weight: 1},
			{ID: "jackpot", Value: 6, Weight: 1},
		},
		TargetExpectedValue: 4,
		ActualExpectedValue: 2,
		DrawCount:           4,
	})

	if result.TargetPerDrawExpectedValue != 1.5 {
		t.Fatalf("target per draw EV = %v, want 1.5", result.TargetPerDrawExpectedValue)
	}
	if result.ActualExpectedValue != 1.5 || !result.Reached {
		t.Fatalf("balanced result = %#v, want reached per-draw EV 1.5", result)
	}
	if got := weightFor(result.Weights, "miss"); got != 3 {
		t.Fatalf("miss weight = %d, want 3", got)
	}
	if got := weightFor(result.Weights, "jackpot"); got != 1 {
		t.Fatalf("jackpot weight = %d, want 1", got)
	}
}

func TestBalanceWeightsLowersProbabilityWhenActualExpectedValueIsHigh(t *testing.T) {
	result := BalanceWeights(BalanceInput{
		Outcomes: []Outcome{
			{ID: "miss", Value: 0, Weight: 1},
			{ID: "jackpot", Value: 6, Weight: 1},
		},
		TargetExpectedValue: 4,
		ActualExpectedValue: 6,
		DrawCount:           4,
	})

	if result.TargetPerDrawExpectedValue != 0.5 {
		t.Fatalf("target per draw EV = %v, want 0.5", result.TargetPerDrawExpectedValue)
	}
	if result.ActualExpectedValue != 0.5 || !result.Reached {
		t.Fatalf("balanced result = %#v, want reached per-draw EV 0.5", result)
	}
	if got := weightFor(result.Weights, "miss"); got != 11 {
		t.Fatalf("miss weight = %d, want 11", got)
	}
}

func TestBalanceWeightsClampsOverpaidPlanToZeroProbability(t *testing.T) {
	result := BalanceWeights(BalanceInput{
		Outcomes: []Outcome{
			{ID: "miss", Value: 0, Weight: 1},
			{ID: "jackpot", Value: 6, Weight: 1},
		},
		TargetExpectedValue: 4,
		ActualExpectedValue: 8,
		DrawCount:           4,
	})

	if result.TargetPerDrawExpectedValue != 0 {
		t.Fatalf("target per draw EV = %v, want 0", result.TargetPerDrawExpectedValue)
	}
	if result.ActualExpectedValue != 0 || !result.Reached {
		t.Fatalf("balanced result = %#v, want reached EV 0", result)
	}
	if got := weightFor(result.Weights, "jackpot"); got != 0 {
		t.Fatalf("jackpot weight = %d, want 0", got)
	}
	if got := weightFor(result.Weights, "miss"); got <= 0 {
		t.Fatalf("miss weight = %d, want a remaining neutral outcome", got)
	}
}

func TestExpectedValueUsesOnlyOutcomeValuesAndWeights(t *testing.T) {
	got := ExpectedValue([]Outcome{
		{ID: "miss", Value: 0, Weight: 2},
		{ID: "second", Value: 4, Weight: 1},
		{ID: "jackpot", Value: 10, Weight: 1},
	})

	if got != 3.5 {
		t.Fatalf("ExpectedValue = %v, want 3.5", got)
	}
}

func weightFor(weights []Weight, id string) int {
	for _, weight := range weights {
		if weight.ID == id {
			return weight.Weight
		}
	}
	return 0
}
