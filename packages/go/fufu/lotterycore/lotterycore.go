package lotterycore

import (
	"fufu/prizepoolcore"
	"fufu/probabilitycore"
)

type Prize = prizepoolcore.Prize
type Weight = prizepoolcore.Weight
type BalanceInput = prizepoolcore.BalanceInput
type BalanceResult = prizepoolcore.BalanceResult

func ExpectedValue(pool []Prize) float64 {
	return prizepoolcore.ExpectedValue(pool)
}

func BalancePool(pool []Prize, targetExpectedValue float64) BalanceResult {
	return prizepoolcore.BalancePool(pool, targetExpectedValue)
}

func BalancePoolForPlan(input BalanceInput) BalanceResult {
	return prizepoolcore.BalancePoolForPlan(input)
}

func EffectiveExpectedValue(targetExpectedValue, actualExpectedValue float64, drawCount int) float64 {
	return probabilitycore.TargetPerDrawExpectedValue(targetExpectedValue, actualExpectedValue, drawCount)
}

func TargetPerDrawExpectedValue(targetExpectedValue, actualExpectedValue float64, drawCount int) float64 {
	return probabilitycore.TargetPerDrawExpectedValue(targetExpectedValue, actualExpectedValue, drawCount)
}

func Roll(pool []Prize, randomInt func(max int) int) Prize {
	return prizepoolcore.Roll(pool, randomInt)
}
