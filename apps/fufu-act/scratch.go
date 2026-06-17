package activityapp

import (
	"fufu/scratchcore"
	"net/http"
)

func handleScratchStart(w http.ResponseWriter, r *http.Request) {
	var b struct {
		CardKey string `json:"cardKey"`
	}
	key, ok, err := readCardKeyRequest(r, &b, func() string { return b.CardKey })
	if err != nil {
		writeCardKeyRequestError(w, err)
		return
	}
	if !ok {
		writeMissingCardKey(w)
		return
	}
	res, err := withCardLock(key, func() (any, error) {
		card, ok, lookupErr := lookupCard(key)
		if lookupErr != nil {
			return nil, lookupErr
		}
		if !ok {
			return nil, httpErr{404, "请先登录"}
		}
		if err := requireCurrentTokenActive(r.Context(), key); err != nil {
			return nil, err
		}
		if !isScratchDollarTier(card.Dollars) {
			return nil, httpErr{403, "此卡密不参与刮刮乐活动"}
		}
		if g, ok, lookupErr := lookupScratch(key); lookupErr != nil {
			return nil, lookupErr
		} else if ok {
			if g.Status == "playing" || !scratchCardHasRemainingRound(card) {
				return scratchStartResponse(g)
			}
			if err := replaceScratchGame(key, scratchMinesForNewGame()); err != nil {
				return nil, err
			}
			if next, ok, lookupErr := lookupScratch(key); lookupErr != nil {
				return nil, lookupErr
			} else if ok {
				return scratchStartResponse(next)
			}
			return nil, httpErr{500, "服务器错误"}
		}
		if !scratchCardHasRemainingRound(card) {
			return nil, httpErr{403, "刮刮乐次数已用完"}
		}
		mines := scratchMinesForNewGame()
		if err := insertScratchGame(key, mines); err != nil {
			if g, ok, lookupErr := lookupScratch(key); lookupErr != nil {
				return nil, lookupErr
			} else if ok {
				return scratchStartResponse(g)
			}
			return nil, err
		}
		return map[string]any{"cells": scratchCellCount, "revealed": []int{}, "prize": 0, "status": "playing"}, nil
	})
	if err != nil {
		writeHTTPError(w, err)
		return
	}
	writeJSON(w, 200, res)
}

func scratchCardHasRemainingRound(card Card) bool {
	return scratchcore.CanStartRound(card.TotalSpins, card.UsedSpins)
}

func scratchMinesForNewGame() []int {
	mines := []int{}
	for len(mines) < scratchMines {
		p := secureRandomInt(scratchCellCount)
		if !intContains(mines, p) {
			mines = append(mines, p)
		}
	}
	return mines
}

func handleScratchReveal(w http.ResponseWriter, r *http.Request) {
	var b struct {
		CardKey   string `json:"cardKey"`
		CellIndex *int   `json:"cellIndex"`
	}
	key, ok, err := readCardKeyRequest(r, &b, func() string { return b.CardKey })
	if err != nil {
		writeCardKeyRequestError(w, err)
		return
	}
	if !ok {
		writeMissingCardKey(w)
		return
	}
	if b.CellIndex == nil || *b.CellIndex < 0 || *b.CellIndex >= scratchCellCount {
		writeJSONError(w, 400, "无效的格子")
		return
	}
	cellIndex := *b.CellIndex
	res, err := withCardLock(key, func() (any, error) {
		g, ok, lookupErr := lookupScratch(key)
		if lookupErr != nil {
			return nil, lookupErr
		}
		if !ok {
			return nil, httpErr{404, "请先开始刮刮乐"}
		}
		if g.Status != "playing" {
			return nil, httpErr{403, "游戏已结束"}
		}
		if _, err := requireScratchEligibleCard(r.Context(), key); err != nil {
			return nil, err
		}
		revealed, err := parseScratchRevealedCells(g.Revealed)
		if err != nil {
			return nil, err
		}
		mines, err := parseScratchMineCells(g.MinePos)
		if err != nil {
			return nil, err
		}
		result, err := scratchcore.Reveal(scratchcore.Game{
			MineCells: mines,
			Revealed:  revealed,
			Prize:     g.PrizeDollars,
			Status:    g.Status,
		}, cellIndex, SnapshotRuntimeConfig().ScratchRewards, scratchMaxReveals, scratchMines, scratchCellCount)
		if err != nil {
			return nil, scratchAppError(err)
		}
		if result.Hit {
			if err := updateScratchLost(key, result.Revealed); err != nil {
				return nil, err
			}
			return map[string]any{"hit": true, "mines": result.Mines, "prize": result.Prize, "status": result.Status, "revealed": result.Revealed}, nil
		}
		if result.Status == "won" && result.Prize > 0 {
			if err := updateScratchWonWithCredit(key, result.Revealed, result.Prize, result.Status); err != nil {
				return nil, err
			}
		} else {
			if err := updateScratchProgress(key, result.Revealed, result.Prize, result.Status); err != nil {
				return nil, err
			}
		}
		response := map[string]any{"hit": false, "prize": result.Prize, "status": result.Status, "revealed": result.Revealed}
		if isScratchGameOver(result.Status) {
			response["mines"] = result.Mines
		}
		return response, nil
	})
	if err != nil {
		writeHTTPError(w, err)
		return
	}
	writeJSON(w, 200, res)
}

func handleScratchCashout(w http.ResponseWriter, r *http.Request) {
	var b struct {
		CardKey string `json:"cardKey"`
	}
	key, ok, err := readCardKeyRequest(r, &b, func() string { return b.CardKey })
	if err != nil {
		writeCardKeyRequestError(w, err)
		return
	}
	if !ok {
		writeMissingCardKey(w)
		return
	}
	res, err := withCardLock(key, func() (any, error) {
		g, ok, lookupErr := lookupScratch(key)
		if lookupErr != nil {
			return nil, lookupErr
		}
		if !ok {
			return nil, httpErr{404, "请先开始刮刮乐"}
		}
		if g.Status != "playing" {
			return nil, httpErr{403, "游戏已结束"}
		}
		if _, err := requireScratchEligibleCard(r.Context(), key); err != nil {
			return nil, err
		}
		revealed, err := parseScratchRevealedCells(g.Revealed)
		if err != nil {
			return nil, err
		}
		mines, err := parseScratchMineCells(g.MinePos)
		if err != nil {
			return nil, err
		}
		result, err := scratchcore.Cashout(scratchcore.Game{
			MineCells: mines,
			Revealed:  revealed,
			Prize:     g.PrizeDollars,
			Status:    g.Status,
		}, SnapshotRuntimeConfig().ScratchRewards, scratchMaxReveals, scratchMines, scratchCellCount)
		if err != nil {
			return nil, scratchAppError(err)
		}
		if result.Prize > 0 {
			if err := updateScratchCashoutWithCredit(key, result.Prize); err != nil {
				return nil, err
			}
		} else {
			if err := updateScratchCashout(key, result.Prize); err != nil {
				return nil, err
			}
		}
		return map[string]any{"prize": result.Prize, "status": result.Status, "revealed": result.Revealed, "mines": result.Mines}, nil
	})
	if err != nil {
		writeHTTPError(w, err)
		return
	}
	writeJSON(w, 200, res)
}

func handleScratchReset(w http.ResponseWriter, r *http.Request) {
	var b struct {
		CardKey string `json:"cardKey"`
	}
	key, ok, err := readCardKeyRequest(r, &b, func() string { return b.CardKey })
	if err != nil {
		writeCardKeyRequestError(w, err)
		return
	}
	if !ok {
		writeMissingCardKey(w)
		return
	}
	res, err := withCardLock(key, func() (any, error) {
		card, ok, lookupErr := lookupCard(key)
		if lookupErr != nil {
			return nil, lookupErr
		}
		if !ok {
			return nil, httpErr{404, "请先登录"}
		}
		if !isTestCardName(card.CardName) {
			return nil, httpErr{403, "仅测试卡可重开"}
		}
		if g, ok, lookupErr := lookupScratch(key); lookupErr != nil {
			return nil, lookupErr
		} else if ok && g.Status == "playing" {
			return nil, httpErr{400, "当前游戏尚未结束"}
		}
		if err := resetScratchTestGame(key); err != nil {
			return nil, err
		}
		return map[string]any{"ok": true}, nil
	})
	if err != nil {
		writeHTTPError(w, err)
		return
	}
	writeJSON(w, 200, res)
}
