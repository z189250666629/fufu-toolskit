package probabilitycore

import (
	"math"
	"sort"
	"strconv"
	"strings"
)

const epsilon = 0.000000001

type Outcome struct {
	ID     string
	Value  float64
	Weight int
}

type Weight struct {
	ID     string
	Weight int
}

type BalanceInput struct {
	Outcomes            []Outcome
	TargetExpectedValue float64
	ActualExpectedValue float64
	DrawCount           int
}

type BalanceResult struct {
	Weights                    []Weight
	TargetExpectedValue        float64
	InputActualExpectedValue   float64
	TargetPerDrawExpectedValue float64
	EffectiveExpectedValue     float64
	ActualExpectedValue        float64
	DrawCount                  int
	Reached                    bool
}

func ExpectedValue(outcomes []Outcome) float64 {
	total := 0
	weightedValue := 0.0
	for _, outcome := range normalizeOutcomes(outcomes) {
		total += outcome.Weight
		weightedValue += outcome.Value * float64(outcome.Weight)
	}
	if total <= 0 {
		return 0
	}
	return weightedValue / float64(total)
}

func BalanceWeights(input BalanceInput) BalanceResult {
	targetPerDraw := TargetPerDrawExpectedValue(input.TargetExpectedValue, input.ActualExpectedValue, input.DrawCount)
	outcomes := normalizeOutcomes(input.Outcomes)
	rawEV := ExpectedValue(outcomes)
	result := BalanceResult{
		Weights:                    weightsFromOutcomes(outcomes),
		TargetExpectedValue:        input.TargetExpectedValue,
		InputActualExpectedValue:   input.ActualExpectedValue,
		TargetPerDrawExpectedValue: targetPerDraw,
		EffectiveExpectedValue:     targetPerDraw,
		ActualExpectedValue:        rawEV,
		DrawCount:                  input.DrawCount,
		Reached:                    almostEqual(rawEV, targetPerDraw),
	}
	if len(outcomes) == 0 || result.Reached {
		return result
	}

	positiveWeight, neutralWeight, positiveValue := outcomeSums(outcomes)
	if positiveWeight <= 0 || positiveValue <= 0 {
		result.ActualExpectedValue = 0
		result.Reached = almostEqual(0, targetPerDraw)
		return result
	}
	if targetPerDraw <= 0 {
		result.Weights = zeroPositiveWeights(outcomes)
		result.ActualExpectedValue = 0
		result.Reached = true
		return result
	}

	winOnlyEV := positiveValue / float64(positiveWeight)
	desiredNeutral := 0
	reached := true
	if targetPerDraw >= winOnlyEV {
		reached = almostEqual(targetPerDraw, winOnlyEV)
	} else {
		desiredTotal := positiveValue / targetPerDraw
		desiredNeutral = int(math.Round(desiredTotal - float64(positiveWeight)))
		if desiredNeutral < 0 {
			desiredNeutral = 0
		}
	}

	result.Weights = rebalanceNeutralWeights(outcomes, neutralWeight, desiredNeutral)
	result.ActualExpectedValue = ExpectedValue(outcomesWithWeights(outcomes, result.Weights))
	result.Reached = reached && almostEqual(result.ActualExpectedValue, targetPerDraw)
	return result
}

func TargetPerDrawExpectedValue(targetExpectedValue, actualExpectedValue float64, drawCount int) float64 {
	if targetExpectedValue <= 0 {
		return 0
	}
	if drawCount <= 0 {
		drawCount = 1
	}
	plannedGameEV := targetExpectedValue + (targetExpectedValue - actualExpectedValue)
	if plannedGameEV < 0 {
		return 0
	}
	return plannedGameEV / float64(drawCount)
}

func normalizeOutcomes(outcomes []Outcome) []Outcome {
	out := make([]Outcome, 0, len(outcomes))
	for index, outcome := range outcomes {
		outcome.ID = strings.TrimSpace(outcome.ID)
		if outcome.ID == "" {
			outcome.ID = strconv.Itoa(index)
		}
		if outcome.Value < 0 || outcome.Weight <= 0 {
			continue
		}
		out = append(out, outcome)
	}
	return out
}

func weightsFromOutcomes(outcomes []Outcome) []Weight {
	weights := make([]Weight, 0, len(outcomes))
	for _, outcome := range outcomes {
		weights = append(weights, Weight{ID: outcome.ID, Weight: outcome.Weight})
	}
	return weights
}

func outcomeSums(outcomes []Outcome) (positiveWeight int, neutralWeight int, positiveValue float64) {
	for _, outcome := range outcomes {
		if outcome.Value > 0 {
			positiveWeight += outcome.Weight
			positiveValue += outcome.Value * float64(outcome.Weight)
			continue
		}
		neutralWeight += outcome.Weight
	}
	return positiveWeight, neutralWeight, positiveValue
}

func zeroPositiveWeights(outcomes []Outcome) []Weight {
	weights := make([]Weight, 0, len(outcomes))
	neutralKept := false
	for _, outcome := range outcomes {
		next := Weight{ID: outcome.ID, Weight: outcome.Weight}
		if outcome.Value > 0 {
			next.Weight = 0
		} else if next.Weight <= 0 {
			next.Weight = 1
		}
		if outcome.Value <= 0 && next.Weight > 0 {
			neutralKept = true
		}
		weights = append(weights, next)
	}
	if !neutralKept && len(weights) > 0 {
		weights[0].Weight = 1
	}
	return weights
}

func rebalanceNeutralWeights(outcomes []Outcome, oldNeutralWeight, newNeutralWeight int) []Weight {
	weights := make([]Weight, 0, len(outcomes))
	neutralRows := []Outcome{}
	for _, outcome := range outcomes {
		if outcome.Value > 0 {
			weights = append(weights, Weight{ID: outcome.ID, Weight: outcome.Weight})
			continue
		}
		neutralRows = append(neutralRows, outcome)
	}
	if newNeutralWeight <= 0 {
		for _, outcome := range neutralRows {
			weights = append(weights, Weight{ID: outcome.ID, Weight: 0})
		}
		return weights
	}
	if len(neutralRows) == 0 || oldNeutralWeight <= 0 {
		weights = append(weights, Weight{ID: "neutral", Weight: newNeutralWeight})
		return weights
	}
	allocations := allocateWeights(neutralRows, oldNeutralWeight, newNeutralWeight)
	for index, outcome := range neutralRows {
		weights = append(weights, Weight{ID: outcome.ID, Weight: allocations[index]})
	}
	return weights
}

func outcomesWithWeights(outcomes []Outcome, weights []Weight) []Outcome {
	byID := map[string]int{}
	for _, weight := range weights {
		byID[weight.ID] = weight.Weight
	}
	out := make([]Outcome, 0, len(outcomes))
	for _, outcome := range outcomes {
		outcome.Weight = byID[outcome.ID]
		if outcome.Weight > 0 {
			out = append(out, outcome)
		}
	}
	return out
}

type allocationRemainder struct {
	index     int
	remainder float64
}

func allocateWeights(rows []Outcome, oldTotal, newTotal int) []int {
	allocations := make([]int, len(rows))
	remainders := make([]allocationRemainder, 0, len(rows))
	used := 0
	for index, row := range rows {
		exact := float64(newTotal) * float64(row.Weight) / float64(oldTotal)
		base := int(math.Floor(exact))
		allocations[index] = base
		used += base
		remainders = append(remainders, allocationRemainder{index: index, remainder: exact - float64(base)})
	}
	sort.SliceStable(remainders, func(i, j int) bool {
		return remainders[i].remainder > remainders[j].remainder
	})
	for remaining := newTotal - used; remaining > 0; remaining-- {
		index := remainders[(newTotal-used-remaining)%len(remainders)].index
		allocations[index]++
	}
	return allocations
}

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) <= epsilon
}
