package activity

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestConfigMarshalsPrizesWithLowercaseJSONKeys(t *testing.T) {
	// The unified admin config editor renders activity.Config as JSON. Prize must
	// serialize with lowercase type/dollars/weight so the editor surface matches
	// /api/prizes and stays consistent across the app.
	data, err := json.Marshal(DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	out := string(data)
	for _, want := range []string{`"type":`, `"dollars":`, `"weight":`} {
		if !strings.Contains(out, want) {
			t.Fatalf("config JSON should contain lowercase prize key %q, got %s", want, out)
		}
	}
	for _, notWant := range []string{`"Type":`, `"Dollars":`, `"Weight":`} {
		if strings.Contains(out, notWant) {
			t.Fatalf("config JSON should not contain capitalized prize key %q, got %s", notWant, out)
		}
	}

	// Round-trip must remain lossless and case-insensitive on input.
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("round-trip unmarshal: %v", err)
	}
	if len(cfg.PrizePool) != len(DefaultConfig().PrizePool) {
		t.Fatalf("prize pool round-trip lost entries: %d", len(cfg.PrizePool))
	}
}

func TestSpinGuaranteeForThousandCard(t *testing.T) {
	got := Spin(1000, false, 9, 10, 0, 0, func(max int) int { return 0 })
	if got.Type != "win" || got.Dollars != 100 {
		t.Fatalf("unexpected guarantee result: %#v", got)
	}
}

func TestDollarsTier(t *testing.T) {
	if DollarsTier(1_500_000, 500_000) != 3 {
		t.Fatalf("bad tier")
	}
}

func TestDefaultConfigReturnsIndependentCopies(t *testing.T) {
	spinMap := DefaultSpinMap()
	spinMap[100] = 999
	if DefaultSpinMap()[100] == 999 {
		t.Fatalf("DefaultSpinMap should return an independent map copy")
	}

	prizePool := DefaultPrizePool()
	prizePool[2].Weight = 1
	if DefaultPrizePool()[2].Weight == 1 {
		t.Fatalf("DefaultPrizePool should return an independent slice copy")
	}

	tierPools := DefaultTierPools()
	tierPools[100][2].Weight = 1
	if DefaultTierPools()[100][2].Weight == 1 {
		t.Fatalf("DefaultTierPools should return independent nested slice copies")
	}

	postJackpotPool := DefaultPostJackpotPool()
	postJackpotPool[2].Weight = 1
	if DefaultPostJackpotPool()[2].Weight == 1 {
		t.Fatalf("DefaultPostJackpotPool should return an independent slice copy")
	}

	scratchRewards := DefaultScratchRewards()
	scratchRewards[0] = 999
	if DefaultScratchRewards()[0] == 999 {
		t.Fatalf("DefaultScratchRewards should return an independent slice copy")
	}
}

func TestSpinWithConfigUsesRuntimePrizeWeights(t *testing.T) {
	cfg := DefaultConfig()
	cfg.SpinMap = map[float64]int{42: 3}
	cfg.TierPools = map[int][]Prize{}
	cfg.PostJackpotPool = []Prize{{Type: "miss", Weight: 1}}
	cfg.PrizePool = []Prize{
		{Type: "miss", Weight: 1},
		{Type: "win", Dollars: 7, Weight: 1},
	}

	got := SpinWithConfig(cfg, 42, false, 0, 3, 0, 0, func(max int) int {
		if max != 2 {
			t.Fatalf("roll total = %d, want 2", max)
		}
		return 1
	})

	if got.Type != "win" || got.Dollars != 7 {
		t.Fatalf("SpinWithConfig = %#v, want $7 win", got)
	}
}

func TestExpectedValueUsesWinningWeightsOnly(t *testing.T) {
	got := ExpectedValue([]Prize{
		{Type: "miss", Weight: 2},
		{Type: "retry", Weight: 1},
		{Type: "win", Dollars: 9, Weight: 3},
	})

	if got != 4.5 {
		t.Fatalf("ExpectedValue = %v, want 4.5", got)
	}
}
