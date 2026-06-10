package activity

import "math"

const (
	StartText               = "2026-05-01 00:00:00"
	EndText                 = "2026-05-08 23:59:59"
	StartTS           int64 = 1777564800
	EndTS             int64 = 1778255999
	ScratchMines            = 2
	ScratchMaxReveals       = 6
)

type Prize struct {
	Type    string
	Dollars int
	Weight  int
}

type SpinResult struct {
	Type    string
	Dollars int
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

func clonePrizePool(pool []Prize) []Prize {
	return append([]Prize(nil), pool...)
}

func DollarsTier(quota, quotaUnit int64) float64 {
	if quotaUnit <= 0 {
		return 0
	}
	return float64(int64(math.Round(float64(quota) / float64(quotaUnit))))
}

func Spin(dollars float64, hasJackpot bool, used, total, maxWon, force int, randomInt func(max int) int) SpinResult {
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
	pool := defaultPrizePool
	if hasJackpot || (dollars >= 100 && float64(maxWon) >= dollars*0.5) {
		pool = defaultPostJackpotPool
	} else if p, ok := defaultTierPools[int(dollars)]; ok {
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
