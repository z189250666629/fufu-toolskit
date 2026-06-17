package prizepoolcore

import (
	"fufu/probabilitycore"
	"strconv"
	"strings"
)

type Prize struct {
	Type       string `json:"type"`
	Dollars    int    `json:"dollars"`
	Weight     int    `json:"weight"`
	Rank       string `json:"rank,omitempty"`
	Label      string `json:"label,omitempty"`
	Advertised bool   `json:"advertised,omitempty"`
}

type Weight = probabilitycore.Weight

type BalanceInput struct {
	Pool                []Prize
	TargetExpectedValue float64
	ActualExpectedValue float64
	DrawCount           int
}

type BalanceResult struct {
	Pool                       []Prize
	TargetExpectedValue        float64
	InputActualExpectedValue   float64
	TargetPerDrawExpectedValue float64
	EffectiveExpectedValue     float64
	ActualExpectedValue        float64
	DrawCount                  int
	Reached                    bool
}

func ExpectedValue(pool []Prize) float64 {
	return probabilitycore.ExpectedValue(Outcomes(pool))
}

func BalancePool(pool []Prize, targetExpectedValue float64) BalanceResult {
	return BalancePoolForPlan(BalanceInput{
		Pool:                pool,
		TargetExpectedValue: targetExpectedValue,
		ActualExpectedValue: targetExpectedValue,
		DrawCount:           1,
	})
}

func BalancePoolForPlan(input BalanceInput) BalanceResult {
	normalized := Normalize(input.Pool)
	plan := probabilitycore.BalanceWeights(probabilitycore.BalanceInput{
		Outcomes:            Outcomes(normalized),
		TargetExpectedValue: input.TargetExpectedValue,
		ActualExpectedValue: input.ActualExpectedValue,
		DrawCount:           input.DrawCount,
	})
	pool := ApplyWeights(normalized, plan.Weights)
	actual := ExpectedValue(pool)
	return BalanceResult{
		Pool:                       pool,
		TargetExpectedValue:        plan.TargetExpectedValue,
		InputActualExpectedValue:   plan.InputActualExpectedValue,
		TargetPerDrawExpectedValue: plan.TargetPerDrawExpectedValue,
		EffectiveExpectedValue:     plan.EffectiveExpectedValue,
		ActualExpectedValue:        actual,
		DrawCount:                  plan.DrawCount,
		Reached:                    plan.Reached,
	}
}

func Normalize(pool []Prize) []Prize {
	out := make([]Prize, 0, len(pool))
	for _, prize := range pool {
		prize.Type = strings.ToLower(strings.TrimSpace(prize.Type))
		prize.Rank = strings.TrimSpace(prize.Rank)
		prize.Label = strings.TrimSpace(prize.Label)
		if prize.Type == "" {
			if prize.Dollars > 0 {
				prize.Type = "win"
			} else {
				prize.Type = "miss"
			}
		}
		if prize.Weight <= 0 && !prize.Advertised {
			continue
		}
		if prize.Type != "win" {
			prize.Dollars = 0
		}
		out = append(out, prize)
	}
	return out
}

func Outcomes(pool []Prize) []probabilitycore.Outcome {
	normalized := Normalize(pool)
	out := make([]probabilitycore.Outcome, 0, len(normalized))
	for index, prize := range normalized {
		value := 0.0
		if prize.Type == "win" && prize.Dollars > 0 {
			value = float64(prize.Dollars)
		}
		out = append(out, probabilitycore.Outcome{
			ID:     strconv.Itoa(index),
			Value:  value,
			Weight: prize.Weight,
		})
	}
	return out
}

func ApplyWeights(pool []Prize, weights []Weight) []Prize {
	normalized := Normalize(pool)
	byID := map[string]int{}
	for _, weight := range weights {
		byID[weight.ID] = weight.Weight
	}
	out := make([]Prize, 0, len(normalized))
	for index, prize := range normalized {
		if weight, ok := byID[strconv.Itoa(index)]; ok {
			prize.Weight = weight
		}
		if prize.Weight > 0 || prize.Advertised {
			out = append(out, prize)
		}
	}
	return out
}

func Roll(pool []Prize, randomInt func(max int) int) Prize {
	total := 0
	normalized := Normalize(pool)
	for _, prize := range normalized {
		if prize.Weight > 0 {
			total += prize.Weight
		}
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
	current := 0
	for _, prize := range normalized {
		if prize.Weight <= 0 {
			continue
		}
		current += prize.Weight
		if n < current {
			return prize
		}
	}
	return Prize{Type: "miss"}
}

func TargetPerDrawExpectedValue(targetExpectedValue, actualExpectedValue float64, drawCount int) float64 {
	return probabilitycore.TargetPerDrawExpectedValue(targetExpectedValue, actualExpectedValue, drawCount)
}
