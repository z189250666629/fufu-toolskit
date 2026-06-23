package activity

import (
	"encoding/json"
	"strings"
	"testing"

	"fufu/poolfundcore"
)

func TestGameRoutesConfigurableDriveGameRouting(t *testing.T) {
	def := DefaultConfig()
	if def.GameForTier(55) != GameScratch || def.GameForTier(100) != GameSlot {
		t.Fatalf("default game routes should send 55 to scratch and 100 to slot, got %#v", def.GameRoutes)
	}

	// Configurable: dedup + drop non-positive / invalid games.
	cfg := NormalizeConfig(Config{GameRoutes: []GameRoute{
		{Dollars: 100, Game: GameScratch},
		{Dollars: 100, Game: GameScratch},
		{Dollars: 0, Game: GameScratch},
		{Dollars: 300, Game: GameScratch},
		{Dollars: 500, Game: "unknown"},
	}})
	if !cfg.IsScratchTier(100) || !cfg.IsScratchTier(300) || cfg.IsScratchTier(55) {
		t.Fatalf("custom game routes wrong: %#v", cfg.GameRoutes)
	}
	if len(cfg.GameRoutes) != 2 || len(cfg.ScratchTiers) != 2 {
		t.Fatalf("game routes should dedup + project scratch tiers: routes=%#v tiers=%#v", cfg.GameRoutes, cfg.ScratchTiers)
	}

	// Explicit empty list = no scratch (every card plays the slot machine).
	empty := NormalizeConfig(Config{GameRoutes: []GameRoute{}})
	if empty.IsScratchTier(55) || len(empty.GameRoutes) != 0 || len(empty.ScratchTiers) != 0 {
		t.Fatalf("explicit empty game routes should disable scratch, got routes=%#v tiers=%#v", empty.GameRoutes, empty.ScratchTiers)
	}

	// Survives JSON round-trip (admin saves the whole Config as JSON).
	raw, err := json.Marshal(NormalizeConfig(Config{GameRoutes: []GameRoute{{Dollars: 55, Game: GameScratch}, {Dollars: 99, Game: GameScratch}}}))
	if err != nil {
		t.Fatal(err)
	}
	var back Config
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if !back.IsScratchTier(55) || !back.IsScratchTier(99) || len(back.GameRoutes) != 2 {
		t.Fatalf("game routes should survive JSON round-trip: %s", raw)
	}
}

func TestLegacyScratchTiersProjectToGameRoutes(t *testing.T) {
	cfg := NormalizeConfig(Config{ScratchTiers: []int{150, 150, 0, 55}})
	if cfg.GameForTier(150) != GameScratch || cfg.GameForTier(55) != GameScratch || cfg.GameForTier(100) != GameSlot {
		t.Fatalf("legacy scratch tiers should project to game routes: %#v", cfg.GameRoutes)
	}
	if len(cfg.GameRoutes) != 2 {
		t.Fatalf("legacy scratch tiers should dedup into two game routes: %#v", cfg.GameRoutes)
	}
}

func TestSubscriptionPlanMappingsNormalizeAndRoundTrip(t *testing.T) {
	cfg := NormalizeConfig(Config{SubscriptionPlanMappings: []SubscriptionPlanMapping{
		{PlanID: 77, Dollars: 100},
		{Title: " VIP 月卡 ", Dollars: 150},
		{Title: "bad", Dollars: 0},
		{Dollars: 300},
	}})
	if len(cfg.SubscriptionPlanMappings) != 2 {
		t.Fatalf("valid subscription mappings should be kept only: %#v", cfg.SubscriptionPlanMappings)
	}
	if cfg.SubscriptionPlanMappings[1].Title != "VIP 月卡" || cfg.SubscriptionPlanMappings[1].Match != "contains" {
		t.Fatalf("title mapping should be trimmed and default to contains: %#v", cfg.SubscriptionPlanMappings[1])
	}

	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var back Config
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if len(back.SubscriptionPlanMappings) != 2 || back.SubscriptionPlanMappings[0].PlanID != 77 || back.SubscriptionPlanMappings[1].Dollars != 150 {
		t.Fatalf("subscription mappings should survive JSON round-trip: cfg=%#v raw=%s", back.SubscriptionPlanMappings, raw)
	}
}

func TestGameConfigsDriveExpectedValuesOnly(t *testing.T) {
	cfg := NormalizeConfig(Config{GameConfigs: []GameConfig{
		{Game: GameSlot, TargetExpectedValue: 4.5, ActualExpectedValue: 3.5},
		{Game: GameScratch, TargetExpectedValue: 2.5, ActualExpectedValue: 2},
		{Game: "", TargetExpectedValue: 99},
	}})

	slot := cfg.GameConfigFor(GameSlot)
	if slot.TargetExpectedValue != 4.5 || slot.ActualExpectedValue != 3.5 {
		t.Fatalf("slot game config = %#v", slot)
	}
	scratch := cfg.GameConfigFor(GameScratch)
	if scratch.TargetExpectedValue != 2.5 || scratch.ActualExpectedValue != 2 {
		t.Fatalf("scratch game config = %#v", scratch)
	}
	if cfg.DrawCountForTier(100) != 1 || cfg.DrawCountForTier(300) != 3 || cfg.DrawCountForTier(55) != 1 {
		t.Fatalf("draw count should come from tier config before gameplay defaults, got 100=%d 300=%d 55=%d", cfg.DrawCountForTier(100), cfg.DrawCountForTier(300), cfg.DrawCountForTier(55))
	}
}

func TestGameRoutesCanOverrideDrawCountByTier(t *testing.T) {
	cfg := NormalizeConfig(Config{
		GameConfigs: []GameConfig{
			{Game: GameSlot, TargetExpectedValue: 4.5, ActualExpectedValue: 3.5},
			{Game: GameScratch, TargetExpectedValue: 2.5, ActualExpectedValue: 2},
		},
		GameRoutes: []GameRoute{
			{Dollars: 100, Game: GameSlot, DrawCount: 9},
			{Dollars: 300, Game: GameSlot, DrawCount: 4},
			{Dollars: 55, Game: GameScratch, DrawCount: 1},
		},
	})

	if cfg.DrawCountForTier(100) != 9 || cfg.DrawCountForTier(300) != 4 || cfg.DrawCountForTier(55) != 1 {
		t.Fatalf("route draw counts not honored: 100=%d 300=%d 55=%d routes=%#v", cfg.DrawCountForTier(100), cfg.DrawCountForTier(300), cfg.DrawCountForTier(55), cfg.GameRoutes)
	}

	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var back Config
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if back.DrawCountForTier(100) != 9 || !strings.Contains(string(raw), `"drawCount":9`) {
		t.Fatalf("route draw count should survive JSON round-trip: cfg=%#v raw=%s", back.GameRoutes, raw)
	}
}

func TestDragonBoatGameRouteUsesTierDrawCount(t *testing.T) {
	cfg := NormalizeConfig(Config{
		GameRoutes: []GameRoute{{Dollars: 55, Game: "端午", DrawCount: 7}},
	})
	if cfg.GameForTier(55) != GameDragon || cfg.DrawCountForTier(55) != 7 {
		t.Fatalf("dragon route not normalized: routes=%#v game=%q draws=%d", cfg.GameRoutes, cfg.GameForTier(55), cfg.DrawCountForTier(55))
	}
	if cfg.GameConfigFor(GameDragon).Game != GameDragon {
		t.Fatalf("dragon game config missing: %#v", cfg.GameConfigs)
	}
}

func TestLegacyGameConfigDrawCountMigratesToCardRoutes(t *testing.T) {
	var cfg Config
	if err := json.Unmarshal([]byte(`{
		"gameConfigs": [
			{"game":"slot","targetExpectedValue":4.5,"actualExpectedValue":4.5,"drawCount":3},
			{"game":"scratch","targetExpectedValue":2.5,"actualExpectedValue":2.5,"drawCount":2}
		],
		"gameRoutes": [
			{"dollars":55,"game":"scratch"},
			{"dollars":100,"game":"slot","drawCount":9}
		]
	}`), &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.DrawCountForTier(55) != 2 || cfg.DrawCountForTier(100) != 9 {
		t.Fatalf("legacy draw counts should migrate to routes without overriding explicit tier counts: routes=%#v", cfg.GameRoutes)
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), `"game":"slot","targetExpectedValue":4.5,"actualExpectedValue":4.5,"drawCount"`) {
		t.Fatalf("gameConfigs should no longer serialize drawCount: %s", raw)
	}
	if !strings.Contains(string(raw), `"dollars":55`) || !strings.Contains(string(raw), `"drawCount":2`) {
		t.Fatalf("migrated route drawCount should survive JSON: %s", raw)
	}
}

func TestScratchRouteDoesNotInheritSlotSpinMapDrawCount(t *testing.T) {
	cfg := NormalizeConfig(Config{
		GameConfigs: []GameConfig{
			{Game: GameSlot},
			{Game: GameScratch},
		},
		GameRoutes: []GameRoute{{Dollars: 300, Game: GameScratch}},
	})

	if got := cfg.DrawCountForTier(300); got != 1 {
		t.Fatalf("scratch route draw count=%d, want scratch default instead of slot spinMap", got)
	}
}

func TestDynamicPrizePoolUpdatesAdvertisedPrizeAmounts(t *testing.T) {
	cfg := DefaultConfig()
	cfg.DynamicPrizePool = poolfundcore.Config{
		Enabled:     true,
		JackpotRate: 0.5,
		SecondRate:  0.3,
		ThirdRate:   0.2,
	}

	got := PrizePoolWithDynamicAwards(cfg, 1000)

	if prizeDollarsByRank(got, "jackpot") != 500 {
		t.Fatalf("jackpot dollars = %d, want 500 in %#v", prizeDollarsByRank(got, "jackpot"), got)
	}
	if prizeDollarsByRank(got, "second") != 300 {
		t.Fatalf("second dollars = %d, want 300 in %#v", prizeDollarsByRank(got, "second"), got)
	}
	if prizeDollarsByRank(got, "third") != 200 {
		t.Fatalf("third dollars = %d, want 200 in %#v", prizeDollarsByRank(got, "third"), got)
	}
}

func TestDynamicPrizePoolContributionRequiresConfiguredTierEconomics(t *testing.T) {
	cfg := DefaultConfig()
	cfg.DynamicPrizePool = poolfundcore.Config{
		Enabled:          true,
		ContributionRate: 0.3,
		TierEconomics: []poolfundcore.TierEconomics{
			{Dollars: 55, Revenue: 55, Cost: 20},
		},
	}

	got, ok := DynamicPoolContributionForTier(cfg, 55)
	if !ok || got.Contribution != 10.5 {
		t.Fatalf("DynamicPoolContributionForTier(55) = %#v/%v, want contribution 10.5", got, ok)
	}
	if got, ok := DynamicPoolContributionForTier(cfg, 100); ok || got.Contribution != 0 {
		t.Fatalf("unconfigured tier contribution = %#v/%v, want zero/false", got, ok)
	}
}

func TestPlanLoginCardUsesPlainTokenAndPurchaseFacts(t *testing.T) {
	cfg := DefaultConfig()
	got := PlanLoginCard(LoginCardPlanInput{
		CardKey:          "sk-shop-card",
		Name:             "shop-card",
		Status:           1,
		IntervalQuota:    100 * 500000,
		CreatedTime:      cfg.StartTS - 1,
		ShopPurchaseTime: cfg.StartText,
		Config:           cfg,
		QuotaUnit:        500000,
	})

	if got.Rejection != "" {
		t.Fatalf("PlanLoginCard rejected: %#v", got)
	}
	if got.Plan.CardKey != "sk-shop-card" || got.Plan.CardName != "shop-card" || got.Plan.Dollars != 100 || got.Plan.TotalDraws != 1 || got.Plan.Source != "shop" || got.Plan.PurchaseTime != cfg.StartText {
		t.Fatalf("plan = %#v", got.Plan)
	}
}

func TestPlanLoginCardRejectsOutsideWindowTokenWithoutPurchase(t *testing.T) {
	cfg := DefaultConfig()
	got := PlanLoginCard(LoginCardPlanInput{
		Name:          "outside-card",
		Status:        1,
		IntervalQuota: 100 * 500000,
		CreatedTime:   cfg.StartTS - 1,
		Config:        cfg,
		QuotaUnit:     500000,
	})

	if got.Rejection != LoginCardOutsideWindow {
		t.Fatalf("rejection=%q, want %q", got.Rejection, LoginCardOutsideWindow)
	}
}

func TestPlanLoginCardAllowsConfiguredScratchTierWithoutSpinMap(t *testing.T) {
	cfg := DefaultConfig()
	cfg.SpinMap = map[float64]int{100: 1}
	cfg.GameRoutes = []GameRoute{{Dollars: 55, Game: GameScratch, DrawCount: 1}}

	got := PlanLoginCard(LoginCardPlanInput{
		CardKey:          "sk-scratch-card",
		Name:             "scratch-card",
		Status:           1,
		IntervalQuota:    55 * 500000,
		CreatedTime:      cfg.StartTS - 1,
		ShopPurchaseTime: cfg.StartText,
		Config:           cfg,
		QuotaUnit:        500000,
	})

	if got.Rejection != "" || got.Plan.Dollars != 55 || got.Plan.TotalDraws != 1 {
		t.Fatalf("scratch plan = %#v", got)
	}
}

func TestPlanLoginCardCalculatesDynamicPoolContribution(t *testing.T) {
	cfg := DefaultConfig()
	cfg.DynamicPrizePool = poolfundcore.Config{
		Enabled:          true,
		ContributionRate: 0.3,
		TierEconomics: []poolfundcore.TierEconomics{
			{Dollars: 55, Revenue: 55, Cost: 20},
		},
	}
	cfg.GameRoutes = []GameRoute{{Dollars: 55, Game: GameScratch, DrawCount: 1}}

	got := PlanLoginCard(LoginCardPlanInput{
		Name:             "scratch-card",
		Status:           1,
		IntervalQuota:    55 * 500000,
		CreatedTime:      cfg.StartTS,
		ShopPurchaseTime: cfg.StartText,
		Config:           cfg,
		QuotaUnit:        500000,
	})

	if got.Rejection != "" || got.Plan.PoolContribution.Contribution != 10.5 {
		t.Fatalf("dynamic contribution plan = %#v", got)
	}
}

func TestConfigJSONDropsLegacyPostJackpotPool(t *testing.T) {
	var cfg Config
	if err := json.Unmarshal([]byte(`{
		"prizePool": [{"type":"win","dollars":7,"weight":1}],
		"postJackpotPrizes": [{"type":"win","dollars":99,"weight":1}],
		"postJackpotRules": [{"minDollars":100,"maxWonRatio":0.5}]
	}`), &cfg); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "postJackpot") || strings.Contains(string(data), `"dollars":99`) {
		t.Fatalf("legacy post jackpot config should not survive normalized JSON: %s", data)
	}
}

func TestNormalizeConfigPrefersActivityWindowTextOverStaleTimestamps(t *testing.T) {
	cfg := NormalizeConfig(Config{
		StartText: "2026-06-18 00:00:00",
		EndText:   "2026-06-30 23:59:59",
		StartTS:   StartTS,
		EndTS:     EndTS,
	})

	if cfg.StartTS != 1781712000 || cfg.EndTS != 1782835199 {
		t.Fatalf("window timestamps not realigned from text: start=%d end=%d", cfg.StartTS, cfg.EndTS)
	}
	if cfg.StartText != "2026-06-18 00:00:00" || cfg.EndText != "2026-06-30 23:59:59" {
		t.Fatalf("window text changed unexpectedly: %#v", cfg)
	}
}

func TestNormalizeConfigBackfillsActivityWindowTextFromTimestamps(t *testing.T) {
	cfg := NormalizeConfig(Config{
		StartTS: 1781712000,
		EndTS:   1782835199,
	})

	if cfg.StartText != "2026-06-18 00:00:00" || cfg.EndText != "2026-06-30 23:59:59" {
		t.Fatalf("window text not backfilled from timestamps: %#v", cfg)
	}
}

func TestNormalizeConfigAddsAdvertisedPrizeMetadataForLegacyPools(t *testing.T) {
	cfg := NormalizeConfig(Config{
		PrizePool: []Prize{
			{Type: "miss", Weight: 1},
			{Type: "win", Dollars: 100, Weight: 1},
			{Type: "win", Dollars: 200, Weight: 1},
			{Type: "win", Dollars: 1000, Weight: 1},
		},
	})

	for _, want := range []struct {
		dollars int
		rank    string
		label   string
	}{
		{100, "third", "三等奖"},
		{200, "second", "二等奖"},
		{1000, "jackpot", "大奖"},
	} {
		prize := findActivityPrize(t, cfg.PrizePool, want.dollars)
		if prize.Rank != want.rank || prize.Label != want.label || !prize.Advertised {
			t.Fatalf("$%d metadata = %#v, want %s/%s advertised", want.dollars, prize, want.rank, want.label)
		}
	}
}

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

	gameConfigs := DefaultGameConfigs()
	gameConfigs[0].TargetExpectedValue = 999
	if DefaultGameConfigs()[0].TargetExpectedValue == 999 {
		t.Fatalf("DefaultGameConfigs should return an independent slice copy")
	}

	scratchRewards := DefaultScratchRewards()
	scratchRewards[0] = 999
	if DefaultScratchRewards()[0] == 999 {
		t.Fatalf("DefaultScratchRewards should return an independent slice copy")
	}
}

func TestNormalizeConfigClampsScratchMaxReveals(t *testing.T) {
	def := NormalizeConfig(Config{})
	if def.ScratchMaxReveals != ScratchMaxReveals {
		t.Fatalf("default scratch max reveals = %d, want %d", def.ScratchMaxReveals, ScratchMaxReveals)
	}

	custom := NormalizeConfig(Config{ScratchMaxReveals: 4})
	if custom.ScratchMaxReveals != 4 {
		t.Fatalf("custom scratch max reveals = %d, want 4", custom.ScratchMaxReveals)
	}

	clamped := NormalizeConfig(Config{ScratchMaxReveals: 99})
	if clamped.ScratchMaxReveals != ScratchCells-ScratchMines {
		t.Fatalf("clamped scratch max reveals = %d, want %d", clamped.ScratchMaxReveals, ScratchCells-ScratchMines)
	}
}

func TestSpinWithConfigUsesRuntimePrizeWeights(t *testing.T) {
	cfg := DefaultConfig()
	cfg.GameConfigs = []GameConfig{{Game: GameSlot, TargetExpectedValue: 4.5, ActualExpectedValue: 4.5}}
	cfg.PrizePool = []Prize{
		{Type: "miss", Weight: 100},
		{Type: "win", Dollars: 9, Weight: 1},
	}

	got := SpinWithConfig(cfg, 42, false, 0, 3, 0, 0, func(max int) int {
		if max != 6 {
			t.Fatalf("roll total = %d, want 6", max)
		}
		return 5
	})

	if got.Type != "win" || got.Dollars != 9 {
		t.Fatalf("SpinWithConfig = %#v, want $9 win", got)
	}
}

func TestSpinWithConfigUsesUnifiedPrizePoolEvenWhenLegacyTierPoolExists(t *testing.T) {
	var cfg Config
	if err := json.Unmarshal([]byte(`{
		"prizePool": [
			{"type":"miss","weight":1},
			{"type":"win","dollars":7,"weight":1}
		],
		"tierPools": {
			"100": [{"type":"win","dollars":99,"weight":1}]
		}
	}`), &cfg); err != nil {
		t.Fatal(err)
	}

	got := SpinWithConfig(cfg, 100, false, 0, 1, 0, 0, func(max int) int {
		if max != 2 {
			t.Fatalf("roll total = %d, want unified pool total 2", max)
		}
		return 1
	})

	if got.Type != "win" || got.Dollars != 7 {
		t.Fatalf("SpinWithConfig should ignore legacy tierPools and use unified prizePool, got %#v", got)
	}
}

func TestSpinCoreConfigForPoolBalanceUsesDynamicAdvertisedAwards(t *testing.T) {
	cfg := DefaultConfig()
	cfg.GameConfigs = []GameConfig{{Game: GameSlot, TargetExpectedValue: 4.5, ActualExpectedValue: 4.5}}
	cfg.DynamicPrizePool = poolfundcore.Config{Enabled: true, JackpotRate: 0.5}
	cfg.PrizePool = []Prize{
		{Type: "miss", Weight: 1},
		{Type: "win", Dollars: 1000, Weight: 1, Rank: "jackpot", Label: "大奖", Advertised: true},
	}

	got := SpinCoreConfigForPoolBalance(cfg, GameSlot, 1, 1000)
	if got.JackpotPrizeDollars != 500 || prizeDollarsByRank(got.PrizePool, "jackpot") != 500 {
		t.Fatalf("SpinCoreConfigForPoolBalance()=%#v, want dynamic $500 jackpot", got)
	}
}

func TestActualExpectedValueUsesBalancedTargetExpectedValue(t *testing.T) {
	cfg := DefaultConfig()
	cfg.GameConfigs = []GameConfig{{Game: GameSlot, TargetExpectedValue: 4.5, ActualExpectedValue: 4.5}}
	cfg.PrizePool = []Prize{
		{Type: "miss", Weight: 100},
		{Type: "retry", Weight: 100},
		{Type: "win", Dollars: 9, Weight: 1},
	}

	if got := ActualExpectedValue(cfg); got != 4.5 {
		t.Fatalf("ActualExpectedValue = %v, want balanced target 4.5", got)
	}
	if got := ExpectedValue(cfg.PrizePool); got == 4.5 {
		t.Fatalf("raw pool should not already match target; test must prove balancing")
	}
}

func TestConfigJSONDropsLegacyTierPools(t *testing.T) {
	var cfg Config
	if err := json.Unmarshal([]byte(`{
		"prizePool": [{"type":"win","dollars":7,"weight":1}],
		"tierPools": {"100": [{"type":"win","dollars":99,"weight":1}]}
	}`), &cfg); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "tierPools") || strings.Contains(string(data), `"dollars":99`) {
		t.Fatalf("legacy tierPools should not survive normalized JSON: %s", data)
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

func findActivityPrize(t *testing.T, pool []Prize, dollars int) Prize {
	t.Helper()
	for _, prize := range pool {
		if prize.Dollars == dollars {
			return prize
		}
	}
	t.Fatalf("missing prize $%d in %#v", dollars, pool)
	return Prize{}
}

func prizeDollarsByRank(pool []Prize, rank string) int {
	for _, prize := range pool {
		if prize.Rank == rank {
			return prize.Dollars
		}
	}
	return 0
}
