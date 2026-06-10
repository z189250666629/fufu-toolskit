package main

import (
	"fufu/activity"
	"fufu/newapi"
	"math"
	"net/http"
	"strconv"
	"strings"
)

func handleLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		CardKey string `json:"cardKey"`
	}
	key, ok := readCardKeyRequest(r, &body, func() string { return body.CardKey })
	if !ok {
		writeMissingCardKey(w)
		return
	}
	card, ok := getCard(key)
	if !ok {
		if tokenSvc == nil {
			writeJSONError(w, 503, "NewAPI 未配置: "+errString(tokenConfigErr))
			return
		}
		t, err := tokenSvc.SearchTokenByKey(r.Context(), key)
		if err != nil {
			writeJSONError(w, 500, err.Error())
			return
		}
		if t == nil {
			writeJSONError(w, 404, "卡密不存在")
			return
		}
		isActTest := strings.Contains(t.Name, "-act-") && strings.Contains(t.Name, "test")
		dollars := 0.0
		source := "shop"
		purchaseTime := ""
		createdInRange := false
		if isActTest {
			parts := strings.Split(t.Name, "-act-")
			dollars, _ = strconv.ParseFloat(parts[0], 64)
			source = "act"
			createdInRange = true
		} else {
			createdInRange = t.CreatedTime >= actStartTS && t.CreatedTime <= actEndTS
			shop := findShopPurchase(key)
			purchased := shop != "" && shop >= actStart && shop <= actEnd
			if !createdInRange && !purchased {
				writeJSONError(w, 403, "此卡密不在活动期间内，不参与活动")
				return
			}
			dollars = dollarsTier(t.IntervalQuota)
			purchaseTime = shop
		}
		isScratch := int(math.Round(dollars)) == 55 && (purchaseTime != "" || createdInRange)
		if dollars == 0 || (spinMap[dollars] == 0 && !isScratch) {
			writeJSONError(w, 403, "此卡密额度不参与活动")
			return
		}
		total := spinMap[dollars]
		_, err = db.Exec(`INSERT INTO cards (card_key,card_name,dollars,total_spins,source,purchase_time) VALUES (?,?,?,?,?,?)`, key, t.Name, dollars, total, source, nullString(purchaseTime))
		if err != nil {
			writeJSONError(w, 500, err.Error())
			return
		}
		card, _ = getCard(key)
	}
	respondCard(w, card)
}

func respondCard(w http.ResponseWriter, card Card) {
	hist := []map[string]any{}
	rows, _ := db.Query(`SELECT prize_dollars, created_at FROM spin_log WHERE card_key=? AND is_retry=0 AND prize_dollars>0 ORDER BY id DESC`, card.CardKey)
	for rows.Next() {
		var p int
		var at string
		_ = rows.Scan(&p, &at)
		hist = append(hist, map[string]any{"prize_dollars": p, "created_at": at})
	}
	rows.Close()
	isScratch := int(math.Round(card.Dollars)) == 55
	var sg any
	if isScratch {
		if g, ok := getScratch(card.CardKey); ok {
			gameOver := g.Status == "won" || g.Status == "lost" || g.Status == "cashout"
			m := map[string]any{"revealed": jsonArr(g.Revealed), "prize": g.PrizeDollars, "status": g.Status}
			if gameOver {
				m["mines"] = jsonArr(g.MinePos)
			}
			sg = m
		}
	}
	writeJSON(w, 200, map[string]any{"cardKey": card.CardKey, "cardName": card.CardName, "dollars": card.Dollars, "totalSpins": card.TotalSpins, "usedSpins": card.UsedSpins, "remainingSpins": card.TotalSpins - card.UsedSpins, "totalWon": card.TotalWon, "wonJackpot": card.WonJackpot != 0, "history": hist, "isScratch": isScratch, "scratchGame": sg})
}

func getCard(key string) (Card, bool) {
	var c Card
	err := db.QueryRow(`SELECT card_key,card_name,dollars,total_spins,used_spins,won_jackpot,total_won,source,purchase_time,rigged FROM cards WHERE card_key=?`, key).Scan(&c.CardKey, &c.CardName, &c.Dollars, &c.TotalSpins, &c.UsedSpins, &c.WonJackpot, &c.TotalWon, &c.Source, &c.PurchaseTime, &c.Rigged)
	return c, err == nil
}

func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func dollarsTier(q int64) float64 {
	unit := int64(newapi.DefaultQuotaUnit)
	if tokenSvc != nil && tokenSvc.QuotaUnit > 0 {
		unit = tokenSvc.QuotaUnit
	}
	return activity.DollarsTier(q, unit)
}
