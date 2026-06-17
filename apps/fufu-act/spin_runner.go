package activityapp

import (
	"encoding/json"
	"net/http"
	"strconv"
)

func runSpinForCard(r *http.Request, key string) (any, error) {
	return withCardLock(key, func() (any, error) {
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
		remaining := card.TotalSpins - card.UsedSpins
		if remaining <= 0 {
			return nil, httpErr{403, "抽奖次数已用完"}
		}
		if !isSpinDollarTier(card.Dollars) {
			return nil, httpErr{403, "此卡密额度不参与活动"}
		}
		maxWon, err := maxSpinPrize(key)
		if err != nil {
			return nil, err
		}
		sr := spin(card.Dollars, card.WonJackpot != 0, card.UsedSpins, card.TotalSpins, maxWon, spinForceForCard(card))
		if sr.Type == "retry" {
			if err := recordSpinRetry(key); err != nil {
				return nil, err
			}
			return map[string]any{"isRetry": true, "isMiss": false, "message": "再来一次！", "remainingSpins": remaining}, nil
		}
		if err := recordSpinResult(key, card, sr, remaining); err != nil {
			return nil, err
		}
		updated, ok, lookupErr := lookupCard(key)
		if lookupErr != nil {
			return nil, lookupErr
		}
		if !ok {
			return nil, httpErr{500, "服务器错误"}
		}
		newRem := updated.TotalSpins - updated.UsedSpins
		if sr.Type == "miss" {
			return map[string]any{"isRetry": false, "isMiss": true, "prize": 0, "remainingSpins": newRem, "totalWon": updated.TotalWon}, nil
		}
		return map[string]any{
			"isRetry":        false,
			"prize":          sr.Dollars,
			"remainingSpins": newRem,
			"totalWon":       updated.TotalWon,
			"wonJackpot":     updated.WonJackpot != 0,
			"rank":           sr.Rank,
			"label":          sr.Label,
			"advertised":     sr.Advertised,
		}, nil
	})
}

func spinForceForCard(card Card) int {
	if !card.Rigged.Valid {
		return 0
	}
	var m map[string]int
	if json.Unmarshal([]byte(card.Rigged.String), &m) != nil {
		return 0
	}
	return m[strconv.Itoa(card.UsedSpins+1)]
}
