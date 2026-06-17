package spincore

import "fufu/prizepoolcore"

type Prize = prizepoolcore.Prize

type Result struct {
	Type       string
	Dollars    int
	Rank       string
	Label      string
	Advertised bool
}

type GuaranteeRule struct {
	DollarTier     int `json:"dollarTier"`
	RemainingSpins int `json:"remainingSpins"`
	MaxWonBelow    int `json:"maxWonBelow,omitempty"`
	PrizeDollars   int `json:"prizeDollars"`
}

type Config struct {
	TargetExpectedValue    float64
	ActualExpectedValue    float64
	DrawCount              int
	PrizePool              []Prize
	GuaranteeRules         []GuaranteeRule
	JackpotPrizeDollars    int
	JackpotEligibleDollars []float64
}

func Spin(cfg Config, dollars float64, hasJackpot bool, used, total, maxWon, force int, randomInt func(max int) int) Result {
	remaining := total - used
	if force > 0 {
		return Result{Type: "win", Dollars: force}
	}
	if prize, ok := guaranteedPrize(cfg.GuaranteeRules, dollars, remaining, maxWon); ok {
		return Result{Type: "win", Dollars: prize}
	}
	pool := cfg.PrizePool
	if cfg.TargetExpectedValue > 0 {
		pool = prizepoolcore.BalancePoolForPlan(prizepoolcore.BalanceInput{
			Pool:                pool,
			TargetExpectedValue: cfg.TargetExpectedValue,
			ActualExpectedValue: cfg.ActualExpectedValue,
			DrawCount:           cfg.DrawCount,
		}).Pool
	}
	p := Roll(pool, randomInt)
	if isBlockedJackpotPrize(cfg, p, dollars, hasJackpot) {
		return Result{Type: "retry"}
	}
	return Result{Type: p.Type, Dollars: p.Dollars, Rank: p.Rank, Label: p.Label, Advertised: p.Advertised}
}

func guaranteedPrize(rules []GuaranteeRule, dollars float64, remaining, maxWon int) (int, bool) {
	for _, rule := range rules {
		if rule.DollarTier <= 0 || int(dollars) != rule.DollarTier {
			continue
		}
		if rule.RemainingSpins > 0 && remaining != rule.RemainingSpins {
			continue
		}
		if rule.MaxWonBelow > 0 && maxWon >= rule.MaxWonBelow {
			continue
		}
		if rule.PrizeDollars <= 0 {
			continue
		}
		return rule.PrizeDollars, true
	}
	return 0, false
}

func isBlockedJackpotPrize(cfg Config, prize Prize, dollars float64, hasJackpot bool) bool {
	if prize.Type != "win" || cfg.JackpotPrizeDollars <= 0 || prize.Dollars != cfg.JackpotPrizeDollars {
		return false
	}
	return hasJackpot || !containsFloat(cfg.JackpotEligibleDollars, dollars)
}

func containsFloat(values []float64, want float64) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func Roll(pool []Prize, randomInt func(max int) int) Prize {
	return prizepoolcore.Roll(pool, randomInt)
}
