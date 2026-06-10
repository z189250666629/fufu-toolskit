package main

import (
	"encoding/json"
	"math"
	"net/http"
	"strings"
)

func handleScratchStart(w http.ResponseWriter, r *http.Request) {
	var b struct {
		CardKey string `json:"cardKey"`
	}
	key, ok := readCardKeyRequest(r, &b, func() string { return b.CardKey })
	if !ok {
		writeMissingCardKey(w)
		return
	}
	card, ok := getCard(key)
	if !ok {
		writeJSONError(w, 404, "请先登录")
		return
	}
	if int(math.Round(card.Dollars)) != 55 {
		writeJSONError(w, 403, "此卡密不参与刮刮乐活动")
		return
	}
	if g, ok := getScratch(key); ok {
		writeJSON(w, 200, map[string]any{"cells": 9, "revealed": jsonArr(g.Revealed), "prize": g.PrizeDollars, "status": g.Status})
		return
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
	_, _ = db.Exec(`INSERT INTO scratch_games (card_key,mine_pos) VALUES (?,?)`, key, string(mb))
	writeJSON(w, 200, map[string]any{"cells": 9, "revealed": []int{}, "prize": 0, "status": "playing"})
}

func handleScratchReveal(w http.ResponseWriter, r *http.Request) {
	var b struct {
		CardKey   string `json:"cardKey"`
		CellIndex int    `json:"cellIndex"`
	}
	key, ok := readCardKeyRequest(r, &b, func() string { return b.CardKey })
	if !ok {
		writeMissingCardKey(w)
		return
	}
	if b.CellIndex < 0 || b.CellIndex > 8 {
		writeJSONError(w, 400, "无效的格子")
		return
	}
	res, err := withCardLock(key, func() (any, error) {
		g, ok := getScratch(key)
		if !ok {
			return nil, httpErr{404, "请先开始刮刮乐"}
		}
		if g.Status != "playing" {
			return nil, httpErr{403, "游戏已结束"}
		}
		revealed := jsonArr(g.Revealed)
		for _, v := range revealed {
			if v == b.CellIndex {
				return nil, httpErr{400, "此格已刮开"}
			}
		}
		mines := jsonArr(g.MinePos)
		revealed = append(revealed, b.CellIndex)
		if intContains(mines, b.CellIndex) {
			rb, _ := json.Marshal(revealed)
			_, _ = db.Exec(`UPDATE scratch_games SET revealed=?, prize_dollars=0, status='lost' WHERE card_key=?`, string(rb), key)
			return map[string]any{"hit": true, "mines": mines, "prize": 0, "status": "lost", "revealed": revealed}, nil
		}
		safe := 0
		for _, v := range revealed {
			if !intContains(mines, v) {
				safe++
			}
		}
		prize := scratchRewards[safe-1]
		status := "playing"
		if safe >= scratchMaxReveals {
			status = "won"
		}
		rb, _ := json.Marshal(revealed)
		_, _ = db.Exec(`UPDATE scratch_games SET revealed=?, prize_dollars=?, status=? WHERE card_key=?`, string(rb), prize, status, key)
		if status == "won" && prize > 0 {
			enqueueCredit(key, prize)
		}
		return map[string]any{"hit": false, "prize": prize, "status": status, "revealed": revealed}, nil
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
	key, ok := readCardKeyRequest(r, &b, func() string { return b.CardKey })
	if !ok {
		writeMissingCardKey(w)
		return
	}
	res, err := withCardLock(key, func() (any, error) {
		g, ok := getScratch(key)
		if !ok {
			return nil, httpErr{404, "请先开始刮刮乐"}
		}
		if g.Status != "playing" {
			return nil, httpErr{403, "游戏已结束"}
		}
		revealed := jsonArr(g.Revealed)
		mines := jsonArr(g.MinePos)
		safe := 0
		for _, v := range revealed {
			if !intContains(mines, v) {
				safe++
			}
		}
		if safe == 0 {
			return nil, httpErr{400, "至少刮开一个安全格才能结算"}
		}
		prize := scratchRewards[safe-1]
		_, _ = db.Exec(`UPDATE scratch_games SET prize_dollars=?, status='cashout' WHERE card_key=?`, prize, key)
		if prize > 0 {
			enqueueCredit(key, prize)
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
	key, ok := readCardKeyRequest(r, &b, func() string { return b.CardKey })
	if !ok {
		writeMissingCardKey(w)
		return
	}
	card, ok := getCard(key)
	if !ok {
		writeJSONError(w, 404, "请先登录")
		return
	}
	if !strings.Contains(card.CardName, "test") {
		writeJSONError(w, 403, "仅测试卡可重开")
		return
	}
	if g, ok := getScratch(key); ok && g.Status == "playing" {
		writeJSONError(w, 400, "当前游戏尚未结束")
		return
	}
	_, _ = db.Exec(`DELETE FROM scratch_games WHERE card_key=?`, key)
	writeJSON(w, 200, map[string]any{"ok": true})
}

func getScratch(key string) (ScratchGame, bool) {
	var g ScratchGame
	err := db.QueryRow(`SELECT id,card_key,mine_pos,revealed,prize_dollars,status FROM scratch_games WHERE card_key=?`, key).Scan(&g.ID, &g.CardKey, &g.MinePos, &g.Revealed, &g.PrizeDollars, &g.Status)
	return g, err == nil
}

func jsonArr(s string) []int {
	var a []int
	_ = json.Unmarshal([]byte(s), &a)
	if a == nil {
		return []int{}
	}
	return a
}

func intContains(a []int, v int) bool {
	for _, x := range a {
		if x == v {
			return true
		}
	}
	return false
}
