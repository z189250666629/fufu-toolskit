package activityapp

import (
	"errors"
	"math"
	"sort"

	"fufu/activity"
	"fufu/scratchcore"
	"net/http"
)

const scratchDynamicPoolRate = 0.10

func scratchGameResponse(g ScratchGame) (map[string]any, error) {
	return scratchAppResponse(scratchcore.GameResponse(g.MinePos, g.Revealed, g.PrizeDollars, g.Status, scratchMaxReveals, scratchMines, scratchCellCount))
}

func scratchStartResponse(g ScratchGame) (map[string]any, error) {
	return scratchAppResponse(scratchcore.StartResponse(g.MinePos, g.Revealed, g.PrizeDollars, g.Status, scratchMaxReveals, scratchMines, scratchCellCount))
}

func scratchAppResponse(response map[string]any, err error) (map[string]any, error) {
	if err != nil {
		return nil, scratchAppError(err)
	}
	return response, nil
}

func isScratchGameOver(status string) bool {
	return scratchcore.IsGameOver(status)
}

func scratchPrizeForSafeCount(safe int) (int, bool) {
	return scratchcore.PrizeForSafeCount(SnapshotRuntimeConfig().ScratchRewards, safe)
}

func scratchRewardsForCurrentPool() ([]int, error) {
	cfg := SnapshotRuntimeConfig()
	if !cfg.DynamicPrizePool.Enabled {
		return fixedLengthScratchRewardProfile(cfg.ScratchRewards, scratchMaxReveals), nil
	}
	balance, err := currentPrizePoolBalance()
	if err != nil {
		return nil, err
	}
	return scratchRewardsForPoolBalance(cfg, balance), nil
}

func scratchRewardsForPoolBalance(cfg activity.Config, poolBalance float64) []int {
	cfg = activity.NormalizeConfig(cfg)
	rewards := fixedLengthScratchRewardProfile(cfg.ScratchRewards, scratchMaxReveals)
	if !cfg.DynamicPrizePool.Enabled {
		return rewards
	}
	fullPrize := int(math.Round(math.Max(0, poolBalance) * scratchDynamicPoolRate))
	if fullPrize <= 0 {
		return make([]int, scratchMaxReveals)
	}
	fullWeight := rewards[len(rewards)-1]
	if fullWeight <= 0 {
		rewards = fixedLengthScratchRewardProfile(activity.DefaultScratchRewards(), scratchMaxReveals)
		fullWeight = rewards[len(rewards)-1]
	}
	out := make([]int, len(rewards))
	last := 0
	for i, weight := range rewards {
		prize := int(math.Round(float64(fullPrize) * float64(weight) / float64(fullWeight)))
		if prize < last {
			prize = last
		}
		if prize == 0 && fullPrize > 0 && weight > 0 {
			prize = 1
		}
		if prize > fullPrize {
			prize = fullPrize
		}
		out[i] = prize
		last = prize
	}
	out[len(out)-1] = fullPrize
	return out
}

func fixedLengthScratchRewardProfile(rewards []int, steps int) []int {
	if steps <= 0 {
		return nil
	}
	clean := make([]int, 0, len(rewards))
	for _, reward := range rewards {
		if reward > 0 {
			clean = append(clean, reward)
		}
	}
	if len(clean) == steps {
		return clean
	}
	defaults := activity.DefaultScratchRewards()
	if len(defaults) >= steps {
		return append([]int(nil), defaults[:steps]...)
	}
	out := make([]int, steps)
	for i := range out {
		out[i] = i + 1
	}
	return out
}

func minimumGuaranteedPrize(cfg activity.Config, scratchRewards []int) int {
	values := []int{}
	for _, reward := range scratchRewards {
		if reward > 0 {
			values = append(values, reward)
			break
		}
	}
	cfg = activity.NormalizeConfig(cfg)
	for _, guarantee := range cfg.SpinGuarantees {
		if guarantee.PrizeDollars > 0 {
			values = append(values, guarantee.PrizeDollars)
		}
	}
	if len(values) == 0 {
		return 0
	}
	sort.Ints(values)
	return values[0]
}

func parseScratchIntArray(s string) ([]int, error) {
	return scratchcore.ParseIntArray(s)
}

func parseScratchRevealedCells(s string) ([]int, error) {
	cells, err := scratchcore.ParseRevealedCells(s, scratchMaxReveals, scratchCellCount)
	if err != nil {
		return nil, scratchAppError(err)
	}
	return cells, nil
}

func parseScratchMineCells(s string) ([]int, error) {
	cells, err := scratchcore.ParseMineCells(s, scratchMines, scratchCellCount)
	if err != nil {
		return nil, scratchAppError(err)
	}
	return cells, nil
}

func validScratchCells(cells []int, maxCount int) bool {
	return scratchcore.ValidCells(cells, maxCount, scratchCellCount)
}

func intContains(a []int, v int) bool {
	return scratchcore.Contains(a, v)
}

func scratchSafeCount(revealed, mines []int) int {
	return scratchcore.SafeCount(revealed, mines)
}

func scratchAppError(err error) error {
	if errors.Is(err, scratchcore.ErrInvalidCells) {
		return httpErr{http.StatusBadRequest, "刮刮乐进度异常，请重开"}
	}
	if errors.Is(err, scratchcore.ErrInvalidCell) {
		return httpErr{http.StatusBadRequest, "无效的格子"}
	}
	if errors.Is(err, scratchcore.ErrCellAlreadyRevealed) {
		return httpErr{http.StatusBadRequest, "此格已刮开"}
	}
	if errors.Is(err, scratchcore.ErrGameNotPlaying) {
		return httpErr{http.StatusForbidden, "游戏已结束"}
	}
	if errors.Is(err, scratchcore.ErrNoSafeCell) {
		return httpErr{http.StatusBadRequest, "至少刮开一个安全格才能结算"}
	}
	return err
}
