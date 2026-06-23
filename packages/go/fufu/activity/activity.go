package activity

import (
	"encoding/json"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"fufu/poolfundcore"
	"fufu/prizepoolcore"
	"fufu/spincore"
)

const (
	StartText               = "2026-05-01 00:00:00"
	EndText                 = "2026-05-08 23:59:59"
	StartTS           int64 = 1777564800
	EndTS             int64 = 1778255999
	ScratchCells            = 9
	ScratchMines            = 2
	ScratchMaxReveals       = 6
)

const (
	GameSlot    = "slot"
	GameScratch = "scratch"
	GameDragon  = "dragonboat"
)

const activityTimeLayout = "2006-01-02 15:04:05"

var activityTimeLocation = loadActivityTimeLocation()

type Prize = prizepoolcore.Prize
type SpinGuaranteeRule = spincore.GuaranteeRule

type SpinResult = spincore.Result

type GameRoute struct {
	Dollars   int    `json:"dollars"`
	Game      string `json:"game"`
	DrawCount int    `json:"drawCount,omitempty"`
}

type GameConfig struct {
	Game                string  `json:"game"`
	TargetExpectedValue float64 `json:"targetExpectedValue,omitempty"`
	ActualExpectedValue float64 `json:"actualExpectedValue,omitempty"`
}

type SubscriptionPlanMapping struct {
	PlanID  int64   `json:"planId,omitempty"`
	Title   string  `json:"title,omitempty"`
	Match   string  `json:"match,omitempty"`
	Dollars float64 `json:"dollars"`
}

type Config struct {
	StartText              string              `json:"startText"`
	EndText                string              `json:"endText"`
	StartTS                int64               `json:"startTS"`
	EndTS                  int64               `json:"endTS"`
	TargetExpectedValue    float64             `json:"targetExpectedValue,omitempty"`
	ActualExpectedValue    float64             `json:"actualExpectedValue,omitempty"`
	SpinMap                map[float64]int     `json:"spinMap"`
	GameConfigs            []GameConfig        `json:"gameConfigs"`
	PrizePool              []Prize             `json:"prizePool"`
	SpinGuarantees         []SpinGuaranteeRule `json:"spinGuarantees"`
	JackpotPrizeDollars    int                 `json:"jackpotPrizeDollars"`
	JackpotEligibleDollars []float64           `json:"jackpotEligibleDollars"`
	DynamicPrizePool       poolfundcore.Config `json:"dynamicPrizePool,omitempty"`
	ScratchRewards         []int               `json:"scratchRewards"`
	ScratchMaxReveals      int                 `json:"scratchMaxReveals,omitempty"`
	// GameRoutes is the source of truth for card-tier gameplay routing.
	// ScratchTiers is kept as a compatibility projection for older admin payloads.
	GameRoutes               []GameRoute               `json:"gameRoutes"`
	ScratchTiers             []int                     `json:"scratchTiers"`
	SubscriptionPlanMappings []SubscriptionPlanMapping `json:"subscriptionPlanMappings,omitempty"`
}

type LoginCardRejection string

const (
	LoginCardDisabled      LoginCardRejection = "disabled"
	LoginCardOutsideWindow LoginCardRejection = "outside_window"
	LoginCardIneligible    LoginCardRejection = "ineligible"
)

type LoginCardPlanInput struct {
	CardKey          string
	Name             string
	Status           int
	IntervalQuota    int64
	CreatedTime      int64
	ShopPurchaseTime string
	Config           Config
	QuotaUnit        int64
}

type LoginCardPlan struct {
	CardKey          string
	CardName         string
	Dollars          float64
	TotalDraws       int
	Game             string
	Source           string
	PurchaseTime     string
	PoolContribution poolfundcore.ContributionResult
}

type LoginCardPlanResult struct {
	Plan      LoginCardPlan
	Rejection LoginCardRejection
}

type gameConfigJSON struct {
	Game                string  `json:"game"`
	TargetExpectedValue float64 `json:"targetExpectedValue,omitempty"`
	ActualExpectedValue float64 `json:"actualExpectedValue,omitempty"`
	DrawCount           int     `json:"drawCount,omitempty"`
}

type configJSON struct {
	StartText                string                    `json:"startText"`
	EndText                  string                    `json:"endText"`
	StartTS                  int64                     `json:"startTS"`
	EndTS                    int64                     `json:"endTS"`
	TargetExpectedValue      float64                   `json:"targetExpectedValue,omitempty"`
	ActualExpectedValue      float64                   `json:"actualExpectedValue,omitempty"`
	SpinMap                  map[string]int            `json:"spinMap"`
	GameConfigs              []gameConfigJSON          `json:"gameConfigs"`
	PrizePool                []Prize                   `json:"prizePool"`
	SpinGuarantees           []SpinGuaranteeRule       `json:"spinGuarantees"`
	JackpotPrizeDollars      int                       `json:"jackpotPrizeDollars"`
	JackpotEligibleDollars   []float64                 `json:"jackpotEligibleDollars"`
	DynamicPrizePool         poolfundcore.Config       `json:"dynamicPrizePool,omitempty"`
	ScratchRewards           []int                     `json:"scratchRewards"`
	ScratchMaxReveals        int                       `json:"scratchMaxReveals,omitempty"`
	GameRoutes               []GameRoute               `json:"gameRoutes"`
	ScratchTiers             []int                     `json:"scratchTiers"`
	SubscriptionPlanMappings []SubscriptionPlanMapping `json:"subscriptionPlanMappings,omitempty"`
}

var defaultSpinMap = map[float64]int{0.1: 100, 100: 1, 150: 1, 300: 3, 500: 4, 1000: 10}

var defaultPrizePool = []Prize{
	{Type: "miss", Weight: 500},
	{Type: "retry", Weight: 500},
	{Type: "win", Dollars: 1, Weight: 1500},
	{Type: "win", Dollars: 5, Weight: 3000},
	{Type: "win", Dollars: 10, Weight: 2000},
	{Type: "win", Dollars: 20, Weight: 1200},
	{Type: "win", Dollars: 50, Weight: 580},
	{Type: "win", Dollars: 100, Weight: 380, Rank: "third", Label: "三等奖", Advertised: true},
	{Type: "win", Dollars: 200, Weight: 200, Rank: "second", Label: "二等奖", Advertised: true},
	{Type: "win", Dollars: 500, Weight: 100},
	{Type: "win", Dollars: 1000, Weight: 40, Rank: "jackpot", Label: "大奖", Advertised: true},
}

var defaultSpinGuarantees = []SpinGuaranteeRule{
	{DollarTier: 1000, RemainingSpins: 1, MaxWonBelow: 50, PrizeDollars: 100},
	{DollarTier: 500, RemainingSpins: 1, MaxWonBelow: 50, PrizeDollars: 20},
}
var defaultJackpotEligibleDollars = []float64{0.1, 1000}

const defaultJackpotPrizeDollars = 1000

var defaultScratchRewards = []int{2, 4, 6, 8, 12, 15}
var defaultGameRoutes = []GameRoute{{Dollars: 55, Game: GameScratch, DrawCount: 1}}
var defaultDynamicPrizePool = poolfundcore.Config{
	Enabled:          false,
	ContributionRate: 0.3,
	JackpotRate:      0.5,
	SecondRate:       0.3,
	ThirdRate:        0.2,
}

var defaultGameConfigs = []GameConfig{
	{Game: GameSlot, TargetExpectedValue: ExpectedValue(defaultPrizePool), ActualExpectedValue: ExpectedValue(defaultPrizePool)},
	{Game: GameScratch, TargetExpectedValue: ExpectedValue(defaultPrizePool), ActualExpectedValue: ExpectedValue(defaultPrizePool)},
	{Game: GameDragon, TargetExpectedValue: ExpectedValue(defaultPrizePool), ActualExpectedValue: ExpectedValue(defaultPrizePool)},
}

func DefaultSpinMap() map[float64]int {
	out := make(map[float64]int, len(defaultSpinMap))
	for dollars, spins := range defaultSpinMap {
		out[dollars] = spins
	}
	return out
}

func DefaultPrizePool() []Prize {
	return clonePrizePool(defaultPrizePool)
}

func DefaultSpinGuarantees() []SpinGuaranteeRule {
	return cloneSpinGuarantees(defaultSpinGuarantees)
}

func DefaultJackpotEligibleDollars() []float64 {
	return append([]float64(nil), defaultJackpotEligibleDollars...)
}

func DefaultScratchRewards() []int {
	return append([]int(nil), defaultScratchRewards...)
}

func DefaultGameRoutes() []GameRoute {
	return append([]GameRoute(nil), defaultGameRoutes...)
}

func DefaultGameConfigs() []GameConfig {
	return append([]GameConfig(nil), defaultGameConfigs...)
}

func DefaultScratchTiers() []int {
	return scratchTiersFromGameRoutes(defaultGameRoutes)
}

// GameForTier returns the configured gameplay for a card dollar tier.
func (cfg Config) GameForTier(dollars float64) string {
	cfg = NormalizeConfig(cfg)
	for _, route := range cfg.GameRoutes {
		if float64(route.Dollars) == dollars {
			return route.Game
		}
	}
	return GameSlot
}

func (cfg Config) GameConfigFor(game string) GameConfig {
	cfg = NormalizeConfig(cfg)
	game = normalizeGameMode(game)
	if game == "" {
		game = GameSlot
	}
	for _, item := range cfg.GameConfigs {
		if item.Game == game {
			return item
		}
	}
	return GameConfig{Game: game}
}

func (cfg Config) DrawCountForTier(dollars float64) int {
	cfg = NormalizeConfig(cfg)
	if !cfg.TierConfigured(dollars) {
		return 0
	}
	for _, route := range cfg.GameRoutes {
		if float64(route.Dollars) != dollars {
			continue
		}
		if route.DrawCount > 0 {
			return route.DrawCount
		}
		if route.Game == GameSlot && cfg.SpinMap[dollars] > 0 {
			return cfg.SpinMap[dollars]
		}
		return 1
	}
	if cfg.SpinMap[dollars] > 0 {
		return cfg.SpinMap[dollars]
	}
	return 0
}

func (cfg Config) TierConfigured(dollars float64) bool {
	cfg = NormalizeConfig(cfg)
	if cfg.SpinMap[dollars] > 0 {
		return true
	}
	for _, route := range cfg.GameRoutes {
		if float64(route.Dollars) == dollars {
			return true
		}
	}
	return false
}

// IsScratchTier reports whether a card of the given dollar tier plays the scratch game.
func (cfg Config) IsScratchTier(dollars float64) bool {
	return cfg.GameForTier(dollars) == GameScratch
}

func normalizeScratchTiers(tiers []int) []int {
	seen := map[int]bool{}
	out := []int{}
	for _, tier := range tiers {
		if tier <= 0 || seen[tier] {
			continue
		}
		seen[tier] = true
		out = append(out, tier)
	}
	return out
}

func normalizeGameMode(game string) string {
	switch strings.ToLower(strings.TrimSpace(game)) {
	case GameScratch, "scratch-card", "scratch_card", "刮刮乐":
		return GameScratch
	case GameSlot, "spin", "slot-machine", "slot_machine", "老虎机":
		return GameSlot
	case GameDragon, "dragon", "duanwu", "端午", "黄金矿工":
		return GameDragon
	default:
		return ""
	}
}

func normalizeGameRoutes(routes []GameRoute) []GameRoute {
	byTier := map[int]GameRoute{}
	tiers := []int{}
	for _, route := range routes {
		if route.Dollars <= 0 {
			continue
		}
		game := normalizeGameMode(route.Game)
		if game == "" {
			continue
		}
		if _, ok := byTier[route.Dollars]; !ok {
			tiers = append(tiers, route.Dollars)
		}
		if route.DrawCount < 0 {
			route.DrawCount = 0
		}
		byTier[route.Dollars] = GameRoute{Dollars: route.Dollars, Game: game, DrawCount: route.DrawCount}
	}
	sort.Ints(tiers)
	out := make([]GameRoute, 0, len(tiers))
	for _, tier := range tiers {
		out = append(out, byTier[tier])
	}
	return out
}

func normalizeSubscriptionPlanMappings(mappings []SubscriptionPlanMapping) []SubscriptionPlanMapping {
	out := make([]SubscriptionPlanMapping, 0, len(mappings))
	for _, mapping := range mappings {
		mapping.Title = strings.TrimSpace(mapping.Title)
		mapping.Match = strings.TrimSpace(strings.ToLower(mapping.Match))
		if mapping.Match == "" && mapping.Title != "" {
			mapping.Match = "contains"
		}
		switch mapping.Match {
		case "", "exact", "contains":
		default:
			mapping.Match = "contains"
		}
		if mapping.Dollars <= 0 {
			continue
		}
		if mapping.PlanID <= 0 && mapping.Title == "" {
			continue
		}
		out = append(out, mapping)
	}
	return out
}

func gameRoutesFromScratchTiers(tiers []int) []GameRoute {
	tiers = normalizeScratchTiers(tiers)
	out := make([]GameRoute, 0, len(tiers))
	for _, tier := range tiers {
		out = append(out, GameRoute{Dollars: tier, Game: GameScratch})
	}
	return out
}

func scratchTiersFromGameRoutes(routes []GameRoute) []int {
	tiers := []int{}
	for _, route := range normalizeGameRoutes(routes) {
		if route.Game == GameScratch {
			tiers = append(tiers, route.Dollars)
		}
	}
	return tiers
}

func DefaultConfig() Config {
	return Config{
		StartText:              StartText,
		EndText:                EndText,
		StartTS:                StartTS,
		EndTS:                  EndTS,
		TargetExpectedValue:    ExpectedValue(defaultPrizePool),
		ActualExpectedValue:    ExpectedValue(defaultPrizePool),
		SpinMap:                DefaultSpinMap(),
		GameConfigs:            DefaultGameConfigs(),
		PrizePool:              DefaultPrizePool(),
		SpinGuarantees:         DefaultSpinGuarantees(),
		JackpotPrizeDollars:    defaultJackpotPrizeDollars,
		JackpotEligibleDollars: DefaultJackpotEligibleDollars(),
		DynamicPrizePool:       poolfundcore.NormalizeConfig(defaultDynamicPrizePool),
		ScratchRewards:         DefaultScratchRewards(),
		ScratchMaxReveals:      ScratchMaxReveals,
		GameRoutes:             DefaultGameRoutes(),
		ScratchTiers:           DefaultScratchTiers(),
	}
}

func CloneConfig(cfg Config) Config {
	cfg = NormalizeConfig(cfg)
	out := cfg
	out.SpinMap = cloneSpinMap(cfg.SpinMap)
	out.GameConfigs = append([]GameConfig(nil), cfg.GameConfigs...)
	out.PrizePool = clonePrizePool(cfg.PrizePool)
	out.SpinGuarantees = cloneSpinGuarantees(cfg.SpinGuarantees)
	out.JackpotEligibleDollars = append([]float64(nil), cfg.JackpotEligibleDollars...)
	out.DynamicPrizePool = cloneDynamicPrizePoolConfig(cfg.DynamicPrizePool)
	out.ScratchRewards = append([]int(nil), cfg.ScratchRewards...)
	out.ScratchMaxReveals = cfg.ScratchMaxReveals
	out.GameRoutes = append([]GameRoute(nil), cfg.GameRoutes...)
	out.ScratchTiers = append(make([]int, 0, len(cfg.ScratchTiers)), cfg.ScratchTiers...)
	out.SubscriptionPlanMappings = append([]SubscriptionPlanMapping(nil), cfg.SubscriptionPlanMappings...)
	return out
}

func loadActivityTimeLocation() *time.Location {
	if loc, err := time.LoadLocation("Asia/Shanghai"); err == nil {
		return loc
	}
	return time.FixedZone("UTC+8", 8*60*60)
}

func parseActivityTimeText(raw string) (int64, string, bool) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return 0, "", false
	}
	parsed, err := time.ParseInLocation(activityTimeLayout, text, activityTimeLocation)
	if err != nil {
		return 0, "", false
	}
	return parsed.Unix(), parsed.In(activityTimeLocation).Format(activityTimeLayout), true
}

func formatActivityTimeText(ts int64) string {
	if ts <= 0 {
		return ""
	}
	return time.Unix(ts, 0).In(activityTimeLocation).Format(activityTimeLayout)
}

func normalizeActivityWindow(text string, ts, defaultTS int64, defaultText string) (string, int64) {
	if parsedTS, parsedText, ok := parseActivityTimeText(text); ok {
		return parsedText, parsedTS
	}
	if ts > 0 {
		return formatActivityTimeText(ts), ts
	}
	return defaultText, defaultTS
}

func NormalizeConfig(cfg Config) Config {
	hasGameConfigs := cfg.GameConfigs != nil
	defaults := Config{
		StartText:              StartText,
		EndText:                EndText,
		StartTS:                StartTS,
		EndTS:                  EndTS,
		TargetExpectedValue:    ExpectedValue(defaultPrizePool),
		ActualExpectedValue:    ExpectedValue(defaultPrizePool),
		SpinMap:                DefaultSpinMap(),
		GameConfigs:            DefaultGameConfigs(),
		PrizePool:              DefaultPrizePool(),
		SpinGuarantees:         DefaultSpinGuarantees(),
		JackpotPrizeDollars:    defaultJackpotPrizeDollars,
		JackpotEligibleDollars: DefaultJackpotEligibleDollars(),
		DynamicPrizePool:       poolfundcore.NormalizeConfig(defaultDynamicPrizePool),
		ScratchRewards:         DefaultScratchRewards(),
		ScratchMaxReveals:      ScratchMaxReveals,
		GameRoutes:             DefaultGameRoutes(),
		ScratchTiers:           DefaultScratchTiers(),
	}
	cfg.StartText, cfg.StartTS = normalizeActivityWindow(cfg.StartText, cfg.StartTS, defaults.StartTS, defaults.StartText)
	cfg.EndText, cfg.EndTS = normalizeActivityWindow(cfg.EndText, cfg.EndTS, defaults.EndTS, defaults.EndText)
	if cfg.TargetExpectedValue < 0 {
		cfg.TargetExpectedValue = 0
	}
	if cfg.ActualExpectedValue < 0 {
		cfg.ActualExpectedValue = 0
	}
	if cfg.SpinMap == nil {
		cfg.SpinMap = defaults.SpinMap
	} else {
		cfg.SpinMap = normalizeSpinMap(cfg.SpinMap)
	}
	if cfg.PrizePool == nil {
		cfg.PrizePool = defaults.PrizePool
	} else {
		cfg.PrizePool = normalizePrizePool(cfg.PrizePool)
	}
	gameDefaults := defaults.GameConfigs
	if !hasGameConfigs {
		ev := ExpectedValue(cfg.PrizePool)
		gameDefaults = []GameConfig{
			{Game: GameSlot, TargetExpectedValue: ev, ActualExpectedValue: ev},
			{Game: GameScratch, TargetExpectedValue: ev, ActualExpectedValue: ev},
		}
	}
	cfg.GameConfigs = normalizeGameConfigs(cfg.GameConfigs, gameDefaults)
	if cfg.TargetExpectedValue == 0 {
		cfg.TargetExpectedValue = gameConfigFromList(cfg.GameConfigs, GameSlot).TargetExpectedValue
	}
	if cfg.ActualExpectedValue == 0 {
		cfg.ActualExpectedValue = gameConfigFromList(cfg.GameConfigs, GameSlot).ActualExpectedValue
	}
	if cfg.SpinGuarantees == nil {
		cfg.SpinGuarantees = defaults.SpinGuarantees
	} else {
		cfg.SpinGuarantees = normalizeSpinGuarantees(cfg.SpinGuarantees)
	}
	if cfg.JackpotPrizeDollars <= 0 {
		cfg.JackpotPrizeDollars = defaults.JackpotPrizeDollars
	}
	if cfg.JackpotEligibleDollars == nil {
		cfg.JackpotEligibleDollars = defaults.JackpotEligibleDollars
	} else {
		cfg.JackpotEligibleDollars = normalizePositiveFloatList(cfg.JackpotEligibleDollars)
	}
	if isZeroDynamicPrizePoolConfig(cfg.DynamicPrizePool) {
		cfg.DynamicPrizePool = defaults.DynamicPrizePool
	} else {
		cfg.DynamicPrizePool = poolfundcore.NormalizeConfig(cfg.DynamicPrizePool)
	}
	if cfg.ScratchRewards == nil {
		cfg.ScratchRewards = defaults.ScratchRewards
	} else {
		cfg.ScratchRewards = normalizeScratchRewards(cfg.ScratchRewards)
	}
	cfg.ScratchMaxReveals = normalizeScratchMaxReveals(cfg.ScratchMaxReveals)
	if cfg.GameRoutes == nil {
		if cfg.ScratchTiers == nil {
			cfg.GameRoutes = defaults.GameRoutes
		} else {
			cfg.GameRoutes = gameRoutesFromScratchTiers(cfg.ScratchTiers)
		}
	} else {
		cfg.GameRoutes = normalizeGameRoutes(cfg.GameRoutes)
	}
	cfg.ScratchTiers = scratchTiersFromGameRoutes(cfg.GameRoutes)
	cfg.SubscriptionPlanMappings = normalizeSubscriptionPlanMappings(cfg.SubscriptionPlanMappings)
	return cfg
}

func (cfg Config) MarshalJSON() ([]byte, error) {
	cfg = NormalizeConfig(cfg)
	return json.Marshal(configJSON{
		StartText:                cfg.StartText,
		EndText:                  cfg.EndText,
		StartTS:                  cfg.StartTS,
		EndTS:                    cfg.EndTS,
		TargetExpectedValue:      cfg.TargetExpectedValue,
		ActualExpectedValue:      cfg.ActualExpectedValue,
		SpinMap:                  spinMapToJSON(cfg.SpinMap),
		GameConfigs:              gameConfigsToJSON(cfg.GameConfigs),
		PrizePool:                cfg.PrizePool,
		SpinGuarantees:           cfg.SpinGuarantees,
		JackpotPrizeDollars:      cfg.JackpotPrizeDollars,
		JackpotEligibleDollars:   cfg.JackpotEligibleDollars,
		DynamicPrizePool:         cfg.DynamicPrizePool,
		ScratchRewards:           cfg.ScratchRewards,
		ScratchMaxReveals:        cfg.ScratchMaxReveals,
		GameRoutes:               cfg.GameRoutes,
		ScratchTiers:             cfg.ScratchTiers,
		SubscriptionPlanMappings: cfg.SubscriptionPlanMappings,
	})
}

func (cfg *Config) UnmarshalJSON(data []byte) error {
	var raw configJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	gameConfigs, legacyDrawCounts := gameConfigsFromJSON(raw.GameConfigs)
	normalized := NormalizeConfig(Config{
		StartText:                raw.StartText,
		EndText:                  raw.EndText,
		StartTS:                  raw.StartTS,
		EndTS:                    raw.EndTS,
		TargetExpectedValue:      raw.TargetExpectedValue,
		ActualExpectedValue:      raw.ActualExpectedValue,
		SpinMap:                  spinMapFromJSON(raw.SpinMap),
		GameConfigs:              gameConfigs,
		PrizePool:                raw.PrizePool,
		SpinGuarantees:           raw.SpinGuarantees,
		JackpotPrizeDollars:      raw.JackpotPrizeDollars,
		JackpotEligibleDollars:   raw.JackpotEligibleDollars,
		DynamicPrizePool:         raw.DynamicPrizePool,
		ScratchRewards:           raw.ScratchRewards,
		ScratchMaxReveals:        raw.ScratchMaxReveals,
		GameRoutes:               raw.GameRoutes,
		ScratchTiers:             raw.ScratchTiers,
		SubscriptionPlanMappings: raw.SubscriptionPlanMappings,
	})
	normalized = applyLegacyGameDrawCounts(normalized, legacyDrawCounts)
	*cfg = NormalizeConfig(normalized)
	return nil
}

func ActualExpectedValue(cfg Config) float64 {
	cfg = NormalizeConfig(cfg)
	return BalancedPrizePoolForGame(cfg, GameSlot).ActualExpectedValue
}

func ExpectedValue(pool []Prize) float64 {
	return prizepoolcore.ExpectedValue(pool)
}

func BalancedPrizePool(cfg Config) []Prize {
	return BalancedPrizePoolForGame(cfg, GameSlot).Pool
}

func BalancedPrizePoolForGame(cfg Config, game string) prizepoolcore.BalanceResult {
	return balancedPrizePoolForGameDraws(cfg, game, 1)
}

func BalancedPrizePoolForTier(cfg Config, dollars float64) prizepoolcore.BalanceResult {
	cfg = NormalizeConfig(cfg)
	return balancedPrizePoolForGameDraws(cfg, cfg.GameForTier(dollars), cfg.DrawCountForTier(dollars))
}

func balancedPrizePoolForGameDraws(cfg Config, game string, drawCount int) prizepoolcore.BalanceResult {
	cfg = NormalizeConfig(cfg)
	gameConfig := cfg.GameConfigFor(game)
	return prizepoolcore.BalancePoolForPlan(prizepoolcore.BalanceInput{
		Pool:                cfg.PrizePool,
		TargetExpectedValue: gameConfig.TargetExpectedValue,
		ActualExpectedValue: gameConfig.ActualExpectedValue,
		DrawCount:           drawCount,
	})
}

func PrizePoolWithDynamicAwards(cfg Config, poolBalance float64) []Prize {
	cfg = NormalizeConfig(cfg)
	if !cfg.DynamicPrizePool.Enabled {
		return clonePrizePool(cfg.PrizePool)
	}
	amounts := poolfundcore.AllocatePrizeAmounts(poolBalance, cfg.DynamicPrizePool)
	pool := clonePrizePool(cfg.PrizePool)
	for i := range pool {
		switch pool[i].Rank {
		case "jackpot":
			pool[i].Dollars = amounts.JackpotDollars
			if amounts.JackpotDollars <= 0 {
				pool[i].Weight = 0
			}
		case "second":
			pool[i].Dollars = amounts.SecondDollars
			if amounts.SecondDollars <= 0 {
				pool[i].Weight = 0
			}
		case "third":
			pool[i].Dollars = amounts.ThirdDollars
			if amounts.ThirdDollars <= 0 {
				pool[i].Weight = 0
			}
		}
	}
	return normalizePrizePool(pool)
}

func DynamicPoolContributionForTier(cfg Config, dollars float64) (poolfundcore.ContributionResult, bool) {
	cfg = NormalizeConfig(cfg)
	if !cfg.DynamicPrizePool.Enabled {
		return poolfundcore.ContributionResult{}, false
	}
	economics, ok := poolfundcore.TierEconomicsForDollars(cfg.DynamicPrizePool, dollars)
	if !ok {
		return poolfundcore.ContributionResult{}, false
	}
	return poolfundcore.Contribution(economics, cfg.DynamicPrizePool.ContributionRate), true
}

func clonePrizePool(pool []Prize) []Prize {
	return append([]Prize(nil), pool...)
}

func cloneSpinMap(in map[float64]int) map[float64]int {
	out := make(map[float64]int, len(in))
	for dollars, spins := range in {
		out[dollars] = spins
	}
	return out
}

func cloneSpinGuarantees(in []SpinGuaranteeRule) []SpinGuaranteeRule {
	return append([]SpinGuaranteeRule(nil), in...)
}

func cloneDynamicPrizePoolConfig(in poolfundcore.Config) poolfundcore.Config {
	out := in
	out.TierEconomics = append([]poolfundcore.TierEconomics(nil), in.TierEconomics...)
	return out
}

func isZeroDynamicPrizePoolConfig(cfg poolfundcore.Config) bool {
	return !cfg.Enabled &&
		cfg.ContributionRate == 0 &&
		cfg.JackpotRate == 0 &&
		cfg.SecondRate == 0 &&
		cfg.ThirdRate == 0 &&
		len(cfg.TierEconomics) == 0
}

func normalizeSpinMap(in map[float64]int) map[float64]int {
	out := map[float64]int{}
	for dollars, spins := range in {
		if dollars > 0 && spins > 0 {
			out[dollars] = spins
		}
	}
	return out
}

func normalizePrizePool(in []Prize) []Prize {
	out := []Prize{}
	for _, p := range in {
		p.Type = strings.TrimSpace(p.Type)
		if p.Type == "" {
			if p.Dollars > 0 {
				p.Type = "win"
			} else {
				p.Type = "miss"
			}
		}
		if p.Weight <= 0 {
			continue
		}
		if p.Type != "win" {
			p.Dollars = 0
		}
		p = withDefaultAdvertisedPrizeMetadata(p)
		out = append(out, p)
	}
	return out
}

func withDefaultAdvertisedPrizeMetadata(p Prize) Prize {
	if p.Type != "win" || p.Dollars <= 0 {
		return p
	}
	rank, label := "", ""
	switch p.Dollars {
	case defaultJackpotPrizeDollars:
		rank, label = "jackpot", "大奖"
	case 200:
		rank, label = "second", "二等奖"
	case 100:
		rank, label = "third", "三等奖"
	}
	if rank == "" {
		return p
	}
	if p.Rank == "" {
		p.Rank = rank
	}
	if p.Label == "" {
		p.Label = label
	}
	p.Advertised = true
	return p
}

func normalizeGameConfigs(in []GameConfig, defaults []GameConfig) []GameConfig {
	byGame := map[string]GameConfig{}
	order := []string{}
	for _, item := range defaults {
		game := normalizeGameMode(item.Game)
		if game == "" {
			continue
		}
		item.Game = game
		if item.TargetExpectedValue < 0 {
			item.TargetExpectedValue = 0
		}
		if item.ActualExpectedValue < 0 {
			item.ActualExpectedValue = 0
		}
		byGame[game] = item
		order = append(order, game)
	}
	for _, item := range in {
		game := normalizeGameMode(item.Game)
		if game == "" {
			continue
		}
		item.Game = game
		if item.TargetExpectedValue < 0 {
			item.TargetExpectedValue = 0
		}
		if item.ActualExpectedValue < 0 {
			item.ActualExpectedValue = 0
		}
		if _, ok := byGame[game]; !ok {
			order = append(order, game)
		}
		byGame[game] = item
	}
	out := make([]GameConfig, 0, len(order))
	seen := map[string]bool{}
	for _, game := range order {
		if seen[game] {
			continue
		}
		seen[game] = true
		item := byGame[game]
		if item.TargetExpectedValue == 0 {
			item.TargetExpectedValue = ExpectedValue(defaultPrizePool)
		}
		out = append(out, item)
	}
	return out
}

func gameConfigFromList(configs []GameConfig, game string) GameConfig {
	game = normalizeGameMode(game)
	for _, item := range configs {
		if item.Game == game {
			return item
		}
	}
	return GameConfig{Game: game}
}

func gameConfigsToJSON(configs []GameConfig) []gameConfigJSON {
	out := make([]gameConfigJSON, 0, len(configs))
	for _, item := range configs {
		out = append(out, gameConfigJSON{
			Game:                item.Game,
			TargetExpectedValue: item.TargetExpectedValue,
			ActualExpectedValue: item.ActualExpectedValue,
		})
	}
	return out
}

func gameConfigsFromJSON(configs []gameConfigJSON) ([]GameConfig, map[string]int) {
	if configs == nil {
		return nil, nil
	}
	out := make([]GameConfig, 0, len(configs))
	legacyDrawCounts := map[string]int{}
	for _, item := range configs {
		out = append(out, GameConfig{
			Game:                item.Game,
			TargetExpectedValue: item.TargetExpectedValue,
			ActualExpectedValue: item.ActualExpectedValue,
		})
		game := normalizeGameMode(item.Game)
		if game != "" && item.DrawCount > 0 {
			legacyDrawCounts[game] = item.DrawCount
		}
	}
	return out, legacyDrawCounts
}

func applyLegacyGameDrawCounts(cfg Config, legacyDrawCounts map[string]int) Config {
	if len(legacyDrawCounts) == 0 {
		return cfg
	}
	routes := normalizeGameRoutes(cfg.GameRoutes)
	for i := range routes {
		if routes[i].DrawCount > 0 {
			continue
		}
		if count := legacyDrawCounts[routes[i].Game]; count > 0 {
			routes[i].DrawCount = count
		}
	}
	cfg.GameRoutes = routes
	cfg.ScratchTiers = scratchTiersFromGameRoutes(routes)
	return cfg
}

func normalizeSpinGuarantees(in []SpinGuaranteeRule) []SpinGuaranteeRule {
	out := []SpinGuaranteeRule{}
	for _, rule := range in {
		if rule.DollarTier <= 0 || rule.PrizeDollars <= 0 {
			continue
		}
		if rule.RemainingSpins < 0 {
			rule.RemainingSpins = 0
		}
		if rule.MaxWonBelow < 0 {
			rule.MaxWonBelow = 0
		}
		out = append(out, rule)
	}
	return out
}

func normalizePositiveFloatList(in []float64) []float64 {
	seen := map[float64]bool{}
	out := []float64{}
	for _, value := range in {
		if value <= 0 || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func normalizeScratchRewards(in []int) []int {
	out := []int{}
	for _, reward := range in {
		if reward > 0 {
			out = append(out, reward)
		}
	}
	return out
}

func normalizeScratchMaxReveals(value int) int {
	maxSafeCells := ScratchCells - ScratchMines
	if maxSafeCells < 1 {
		maxSafeCells = 1
	}
	if value <= 0 {
		return ScratchMaxReveals
	}
	if value > maxSafeCells {
		return maxSafeCells
	}
	return value
}

func spinMapToJSON(in map[float64]int) map[string]int {
	out := map[string]int{}
	keys := make([]float64, 0, len(in))
	for dollars := range in {
		keys = append(keys, dollars)
	}
	sort.Float64s(keys)
	for _, dollars := range keys {
		out[strconv.FormatFloat(dollars, 'f', -1, 64)] = in[dollars]
	}
	return out
}

func spinMapFromJSON(in map[string]int) map[float64]int {
	if in == nil {
		return nil
	}
	out := map[float64]int{}
	for key, spins := range in {
		dollars, err := strconv.ParseFloat(strings.TrimSpace(key), 64)
		if err == nil && dollars > 0 && spins > 0 {
			out[dollars] = spins
		}
	}
	return out
}

func DollarsTier(quota, quotaUnit int64) float64 {
	if quotaUnit <= 0 {
		return 0
	}
	return float64(int64(math.Round(float64(quota) / float64(quotaUnit))))
}

func PlanLoginCard(input LoginCardPlanInput) LoginCardPlanResult {
	if input.Status != 1 {
		return LoginCardPlanResult{Rejection: LoginCardDisabled}
	}

	cfg := CloneConfig(input.Config)
	dollars := 0.0
	source := "shop"
	purchaseTime := ""
	createdInRange := false
	if actTestDollars, ok := ActTestDollarsFromName(input.Name); ok {
		dollars = actTestDollars
		source = "act"
		createdInRange = true
	} else {
		createdInRange = input.CreatedTime >= cfg.StartTS && input.CreatedTime <= cfg.EndTS
		purchased := input.ShopPurchaseTime != "" && input.ShopPurchaseTime >= cfg.StartText && input.ShopPurchaseTime <= cfg.EndText
		if !createdInRange && !purchased {
			return LoginCardPlanResult{Rejection: LoginCardOutsideWindow}
		}
		dollars = DollarsTier(input.IntervalQuota, input.QuotaUnit)
		purchaseTime = input.ShopPurchaseTime
	}

	game := cfg.GameForTier(dollars)
	drawCount := cfg.DrawCountForTier(dollars)
	isScratch := game == GameScratch && (purchaseTime != "" || createdInRange)
	if dollars == 0 || drawCount <= 0 || (game == GameScratch && !isScratch) {
		return LoginCardPlanResult{Rejection: LoginCardIneligible}
	}
	contribution, _ := DynamicPoolContributionForTier(cfg, dollars)
	return LoginCardPlanResult{Plan: LoginCardPlan{
		CardKey:          input.CardKey,
		CardName:         input.Name,
		Dollars:          dollars,
		TotalDraws:       drawCount,
		Game:             game,
		Source:           source,
		PurchaseTime:     purchaseTime,
		PoolContribution: contribution,
	}}
}

func ActTestDollarsFromName(name string) (float64, bool) {
	prefix, suffix, ok := strings.Cut(strings.TrimSpace(name), "-act-")
	if !ok {
		return 0, false
	}
	dollars, err := strconv.ParseFloat(strings.TrimSpace(prefix), 64)
	if err != nil || dollars <= 0 {
		return 0, false
	}
	for _, part := range strings.FieldsFunc(suffix, isTestCardNameSeparator) {
		if IsTestCardSegment(part) {
			return dollars, true
		}
	}
	return 0, false
}

func IsTestCardName(name string) bool {
	for _, part := range strings.FieldsFunc(strings.TrimSpace(name), isTestCardNameSeparator) {
		if IsTestCardSegment(part) {
			return true
		}
	}
	return false
}

func IsTestCardSegment(value string) bool {
	return strings.EqualFold(strings.TrimSpace(value), "test")
}

func isTestCardNameSeparator(r rune) bool {
	return r == '-' || r == '_' || r == ' ' || r == '.'
}

func Spin(dollars float64, hasJackpot bool, used, total, maxWon, force int, randomInt func(max int) int) SpinResult {
	return SpinWithConfig(DefaultConfig(), dollars, hasJackpot, used, total, maxWon, force, randomInt)
}

func SpinWithConfig(cfg Config, dollars float64, hasJackpot bool, used, total, maxWon, force int, randomInt func(max int) int) SpinResult {
	cfg = NormalizeConfig(cfg)
	return spincore.Spin(spinCoreConfig(cfg, dollars, total), dollars, hasJackpot, used, total, maxWon, force, randomInt)
}

func SpinWithPoolBalance(cfg Config, dollars float64, hasJackpot bool, used, total, maxWon, force int, poolBalance float64, randomInt func(max int) int) SpinResult {
	cfg = NormalizeConfig(cfg)
	return spincore.Spin(SpinCoreConfigForPoolBalance(cfg, GameSlot, total, poolBalance), dollars, hasJackpot, used, total, maxWon, force, randomInt)
}

func Roll(pool []Prize, randomInt func(max int) int) Prize {
	return prizepoolcore.Roll(pool, randomInt)
}

func SpinCoreConfigForPoolBalance(cfg Config, game string, drawCount int, poolBalance float64) spincore.Config {
	cfg = NormalizeConfig(cfg)
	gameConfig := cfg.GameConfigFor(game)
	pool := cfg.PrizePool
	jackpotPrizeDollars := cfg.JackpotPrizeDollars
	if cfg.DynamicPrizePool.Enabled {
		pool = PrizePoolWithDynamicAwards(cfg, poolBalance)
		if amount := prizeDollarsForRank(pool, "jackpot"); amount > 0 {
			jackpotPrizeDollars = amount
		}
	}
	return spincore.Config{
		TargetExpectedValue:    gameConfig.TargetExpectedValue,
		ActualExpectedValue:    gameConfig.ActualExpectedValue,
		DrawCount:              drawCount,
		PrizePool:              pool,
		GuaranteeRules:         cfg.SpinGuarantees,
		JackpotPrizeDollars:    jackpotPrizeDollars,
		JackpotEligibleDollars: cfg.JackpotEligibleDollars,
	}
}

func spinCoreConfig(cfg Config, dollars float64, total int) spincore.Config {
	gameConfig := cfg.GameConfigFor(GameSlot)
	drawCount := total
	if drawCount <= 0 {
		drawCount = cfg.DrawCountForTier(dollars)
	}
	return spincore.Config{
		TargetExpectedValue:    gameConfig.TargetExpectedValue,
		ActualExpectedValue:    gameConfig.ActualExpectedValue,
		DrawCount:              drawCount,
		PrizePool:              cfg.PrizePool,
		GuaranteeRules:         cfg.SpinGuarantees,
		JackpotPrizeDollars:    cfg.JackpotPrizeDollars,
		JackpotEligibleDollars: cfg.JackpotEligibleDollars,
	}
}

func prizeDollarsForRank(pool []Prize, rank string) int {
	for _, prize := range pool {
		if prize.Rank == rank {
			return prize.Dollars
		}
	}
	return 0
}
