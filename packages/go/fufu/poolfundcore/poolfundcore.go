package poolfundcore

import "math"

type Config struct {
	Enabled          bool            `json:"enabled,omitempty"`
	ContributionRate float64         `json:"contributionRate,omitempty"`
	JackpotRate      float64         `json:"jackpotRate,omitempty"`
	SecondRate       float64         `json:"secondRate,omitempty"`
	ThirdRate        float64         `json:"thirdRate,omitempty"`
	TierEconomics    []TierEconomics `json:"tierEconomics,omitempty"`
}

type TierEconomics struct {
	Dollars float64 `json:"dollars"`
	Revenue float64 `json:"revenue"`
	Cost    float64 `json:"cost"`
}

type Economics struct {
	Revenue float64
	Cost    float64
}

type ContributionResult struct {
	Revenue      float64
	Cost         float64
	NetProfit    float64
	Contribution float64
}

type PrizeAmounts struct {
	Jackpot        float64
	Second         float64
	Third          float64
	JackpotDollars int
	SecondDollars  int
	ThirdDollars   int
}

type PayoutPrize struct {
	Type       string
	Dollars    int
	Rank       string
	Advertised bool
}

func NormalizeConfig(cfg Config) Config {
	cfg.ContributionRate = clampRate(cfg.ContributionRate)
	cfg.JackpotRate = clampRate(cfg.JackpotRate)
	cfg.SecondRate = clampRate(cfg.SecondRate)
	cfg.ThirdRate = clampRate(cfg.ThirdRate)
	out := cfg
	out.TierEconomics = make([]TierEconomics, 0, len(cfg.TierEconomics))
	for _, item := range cfg.TierEconomics {
		if item.Dollars <= 0 {
			continue
		}
		if item.Revenue < 0 {
			item.Revenue = 0
		}
		if item.Cost < 0 {
			item.Cost = 0
		}
		out.TierEconomics = append(out.TierEconomics, item)
	}
	return out
}

func Contribution(e Economics, contributionRate float64) ContributionResult {
	net := e.Revenue - e.Cost
	contribution := 0.0
	if net > 0 {
		contribution = net * clampRate(contributionRate)
	}
	return ContributionResult{
		Revenue:      e.Revenue,
		Cost:         e.Cost,
		NetProfit:    net,
		Contribution: contribution,
	}
}

func AllocatePrizeAmounts(poolBalance float64, cfg Config) PrizeAmounts {
	cfg = NormalizeConfig(cfg)
	if !cfg.Enabled || poolBalance <= 0 {
		return PrizeAmounts{}
	}
	out := PrizeAmounts{
		Jackpot: poolBalance * cfg.JackpotRate,
		Second:  poolBalance * cfg.SecondRate,
		Third:   poolBalance * cfg.ThirdRate,
	}
	out.JackpotDollars = WholeDollars(out.Jackpot)
	out.SecondDollars = WholeDollars(out.Second)
	out.ThirdDollars = WholeDollars(out.Third)
	return out
}

func WholeDollars(value float64) int {
	if value <= 0 {
		return 0
	}
	return int(math.Floor(value))
}

func IsPayoutPrize(prize PayoutPrize) bool {
	if prize.Type != "win" || prize.Dollars <= 0 || !prize.Advertised {
		return false
	}
	switch prize.Rank {
	case "jackpot", "second", "third":
		return true
	default:
		return false
	}
}

func TierEconomicsForDollars(cfg Config, dollars float64) (Economics, bool) {
	cfg = NormalizeConfig(cfg)
	for _, item := range cfg.TierEconomics {
		if item.Dollars == dollars {
			return Economics{Revenue: item.Revenue, Cost: item.Cost}, true
		}
	}
	return Economics{}, false
}

func clampRate(value float64) float64 {
	if value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}
