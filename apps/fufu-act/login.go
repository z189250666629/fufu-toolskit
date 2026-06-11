package activityapp

import (
	"database/sql"
	"errors"
	"fufu/activity"
	"fufu/newapi"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func handleLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		CardKey string `json:"cardKey"`
	}
	key, ok, err := readCardKeyRequest(r, &body, func() string { return body.CardKey })
	if err != nil {
		writeCardKeyRequestError(w, err)
		return
	}
	if !ok {
		writeMissingCardKey(w)
		return
	}
	card, ok, lookupErr := lookupCard(key)
	if lookupErr != nil {
		writeJSONError(w, http.StatusInternalServerError, "服务器错误")
		return
	}
	if ok {
		if err := requireCurrentTokenActive(r.Context(), key); err != nil {
			writeHTTPError(w, err)
			return
		}
	} else {
		if tokenSvc == nil {
			writeJSONError(w, 503, "NewAPI 未配置")
			return
		}
		client := loginClientIP(r)
		now := time.Now()
		if blockedUntil, allowed := unknownLoginLimiter.allow(client, key, now); !allowed {
			writeUnknownLoginRateLimited(w, blockedUntil)
			return
		}
		t, err := tokenSvc.SearchTokenByKey(r.Context(), key)
		if err != nil {
			writeJSONError(w, 500, "服务器错误")
			return
		}
		if t == nil {
			unknownLoginLimiter.recordUnknown(client, key, now)
			writeJSONError(w, 404, "卡密不存在")
			return
		}
		unknownLoginLimiter.clear(client, key)
		if t.Status != 1 {
			writeJSONError(w, 403, "此卡密已被禁用，无法参与活动")
			return
		}
		dollars := 0.0
		source := "shop"
		purchaseTime := ""
		createdInRange := false
		actTestDollars, isActTest := parseActTestTokenName(t.Name)
		window := activityWindow()
		if isActTest {
			dollars = actTestDollars
			source = "act"
			createdInRange = true
		} else {
			createdInRange = t.CreatedTime >= window.StartTS && t.CreatedTime <= window.EndTS
			shop, err := findShopPurchase(r.Context(), key)
			if err != nil {
				writeJSONError(w, http.StatusBadGateway, "店铺查询失败，请稍后再试")
				return
			}
			purchased := shop.PurchaseTime != "" && shop.PurchaseTime >= window.StartText && shop.PurchaseTime <= window.EndText
			if !createdInRange && !purchased {
				writeJSONError(w, 403, "此卡密不在活动期间内，不参与活动")
				return
			}
			dollars = dollarsTier(t.IntervalQuota)
			purchaseTime = shop.PurchaseTime
		}
		cfg := SnapshotRuntimeConfig()
		isScratch := isScratchDollarTier(dollars) && (purchaseTime != "" || createdInRange)
		if dollars == 0 || (cfg.SpinMap[dollars] == 0 && !isScratch) {
			writeJSONError(w, 403, "此卡密额度不参与活动")
			return
		}
		total := cfg.SpinMap[dollars]
		_, err = db.Exec(`INSERT INTO cards (card_key,card_name,dollars,total_spins,source,purchase_time) VALUES (?,?,?,?,?,?)`, key, t.Name, dollars, total, source, nullString(purchaseTime))
		if err != nil {
			writeJSONError(w, 500, "服务器错误")
			return
		}
		card, ok, lookupErr = lookupCard(key)
		if lookupErr != nil || !ok {
			writeJSONError(w, http.StatusInternalServerError, "服务器错误")
			return
		}
	}
	respondCard(w, card)
}

func respondCard(w http.ResponseWriter, card Card) {
	hist, err := loadSpinHistory(card.CardKey)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "服务器错误")
		return
	}
	isScratch := isScratchDollarTier(card.Dollars)
	var sg any
	if isScratch {
		g, ok, err := lookupScratch(card.CardKey)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "服务器错误")
			return
		}
		if ok {
			sg, err = scratchGameResponse(g)
			if err != nil {
				writeJSONError(w, http.StatusInternalServerError, "服务器错误")
				return
			}
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
	c, ok, _ := lookupCard(key)
	return c, ok
}

func lookupCard(key string) (Card, bool, error) {
	var c Card
	err := db.QueryRow(`SELECT card_key,card_name,dollars,total_spins,used_spins,won_jackpot,total_won,source,purchase_time,rigged FROM cards WHERE card_key=?`, key).Scan(&c.CardKey, &c.CardName, &c.Dollars, &c.TotalSpins, &c.UsedSpins, &c.WonJackpot, &c.TotalWon, &c.Source, &c.PurchaseTime, &c.Rigged)
	if err == nil {
		return c, true, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return Card{}, false, nil
	}
	return Card{}, false, err
}

func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func isScratchDollarTier(dollars float64) bool {
	return dollars == 55
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
