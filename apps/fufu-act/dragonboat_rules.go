package activityapp

import (
	"encoding/json"

	"fufu/activity"
	"fufu/dragonboatcore"
)

func isDragonBoatDollarTier(dollars float64) bool {
	cfg := SnapshotRuntimeConfig()
	return cfg.GameForTier(dollars) == activity.GameDragon && cfg.DrawCountForTier(dollars) > 0
}

func dragonBoatRound(g DragonBoatGame) dragonboatcore.Round {
	return dragonboatcore.Round{
		CardKey:      g.CardKey,
		FishingUsed:  g.FishingUsed,
		ZongziCaught: g.ZongziCaught,
		ZongziPeeled: g.ZongziPeeled,
		Status:       g.Status,
	}
}

func dragonBoatCoreConfig() dragonboatcore.Config {
	return dragonboatcore.Config{
		PrizePool: SnapshotRuntimeConfig().PrizePool,
	}
}

func dragonBoatSceneSeed(game DragonBoatGame, card Card) dragonboatcore.SceneSeed {
	return dragonboatcore.CalculateSceneSeed(dragonboatcore.SceneSeedInput{
		CardKey:          card.CardKey,
		TotalDraws:       card.TotalSpins,
		FishingUsed:      game.FishingUsed,
		ZongziCaught:     game.ZongziCaught,
		ZongziPeeled:     game.ZongziPeeled,
		RemovedObjectIDs: dragonBoatRemovedObjectIDs(game.RemovedObjects),
	})
}

func dragonBoatAppResponse(game DragonBoatGame, card Card) map[string]any {
	remainingFish := card.TotalSpins - game.FishingUsed
	if remainingFish < 0 {
		remainingFish = 0
	}
	zongziReady := game.ZongziCaught - game.ZongziPeeled
	if zongziReady < 0 {
		zongziReady = 0
	}
	return map[string]any{
		"fishingTotal":  card.TotalSpins,
		"status":        game.Status,
		"fishingUsed":   game.FishingUsed,
		"remainingFish": remainingFish,
		"zongziCaught":  game.ZongziCaught,
		"zongziPeeled":  game.ZongziPeeled,
		"zongziReady":   zongziReady,
		"totalWon":      card.TotalWon,
		"sceneSeed":     dragonBoatSceneSeed(game, card),
	}
}

// dragonBoatAppResponseWithPeels is the standard dragon-boat response plus the
// per-zongzi reward list (for the result carousel). Loaded from spin_log so it
// persists across reloads.
func dragonBoatAppResponseWithPeels(game DragonBoatGame, card Card) (map[string]any, error) {
	out := dragonBoatAppResponse(game, card)
	peels, err := loadDragonPeels(card.CardKey)
	if err != nil {
		return nil, err
	}
	out["peels"] = peels
	return out, nil
}

func dragonBoatPrizeMap(prize spinResult) map[string]any {
	return map[string]any{
		"type":       prize.Type,
		"prize":      prize.Dollars,
		"rank":       prize.Rank,
		"label":      prize.Label,
		"advertised": prize.Advertised,
		"isMiss":     prize.Type == "miss",
	}
}

func dragonBoatRemovedObjectIDs(raw string) []string {
	var ids []string
	if raw == "" {
		return ids
	}
	if err := json.Unmarshal([]byte(raw), &ids); err != nil {
		return nil
	}
	out := ids[:0]
	seen := map[string]bool{}
	for _, id := range ids {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

func dragonBoatRemovedObjectsAfter(raw string, cast dragonboatcore.CastResult) string {
	ids := dragonBoatRemovedObjectIDs(raw)
	next := ""
	if cast.Hit && cast.ZongziID != "" {
		next = cast.ZongziID
	} else if cast.BlockedBy != "" {
		next = cast.BlockedBy
	}
	if next == "" {
		return dragonBoatRemovedObjectsJSON(ids)
	}
	for _, id := range ids {
		if id == next {
			return dragonBoatRemovedObjectsJSON(ids)
		}
	}
	ids = append(ids, next)
	return dragonBoatRemovedObjectsJSON(ids)
}

func dragonBoatRemovedObjectsJSON(ids []string) string {
	data, err := json.Marshal(ids)
	if err != nil {
		return "[]"
	}
	return string(data)
}
