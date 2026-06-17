package activityapp

import (
	"net/http"

	"fufu/activity"
	"fufu/dragonboatcore"
)

func handleDragonBoatStart(w http.ResponseWriter, r *http.Request) {
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
		card, err := requireDragonBoatEligibleCard(r.Context(), key)
		if err != nil {
			return nil, err
		}
		game, ok, err := lookupDragonBoat(key)
		if err != nil {
			return nil, err
		}
		if !ok {
			if err := insertDragonBoatGame(key); err != nil {
				return nil, err
			}
			game, ok, err = lookupDragonBoat(key)
			if err != nil {
				return nil, err
			}
			if !ok {
				return nil, httpErr{500, "服务器错误"}
			}
		}
		return dragonBoatAppResponseWithPeels(game, card)
	})
	if err != nil {
		writeHTTPError(w, err)
		return
	}
	writeJSON(w, 200, res)
}

func handleDragonBoatFish(w http.ResponseWriter, r *http.Request) {
	var b struct {
		CardKey string                `json:"cardKey"`
		Target  *dragonboatcore.Point `json:"target"`
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
		card, err := requireDragonBoatEligibleCard(r.Context(), key)
		if err != nil {
			return nil, err
		}
		game, ok, err := lookupDragonBoat(key)
		if err != nil {
			return nil, err
		}
		if !ok {
			if err := insertDragonBoatGame(key); err != nil {
				return nil, err
			}
			game, ok, err = lookupDragonBoat(key)
			if err != nil {
				return nil, err
			}
			if !ok {
				return nil, httpErr{500, "服务器错误"}
			}
		}
		if !dragonboatcore.CanFish(card.TotalSpins, dragonBoatRound(game)) {
			return nil, httpErr{403, "捕捞次数已用完"}
		}
		scene := dragonBoatSceneSeed(game, card)
		target := dragonboatcore.DefaultCastTarget(scene)
		if b.Target != nil {
			target = *b.Target
		}
		cast := dragonboatcore.ResolveCast(scene, target)
		result := dragonboatcore.Fish(card.TotalSpins, dragonBoatRound(game), cast)
		removedObjects := dragonBoatRemovedObjectsAfter(game.RemovedObjects, cast)
		if err := updateDragonBoatFishing(key, result.FishingUsed, result.ZongziCaught, result.Status, removedObjects); err != nil {
			return nil, err
		}
		next, ok, err := lookupDragonBoat(key)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, httpErr{500, "服务器错误"}
		}
		out, err := dragonBoatAppResponseWithPeels(next, card)
		if err != nil {
			return nil, err
		}
		out["hit"] = result.Hit
		out["cast"] = cast
		return out, nil
	})
	if err != nil {
		writeHTTPError(w, err)
		return
	}
	writeJSON(w, 200, res)
}

func handleDragonBoatPeel(w http.ResponseWriter, r *http.Request) {
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
		card, err := requireDragonBoatEligibleCard(r.Context(), key)
		if err != nil {
			return nil, err
		}
		game, ok, err := lookupDragonBoat(key)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, httpErr{404, "请先开始端午捕粽"}
		}
		if !dragonboatcore.CanPeel(dragonBoatRound(game)) {
			return nil, httpErr{403, "暂无可剥开的粽子"}
		}
		cfg := dragonBoatCoreConfig()
		cfg.PrizePool = dragonBoatPrizePool()
		result, ok := dragonboatcore.Peel(cfg, card.TotalSpins, dragonBoatRound(game), secureRandomInt)
		if !ok {
			return nil, httpErr{403, "暂无可剥开的粽子"}
		}
		prize := spinAppResult(activity.SpinResult{
			Type:       result.Prize.Type,
			Dollars:    result.Prize.Dollars,
			Rank:       result.Prize.Rank,
			Label:      result.Prize.Label,
			Advertised: result.Prize.Advertised,
		})
		if prize.Type == "" {
			prize.Type = "miss"
		}
		if err := finishDragonBoatPeel(key, card, prize, result.FishingUsed, result.ZongziCaught, result.ZongziPeeled, result.Status); err != nil {
			return nil, err
		}
		updated, ok, err := lookupCard(key)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, httpErr{500, "服务器错误"}
		}
		next, ok, err := lookupDragonBoat(key)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, httpErr{500, "服务器错误"}
		}
		out, err := dragonBoatAppResponseWithPeels(next, updated)
		if err != nil {
			return nil, err
		}
		out["prizeResult"] = dragonBoatPrizeMap(prize)
		out["totalWon"] = updated.TotalWon
		return out, nil
	})
	if err != nil {
		writeHTTPError(w, err)
		return
	}
	writeJSON(w, 200, res)
}

func dragonBoatPrizePool() []activity.Prize {
	cfg := SnapshotRuntimeConfig()
	if cfg.DynamicPrizePool.Enabled {
		if balance, err := currentPrizePoolBalance(); err == nil {
			return activity.SpinCoreConfigForPoolBalance(cfg, activity.GameDragon, 1, balance).PrizePool
		}
	}
	return activity.BalancedPrizePoolForGame(cfg, activity.GameDragon).Pool
}
