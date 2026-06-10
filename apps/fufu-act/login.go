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
	key, ok, err := readCardKeyRequest(r, &body, func() string { return body.CardKey })
	if err != nil {
		writeMalformedCardKeyRequest(w)
		return
	}
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
		if t.Status != 1 {
			writeJSONError(w, 403, "此卡密已被禁用，无法参与活动")
			return
		}
		dollars := 0.0
		source := "shop"
		purchaseTime := ""
		createdInRange := false
		actTestDollars, isActTest := parseActTestTokenName(t.Name)
		if isActTest {
			dollars = actTestDollars
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
	hist, err := loadSpinHistory(card.CardKey)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	isScratch := int(math.Round(card.Dollars)) == 55
	var sg any
	if isScratch {
		if g, ok := getScratch(card.CardKey); ok {
			sg = scratchGameResponse(g)
		}
	}
	writeJSON(w, 200, map[string]any{"cardKey": card.CardKey, "cardName": card.CardName, "dollars": card.Dollars, "totalSpins": card.TotalSpins, "usedSpins": card.UsedSpins, "remainingSpins": card.TotalSpins - card.UsedSpins, "totalWon": card.TotalWon, "wonJackpot": card.WonJackpot != 0, "history": hist, "isScratch": isScratch, "scratchGame": sg})
}

func loadSpinHistory(cardKey string) ([]map[string]any, error) {
	hist := []map[string]any{}
	rows, err := db.Query(`SELECT prize_dollars, created_at FROM spin_log WHERE card_key=? AND is_retry=0 AND prize_dollars>0 ORDER BY id DESC`, cardKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var p int
		var at string
		if err := rows.Scan(&p, &at); err != nil {
			return nil, err
		}
		hist = append(hist, map[string]any{"prize_dollars": p, "created_at": at})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return hist, nil
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

func parseActTestTokenName(name string) (float64, bool) {
	prefix, suffix, ok := strings.Cut(strings.TrimSpace(name), "-act-")
	if !ok {
		return 0, false
	}
	dollars, err := strconv.ParseFloat(strings.TrimSpace(prefix), 64)
	if err != nil || dollars <= 0 {
		return 0, false
	}
	for _, part := range strings.FieldsFunc(suffix, func(r rune) bool {
		return r == '-' || r == '_' || r == ' ' || r == '.'
	}) {
		if isTestCardSegment(part) {
			return dollars, true
		}
	}
	return 0, false
}

func isTestCardName(name string) bool {
	for _, part := range strings.FieldsFunc(strings.TrimSpace(name), func(r rune) bool {
		return r == '-' || r == '_' || r == ' ' || r == '.'
	}) {
		if isTestCardSegment(part) {
			return true
		}
	}
	return false
}

func isTestCardSegment(value string) bool {
	return strings.EqualFold(strings.TrimSpace(value), "test")
}

func dollarsTier(q int64) float64 {
	unit := int64(newapi.DefaultQuotaUnit)
	if tokenSvc != nil && tokenSvc.QuotaUnit > 0 {
		unit = tokenSvc.QuotaUnit
	}
	return activity.DollarsTier(q, unit)
}
