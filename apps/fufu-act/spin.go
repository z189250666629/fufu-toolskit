package main

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fufu/activity"
	"net/http"
	"strconv"
)

func handleSpin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		CardKey string `json:"cardKey"`
	}
	key, ok, err := readCardKeyRequest(r, &body, func() string { return body.CardKey })
	if err != nil {
		writeMalformedCardKeyRequest(w)
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
		remaining := card.TotalSpins - card.UsedSpins
		if remaining <= 0 {
			return nil, httpErr{403, "抽奖次数已用完"}
		}
		if !isSpinDollarTier(card.Dollars) {
			return nil, httpErr{403, "此卡密额度不参与活动"}
		}
		maxWon := 0
		if err := db.QueryRow(`SELECT COALESCE(MAX(prize_dollars),0) FROM spin_log WHERE card_key=? AND is_retry=0`, key).Scan(&maxWon); err != nil {
			return nil, err
		}
		force := 0
		if card.Rigged.Valid {
			var m map[string]int
			if json.Unmarshal([]byte(card.Rigged.String), &m) == nil {
				force = m[strconv.Itoa(card.UsedSpins+1)]
			}
		}
		sr := spin(card.Dollars, card.WonJackpot != 0, card.UsedSpins, card.TotalSpins, maxWon, force)
		if sr.Type == "retry" {
			if _, err := db.Exec(`INSERT INTO spin_log (card_key,prize_dollars,is_retry) VALUES (?,0,1)`, key); err != nil {
				return nil, err
			}
			return map[string]any{"isRetry": true, "isMiss": false, "message": "再来一次！", "remainingSpins": remaining}, nil
		}
		tx, err := db.Begin()
		if err != nil {
			return nil, err
		}
		committed := false
		defer func() {
			if !committed {
				_ = tx.Rollback()
			}
		}()
		if sr.Type == "miss" {
			if _, err := tx.Exec(`UPDATE cards SET used_spins=used_spins+1,last_spin_at=datetime('now') WHERE card_key=?`, key); err != nil {
				return nil, err
			}
			if _, err := tx.Exec(`INSERT INTO spin_log (card_key,prize_dollars,is_retry) VALUES (?,0,0)`, key); err != nil {
				return nil, err
			}
		} else {
			jack := 0
			if sr.Dollars == 1000 {
				jack = 1
			}
			if _, err := tx.Exec(`UPDATE cards SET used_spins=used_spins+1, won_jackpot=won_jackpot+?, total_won=total_won+?, last_spin_at=datetime('now') WHERE card_key=?`, jack, sr.Dollars, key); err != nil {
				return nil, err
			}
			if _, err := tx.Exec(`INSERT INTO spin_log (card_key,prize_dollars,is_retry) VALUES (?,?,0)`, key, sr.Dollars); err != nil {
				return nil, err
			}
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		committed = true
		updated, ok, lookupErr := lookupCard(key)
		if lookupErr != nil {
			return nil, lookupErr
		}
		if !ok {
			return nil, httpErr{500, "服务器错误"}
		}
		newRem := updated.TotalSpins - updated.UsedSpins
		if newRem <= 0 && updated.TotalWon > 0 {
			enqueueCredit(key, updated.TotalWon)
		}
		if sr.Type == "miss" {
			return map[string]any{"isRetry": false, "isMiss": true, "prize": 0, "remainingSpins": newRem, "totalWon": updated.TotalWon}, nil
		}
		return map[string]any{"isRetry": false, "prize": sr.Dollars, "remainingSpins": newRem, "totalWon": updated.TotalWon, "wonJackpot": updated.WonJackpot != 0}, nil
	})
	if err != nil {
		writeHTTPError(w, err)
		return
	}
	writeJSON(w, 200, res)
}

func spin(dollars float64, hasJackpot bool, used, total, maxWon, force int) spinResult {
	got := activity.Spin(dollars, hasJackpot, used, total, maxWon, force, secureRandomInt)
	return spinResult{got.Type, got.Dollars}
}

func isSpinDollarTier(dollars float64) bool {
	return spinMap[dollars] > 0
}

func secureRandomInt(max int) int {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return int(binary.BigEndian.Uint32(b[:]) % uint32(max))
}
