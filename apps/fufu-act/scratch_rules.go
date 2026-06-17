package activityapp

import (
	"errors"
	"fufu/scratchcore"
	"net/http"
)

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
