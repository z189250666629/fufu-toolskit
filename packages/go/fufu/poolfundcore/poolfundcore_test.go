package poolfundcore

import "testing"

func TestContributionUsesPositiveNetProfitOnly(t *testing.T) {
	got := Contribution(Economics{Revenue: 55, Cost: 20}, 0.3)

	if got.Revenue != 55 || got.Cost != 20 || got.NetProfit != 35 || got.Contribution != 10.5 {
		t.Fatalf("Contribution() = %#v, want revenue 55 cost 20 net 35 contribution 10.5", got)
	}

	loss := Contribution(Economics{Revenue: 10, Cost: 20}, 0.3)
	if loss.NetProfit != -10 || loss.Contribution != 0 {
		t.Fatalf("loss Contribution() = %#v, want raw net -10 and zero pool contribution", loss)
	}
}

func TestAllocatePrizeAmountsUsesPoolPercentages(t *testing.T) {
	got := AllocatePrizeAmounts(1000, Config{
		Enabled:     true,
		JackpotRate: 0.5,
		SecondRate:  0.3,
		ThirdRate:   0.2,
	})

	if got.Jackpot != 500 || got.Second != 300 || got.Third != 200 {
		t.Fatalf("AllocatePrizeAmounts() = %#v, want 500/300/200", got)
	}
	if got.JackpotDollars != 500 || got.SecondDollars != 300 || got.ThirdDollars != 200 {
		t.Fatalf("whole-dollar amounts = %#v, want 500/300/200", got)
	}
}

func TestAllocatePrizeAmountsFloorsToWholeDollars(t *testing.T) {
	got := AllocatePrizeAmounts(999.9, Config{
		Enabled:     true,
		JackpotRate: 0.5,
		SecondRate:  0.3,
		ThirdRate:   0.2,
	})

	if got.JackpotDollars != 499 || got.SecondDollars != 299 || got.ThirdDollars != 199 {
		t.Fatalf("whole-dollar amounts = %#v, want floor 499/299/199", got)
	}
}

func TestTierEconomicsLooksUpConfiguredDollarTier(t *testing.T) {
	cfg := Config{TierEconomics: []TierEconomics{
		{Dollars: 55, Revenue: 55, Cost: 20},
		{Dollars: 100, Revenue: 100, Cost: 60},
	}}

	got, ok := TierEconomicsForDollars(cfg, 55)
	if !ok || got.Revenue != 55 || got.Cost != 20 {
		t.Fatalf("TierEconomicsForDollars(55) = %#v/%v", got, ok)
	}
	if _, ok := TierEconomicsForDollars(cfg, 56); ok {
		t.Fatal("unconfigured tier should not have economics")
	}
}

func TestIsPayoutPrizeOnlyAcceptsAdvertisedMajorWins(t *testing.T) {
	cases := []struct {
		name string
		in   PayoutPrize
		want bool
	}{
		{"jackpot advertised win", PayoutPrize{Type: "win", Dollars: 500, Rank: "jackpot", Advertised: true}, true},
		{"second advertised win", PayoutPrize{Type: "win", Dollars: 200, Rank: "second", Advertised: true}, true},
		{"third advertised win", PayoutPrize{Type: "win", Dollars: 100, Rank: "third", Advertised: true}, true},
		{"normal win is not dynamic payout", PayoutPrize{Type: "win", Dollars: 20, Rank: "", Advertised: true}, false},
		{"not advertised", PayoutPrize{Type: "win", Dollars: 500, Rank: "jackpot"}, false},
		{"zero dollars", PayoutPrize{Type: "win", Rank: "jackpot", Advertised: true}, false},
		{"miss", PayoutPrize{Type: "miss", Dollars: 500, Rank: "jackpot", Advertised: true}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := IsPayoutPrize(c.in); got != c.want {
				t.Fatalf("IsPayoutPrize(%#v)=%v, want %v", c.in, got, c.want)
			}
		})
	}
}
