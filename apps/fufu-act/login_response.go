package activityapp

import (
	"net/http"

	"fufu/activity"
)

func respondCard(w http.ResponseWriter, card Card) {
	hist, err := loadSpinHistory(card.CardKey)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "服务器错误")
		return
	}
	cfg := SnapshotRuntimeConfig()
	isScratch := cfg.IsScratchTier(card.Dollars)
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
	writeJSON(w, http.StatusOK, buildLoginCardResponse(card, hist, sg, cfg))
}

func buildLoginCardResponse(card Card, history []map[string]any, scratchGame any, cfg activity.Config) map[string]any {
	cfg = activity.CloneConfig(cfg)
	return map[string]any{
		"cardKey":        card.CardKey,
		"cardName":       card.CardName,
		"dollars":        card.Dollars,
		"totalSpins":     card.TotalSpins,
		"usedSpins":      card.UsedSpins,
		"remainingSpins": card.TotalSpins - card.UsedSpins,
		"totalWon":       card.TotalWon,
		"wonJackpot":     card.WonJackpot != 0,
		"history":        history,
		"isScratch":      cfg.IsScratchTier(card.Dollars),
		"scratchGame":    scratchGame,
	}
}
