package spincore

import "testing"

func TestSpinGuaranteeForThousandCard(t *testing.T) {
	cfg := Config{GuaranteeRules: []GuaranteeRule{{DollarTier: 1000, RemainingSpins: 1, MaxWonBelow: 50, PrizeDollars: 100}}}
	got := Spin(cfg, 1000, false, 9, 10, 0, 0, func(max int) int { return 0 })
	if got.Type != "win" || got.Dollars != 100 {
		t.Fatalf("Spin guarantee = %#v, want $100 win", got)
	}
}

func TestSpinUsesConfiguredPrizeWeights(t *testing.T) {
	cfg := Config{
		PrizePool: []Prize{{Type: "miss", Weight: 1}, {Type: "win", Dollars: 7, Weight: 1}},
	}
	got := Spin(cfg, 42, false, 0, 3, 0, 0, func(max int) int {
		if max != 2 {
			t.Fatalf("roll total=%d, want 2", max)
		}
		return 1
	})
	if got.Type != "win" || got.Dollars != 7 {
		t.Fatalf("Spin = %#v, want $7 win", got)
	}
}

func TestSpinReturnsAdvertisedPrizeMetadataForPrompting(t *testing.T) {
	cfg := Config{
		PrizePool: []Prize{
			{Type: "miss", Weight: 1},
			{Type: "win", Dollars: 1000, Weight: 1, Rank: "jackpot", Label: "大奖", Advertised: true},
		},
		JackpotEligibleDollars: []float64{100},
		JackpotPrizeDollars:    1000,
	}
	got := Spin(cfg, 100, false, 0, 1, 0, 0, func(max int) int { return 1 })
	if got.Type != "win" || got.Dollars != 1000 || got.Rank != "jackpot" || got.Label != "大奖" || !got.Advertised {
		t.Fatalf("Spin metadata = %#v, want advertised jackpot result", got)
	}
}

func TestSpinBalancesPrizePoolToTargetExpectedValue(t *testing.T) {
	cfg := Config{
		TargetExpectedValue: 4.5,
		ActualExpectedValue: 4.5,
		DrawCount:           1,
		PrizePool: []Prize{
			{Type: "miss", Weight: 100},
			{Type: "retry", Weight: 100},
			{Type: "win", Dollars: 9, Weight: 1},
		},
	}
	got := Spin(cfg, 100, false, 0, 3, 0, 0, func(max int) int {
		if max != 2 {
			t.Fatalf("roll total=%d, want balanced total 2", max)
		}
		return 1
	})
	if got.Type != "win" || got.Dollars != 9 {
		t.Fatalf("Spin = %#v, want $9 win", got)
	}
}

func TestSpinUsesSameBalancedPoolAcrossDollarTiers(t *testing.T) {
	cfg := Config{
		TargetExpectedValue: 4.5,
		ActualExpectedValue: 4.5,
		DrawCount:           1,
		PrizePool: []Prize{
			{Type: "miss", Weight: 100},
			{Type: "win", Dollars: 9, Weight: 1},
		},
	}
	for _, dollars := range []float64{100, 500, 1000} {
		Spin(cfg, dollars, false, 0, 3, 0, 0, func(max int) int {
			if max != 2 {
				t.Fatalf("tier %.0f roll total=%d, want unified balanced total 2", dollars, max)
			}
			return 0
		})
	}
}

func TestSpinDoesNotSwitchPrizePoolAfterJackpot(t *testing.T) {
	cfg := Config{
		TargetExpectedValue: 4.5,
		ActualExpectedValue: 4.5,
		DrawCount:           1,
		PrizePool: []Prize{
			{Type: "miss", Weight: 100},
			{Type: "win", Dollars: 9, Weight: 1},
		},
	}
	got := Spin(cfg, 100, true, 0, 3, 60, 0, func(max int) int {
		if max != 2 {
			t.Fatalf("roll total=%d, want unified balanced pool total 2", max)
		}
		return 1
	})
	if got.Type != "win" || got.Dollars != 9 {
		t.Fatalf("Spin should keep unified pool after jackpot, got %#v", got)
	}
}

func TestSpinBlocksJackpotForIneligibleTier(t *testing.T) {
	cfg := Config{PrizePool: []Prize{{Type: "win", Dollars: 1000, Weight: 1}}, JackpotPrizeDollars: 1000, JackpotEligibleDollars: []float64{1000}}
	got := Spin(cfg, 100, false, 0, 3, 0, 0, func(max int) int { return 0 })
	if got.Type != "retry" || got.Dollars != 0 {
		t.Fatalf("Spin should retry jackpot for ineligible tier, got %#v", got)
	}
}

func TestRollClampsRandomInput(t *testing.T) {
	pool := []Prize{{Type: "miss", Weight: 1}, {Type: "win", Dollars: 5, Weight: 1}}
	if got := Roll(pool, func(max int) int { return -10 }); got.Type != "miss" {
		t.Fatalf("negative random should clamp to first prize, got %#v", got)
	}
	if got := Roll(pool, func(max int) int { return 99 }); got.Type != "win" || got.Dollars != 5 {
		t.Fatalf("large random should clamp to last prize, got %#v", got)
	}
}
