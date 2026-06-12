package activity

import (
	"encoding/json"
	"math"
	"sort"
	"strconv"
	"strings"
)

const (
	StartText               = "2026-05-01 00:00:00"
	EndText                 = "2026-05-08 23:59:59"
	StartTS           int64 = 1777564800
	EndTS             int64 = 1778255999
	ScratchMines            = 2
	ScratchMaxReveals       = 6
)

type Prize struct {
	Type    string `json:"type"`
	Dollars int    `json:"dollars"`
	Weight  int    `json:"weight"`
}

type SpinResult struct {
	Type    string
	Dollars int
}

type Config struct {
	StartText           string          `json:"startText"`
	EndText             string          `json:"endText"`
	StartTS             int64           `json:"startTS"`
	EndTS               int64           `json:"endTS"`
	TargetExpectedValue float64         `json:"targetExpectedValue,omitempty"`
	SpinMap             map[float64]int `json:"spinMap"`
	PrizePool           []Prize         `json:"prizePool"`
	TierPools           map[int][]Prize `json:"tierPools"`
	PostJackpotPool     []Prize         `json:"postJackpotPrizes"`
	ScratchRewards      []int           `json:"scratchRewards"`
}

type configJSON struct {
	StartText           string             `json:"startText"`
	EndText             string             `json:"endText"`
	StartTS             int64              `json:"startTS"`
	EndTS               int64              `json:"endTS"`
	TargetExpectedValue float64            `json:"targetExpectedValue,omitempty"`
	SpinMap             map[string]int     `json:"spinMap"`
	PrizePool           []Prize            `json:"prizePool"`
	TierPools           map[string][]Prize `json:"tierPools"`
	PostJackpotPool     []Prize            `json:"postJackpotPrizes"`
	ScratchRewards      []int              `json:"scratchRewards"`
}

var defaultSpinMap = map[float64]int{0.1: 100, 100: 1, 150: 1, 300: 3, 500: 4, 1000: 10}

var defaultPrizePool = []Prize{{"miss", 0, 500}, {"retry", 0, 500}, {"win", 1, 1500}, {"win", 5, 3000}, {"win", 10, 2000}, {"win", 20, 1200}, {"win", 50, 580}, {"win", 100, 380}, {"win", 200, 200}, {"win", 500, 100}, {"win", 1000, 40}}

var defaultTierPools = map[int][]Prize{
	100:  {{"miss", 0, 500}, {"retry", 0, 500}, {"win", 1, 1800}, {"win", 5, 3500}, {"win", 10, 2300}, {"win", 20, 1000}, {"win", 50, 250}, {"win", 100, 100}, {"win", 200, 30}, {"win", 500, 15}, {"win", 1000, 5}},
	150:  {{"miss", 0, 500}, {"retry", 0, 500}, {"win", 1, 1200}, {"win", 5, 3000}, {"win", 10, 2500}, {"win", 20, 1500}, {"win", 50, 500}, {"win", 100, 180}, {"win", 200, 70}, {"win", 500, 35}, {"win", 1000, 15}},
	300:  {{"miss", 0, 500}, {"retry", 0, 500}, {"win", 1, 2200}, {"win", 5, 3300}, {"win", 10, 2000}, {"win", 20, 1000}, {"win", 50, 300}, {"win", 100, 120}, {"win", 200, 50}, {"win", 500, 20}, {"win", 1000, 10}},
	500:  {{"miss", 0, 500}, {"retry", 0, 500}, {"win", 1, 1500}, {"win", 5, 3100}, {"win", 10, 2100}, {"win", 20, 1200}, {"win", 50, 580}, {"win", 100, 300}, {"win", 200, 150}, {"win", 500, 40}, {"win", 1000, 30}},
	1000: {{"miss", 0, 500}, {"retry", 0, 500}, {"win", 1, 1500}, {"win", 5, 3000}, {"win", 10, 2000}, {"win", 20, 1200}, {"win", 50, 580}, {"win", 100, 380}, {"win", 200, 200}, {"win", 500, 120}, {"win", 1000, 20}},
}

var defaultPostJackpotPool = []Prize{{"miss", 0, 500}, {"retry", 0, 500}, {"win", 1, 3000}, {"win", 5, 3500}, {"win", 10, 1700}, {"win", 20, 800}}
var defaultScratchRewards = []int{2, 4, 6, 8, 12, 15}

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

func DefaultTierPools() map[int][]Prize {
	out := make(map[int][]Prize, len(defaultTierPools))
	for dollars, pool := range defaultTierPools {
		out[dollars] = clonePrizePool(pool)
	}
	return out
}

func DefaultPostJackpotPool() []Prize {
	return clonePrizePool(defaultPostJackpotPool)
}

func DefaultScratchRewards() []int {
	return append([]int(nil), defaultScratchRewards...)
}

func DefaultConfig() Config {
	return Config{
		StartText:           StartText,
		EndText:             EndText,
		StartTS:             StartTS,
		EndTS:               EndTS,
		TargetExpectedValue: ExpectedValue(defaultPrizePool),
		SpinMap:             DefaultSpinMap(),
		PrizePool:           DefaultPrizePool(),
		TierPools:           DefaultTierPools(),
		PostJackpotPool:     DefaultPostJackpotPool(),
		ScratchRewards:      DefaultScratchRewards(),
	}
}

func CloneConfig(cfg Config) Config {
	cfg = NormalizeConfig(cfg)
	out := cfg
	out.SpinMap = cloneSpinMap(cfg.SpinMap)
	out.PrizePool = clonePrizePool(cfg.PrizePool)
	out.TierPools = cloneTierPools(cfg.TierPools)
	out.PostJackpotPool = clonePrizePool(cfg.PostJackpotPool)
	out.ScratchRewards = append([]int(nil), cfg.ScratchRewards...)
	return out
}

func NormalizeConfig(cfg Config) Config {
	defaults := Config{
		StartText:           StartText,
		EndText:             EndText,
		StartTS:             StartTS,
		EndTS:               EndTS,
		TargetExpectedValue: ExpectedValue(defaultPrizePool),
		SpinMap:             DefaultSpinMap(),
		PrizePool:           DefaultPrizePool(),
		TierPools:           DefaultTierPools(),
		PostJackpotPool:     DefaultPostJackpotPool(),
		ScratchRewards:      DefaultScratchRewards(),
	}
	if strings.TrimSpace(cfg.StartText) == "" {
		cfg.StartText = defaults.StartText
	}
	if strings.TrimSpace(cfg.EndText) == "" {
		cfg.EndText = defaults.EndText
	}
	if cfg.StartTS <= 0 {
		cfg.StartTS = defaults.StartTS
	}
	if cfg.EndTS <= 0 {
		cfg.EndTS = defaults.EndTS
	}
	if cfg.TargetExpectedValue < 0 {
		cfg.TargetExpectedValue = 0
	}
	if cfg.TargetExpectedValue == 0 {
		cfg.TargetExpectedValue = defaults.TargetExpectedValue
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
	if cfg.TierPools == nil {
		cfg.TierPools = defaults.TierPools
	} else {
		cfg.TierPools = normalizeTierPools(cfg.TierPools)
	}
	if cfg.PostJackpotPool == nil {
		cfg.PostJackpotPool = defaults.PostJackpotPool
	} else {
		cfg.PostJackpotPool = normalizePrizePool(cfg.PostJackpotPool)
	}
	if cfg.ScratchRewards == nil {
		cfg.ScratchRewards = defaults.ScratchRewards
	} else {
		cfg.ScratchRewards = normalizeScratchRewards(cfg.ScratchRewards)
	}
	return cfg
}

func (cfg Config) MarshalJSON() ([]byte, error) {
	cfg = NormalizeConfig(cfg)
	return json.Marshal(configJSON{
		StartText:           cfg.StartText,
		EndText:             cfg.EndText,
		StartTS:             cfg.StartTS,
		EndTS:               cfg.EndTS,
		TargetExpectedValue: cfg.TargetExpectedValue,
		SpinMap:             spinMapToJSON(cfg.SpinMap),
		PrizePool:           cfg.PrizePool,
		TierPools:           tierPoolsToJSON(cfg.TierPools),
		PostJackpotPool:     cfg.PostJackpotPool,
		ScratchRewards:      cfg.ScratchRewards,
	})
}

func (cfg *Config) UnmarshalJSON(data []byte) error {
	var raw configJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*cfg = NormalizeConfig(Config{
		StartText:           raw.StartText,
		EndText:             raw.EndText,
		StartTS:             raw.StartTS,
		EndTS:               raw.EndTS,
		TargetExpectedValue: raw.TargetExpectedValue,
		SpinMap:             spinMapFromJSON(raw.SpinMap),
		PrizePool:           raw.PrizePool,
		TierPools:           tierPoolsFromJSON(raw.TierPools),
		PostJackpotPool:     raw.PostJackpotPool,
		ScratchRewards:      raw.ScratchRewards,
	})
	return nil
}

func ActualExpectedValue(cfg Config) float64 {
	cfg = NormalizeConfig(cfg)
	return ExpectedValue(cfg.PrizePool)
}

func ExpectedValue(pool []Prize) float64 {
	total := 0
	won := 0
	for _, p := range pool {
		if p.Weight <= 0 {
			continue
		}
		total += p.Weight
		if p.Type == "win" && p.Dollars > 0 {
			won += p.Dollars * p.Weight
		}
	}
	if total <= 0 {
		return 0
	}
	return float64(won) / float64(total)
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

func cloneTierPools(in map[int][]Prize) map[int][]Prize {
	out := make(map[int][]Prize, len(in))
	for dollars, pool := range in {
		out[dollars] = clonePrizePool(pool)
	}
	return out
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
		out = append(out, p)
	}
	return out
}

func normalizeTierPools(in map[int][]Prize) map[int][]Prize {
	out := map[int][]Prize{}
	for dollars, pool := range in {
		if dollars <= 0 {
			continue
		}
		out[dollars] = normalizePrizePool(pool)
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

func tierPoolsToJSON(in map[int][]Prize) map[string][]Prize {
	out := map[string][]Prize{}
	keys := make([]int, 0, len(in))
	for dollars := range in {
		keys = append(keys, dollars)
	}
	sort.Ints(keys)
	for _, dollars := range keys {
		out[strconv.Itoa(dollars)] = clonePrizePool(in[dollars])
	}
	return out
}

func tierPoolsFromJSON(in map[string][]Prize) map[int][]Prize {
	if in == nil {
		return nil
	}
	out := map[int][]Prize{}
	for key, pool := range in {
		dollars, err := strconv.Atoi(strings.TrimSpace(key))
		if err == nil && dollars > 0 {
			out[dollars] = normalizePrizePool(pool)
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

func Spin(dollars float64, hasJackpot bool, used, total, maxWon, force int, randomInt func(max int) int) SpinResult {
	return SpinWithConfig(DefaultConfig(), dollars, hasJackpot, used, total, maxWon, force, randomInt)
}

func SpinWithConfig(cfg Config, dollars float64, hasJackpot bool, used, total, maxWon, force int, randomInt func(max int) int) SpinResult {
	cfg = NormalizeConfig(cfg)
	remaining := total - used
	if force > 0 {
		return SpinResult{"win", force}
	}
	if int(dollars) == 1000 && remaining == 1 && maxWon < 50 {
		return SpinResult{"win", 100}
	}
	if int(dollars) == 500 && remaining == 1 && maxWon < 50 {
		return SpinResult{"win", 20}
	}
	pool := cfg.PrizePool
	if hasJackpot || (dollars >= 100 && float64(maxWon) >= dollars*0.5) {
		pool = cfg.PostJackpotPool
	} else if p, ok := cfg.TierPools[int(dollars)]; ok {
		pool = p
	}
	p := Roll(pool, randomInt)
	if p.Type == "win" && p.Dollars == 1000 && ((int(dollars) != 1000 && dollars != 0.1) || hasJackpot) {
		return SpinResult{"retry", 0}
	}
	return SpinResult{p.Type, p.Dollars}
}

func Roll(pool []Prize, randomInt func(max int) int) Prize {
	total := 0
	for _, p := range pool {
		total += p.Weight
	}
	if total <= 0 {
		return Prize{Type: "miss"}
	}
	if randomInt == nil {
		randomInt = func(max int) int { return 0 }
	}
	n := randomInt(total)
	if n < 0 {
		n = 0
	}
	if n >= total {
		n = total - 1
	}
	c := 0
	for _, p := range pool {
		c += p.Weight
		if n < c {
			return p
		}
	}
	return pool[0]
}
