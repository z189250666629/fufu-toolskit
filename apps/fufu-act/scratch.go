package main

import (
	"database/sql"
	"encoding/json"
	"errors"
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
			return scratchStartResponse(g)
		}
		mines := []int{}
		for len(mines) < scratchMines {
			p := secureRandomInt(9)
			exists := false
			for _, m := range mines {
				if m == p {
					exists = true
				}
			}
			if !exists {
				mines = append(mines, p)
			}
		}
		mb, _ := json.Marshal(mines)
		if _, err := db.Exec(`INSERT INTO scratch_games (card_key,mine_pos) VALUES (?,?)`, key, string(mb)); err != nil {
			if g, ok, lookupErr := lookupScratch(key); lookupErr != nil {
				return nil, lookupErr
			} else if ok {
				return scratchStartResponse(g)
			}
			return nil, err
		}
		return map[string]any{"cells": 9, "revealed": []int{}, "prize": 0, "status": "playing"}, nil
	})
	if err != nil {
		writeHTTPError(w, err)
		return
	}
	writeJSON(w, 200, res)
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
	if b.CellIndex == nil || *b.CellIndex < 0 || *b.CellIndex > 8 {
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
		for _, v := range revealed {
			if v == cellIndex {
				return nil, httpErr{400, "此格已刮开"}
			}
		}
		mines, err := parseScratchMineCells(g.MinePos)
		if err != nil {
			return nil, err
		}
		revealed = append(revealed, cellIndex)
		if intContains(mines, cellIndex) {
			rb, _ := json.Marshal(revealed)
			if _, err := db.Exec(`UPDATE scratch_games SET revealed=?, prize_dollars=0, status='lost' WHERE card_key=?`, string(rb), key); err != nil {
				return nil, err
			}
			return map[string]any{"hit": true, "mines": mines, "prize": 0, "status": "lost", "revealed": revealed}, nil
		}
		safe := 0
		for _, v := range revealed {
			if !intContains(mines, v) {
				safe++
			}
		}
		prize, ok := scratchPrizeForSafeCount(safe)
		if !ok {
			return nil, httpErr{400, "刮刮乐进度异常，请重开"}
		}
		status := "playing"
		if safe >= scratchMaxReveals {
			status = "won"
		}
		rb, _ := json.Marshal(revealed)
		if status == "won" && prize > 0 {
			if err := withTx(func(tx *sql.Tx) error {
				if _, err := tx.Exec(`UPDATE scratch_games SET revealed=?, prize_dollars=?, status=? WHERE card_key=?`, string(rb), prize, status, key); err != nil {
					return err
				}
				return enqueueCreditWith(tx, key, prize)
			}); err != nil {
				return nil, err
			}
		} else {
			if _, err := db.Exec(`UPDATE scratch_games SET revealed=?, prize_dollars=?, status=? WHERE card_key=?`, string(rb), prize, status, key); err != nil {
				return nil, err
			}
		}
		response := map[string]any{"hit": false, "prize": prize, "status": status, "revealed": revealed}
		if isScratchGameOver(status) {
			response["mines"] = mines
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
		safe := 0
		for _, v := range revealed {
			if !intContains(mines, v) {
				safe++
			}
		}
		if safe == 0 {
			return nil, httpErr{400, "至少刮开一个安全格才能结算"}
		}
		prize, ok := scratchPrizeForSafeCount(safe)
		if !ok {
			return nil, httpErr{400, "刮刮乐进度异常，请重开"}
		}
		if prize > 0 {
			if err := withTx(func(tx *sql.Tx) error {
				if _, err := tx.Exec(`UPDATE scratch_games SET prize_dollars=?, status='cashout' WHERE card_key=?`, prize, key); err != nil {
					return err
				}
				return enqueueCreditWith(tx, key, prize)
			}); err != nil {
				return nil, err
			}
		} else {
			if _, err := db.Exec(`UPDATE scratch_games SET prize_dollars=?, status='cashout' WHERE card_key=?`, prize, key); err != nil {
				return nil, err
			}
		}
		return map[string]any{"prize": prize, "status": "cashout", "revealed": revealed, "mines": mines}, nil
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
		if err := withTx(func(tx *sql.Tx) error {
			if _, err := tx.Exec(`UPDATE credit_queue SET status='archived', error='superseded by test scratch reset' WHERE card_key=? AND status='done'`, key); err != nil {
				return err
			}
			if _, err := tx.Exec(`DELETE FROM scratch_games WHERE card_key=?`, key); err != nil {
				return err
			}
			return nil
		}); err != nil {
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

func getScratch(key string) (ScratchGame, bool) {
	g, ok, _ := lookupScratch(key)
	return g, ok
}

func lookupScratch(key string) (ScratchGame, bool, error) {
	var g ScratchGame
	err := db.QueryRow(`SELECT id,card_key,mine_pos,revealed,prize_dollars,status FROM scratch_games WHERE card_key=?`, key).Scan(&g.ID, &g.CardKey, &g.MinePos, &g.Revealed, &g.PrizeDollars, &g.Status)
	if err == nil {
		return g, true, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return ScratchGame{}, false, nil
	}
	return ScratchGame{}, false, err
}

func scratchGameResponse(g ScratchGame) (map[string]any, error) {
	revealed, err := parseScratchRevealedCells(g.Revealed)
	if err != nil {
		return nil, err
	}
	response := map[string]any{"revealed": revealed, "prize": g.PrizeDollars, "status": g.Status}
	if isScratchGameOver(g.Status) {
		mines, err := parseScratchMineCells(g.MinePos)
		if err != nil {
			return nil, err
		}
		response["mines"] = mines
	}
	return response, nil
}

func scratchStartResponse(g ScratchGame) (map[string]any, error) {
	response, err := scratchGameResponse(g)
	if err != nil {
		return nil, err
	}
	response["cells"] = 9
	return response, nil
}

func isScratchGameOver(status string) bool {
	return status == "won" || status == "lost" || status == "cashout"
}

func scratchPrizeForSafeCount(safe int) (int, bool) {
	if safe <= 0 || safe > len(scratchRewards) {
		return 0, false
	}
	return scratchRewards[safe-1], true
}

func parseScratchIntArray(s string) ([]int, error) {
	var a []int
	if err := json.Unmarshal([]byte(s), &a); err != nil {
		return nil, err
	}
	if a == nil {
		return []int{}, nil
	}
	return a, nil
}

func parseScratchRevealedCells(s string) ([]int, error) {
	cells, err := parseScratchIntArray(s)
	if err != nil {
		return nil, err
	}
	if !validScratchCells(cells, scratchMaxReveals) {
		return nil, httpErr{http.StatusBadRequest, "刮刮乐进度异常，请重开"}
	}
	return cells, nil
}

func parseScratchMineCells(s string) ([]int, error) {
	cells, err := parseScratchIntArray(s)
	if err != nil {
		return nil, err
	}
	if len(cells) != scratchMines || !validScratchCells(cells, scratchMines) {
		return nil, httpErr{http.StatusBadRequest, "刮刮乐进度异常，请重开"}
	}
	return cells, nil
}

func validScratchCells(cells []int, maxCount int) bool {
	if len(cells) > maxCount {
		return false
	}
	seen := map[int]bool{}
	for _, cell := range cells {
		if cell < 0 || cell > 8 || seen[cell] {
			return false
		}
		seen[cell] = true
	}
	return true
}

func intContains(a []int, v int) bool {
	for _, x := range a {
		if x == v {
			return true
		}
	}
	return false
}
