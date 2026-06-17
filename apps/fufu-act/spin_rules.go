package activityapp

import (
	"crypto/rand"
	"encoding/binary"
	"fufu/activity"
)

func spin(dollars float64, hasJackpot bool, used, total, maxWon, force int) spinResult {
	cfg := SnapshotRuntimeConfig()
	if cfg.DynamicPrizePool.Enabled {
		if balance, err := currentPrizePoolBalance(); err == nil {
			return spinAppResult(activity.SpinWithPoolBalance(cfg, dollars, hasJackpot, used, total, maxWon, force, balance, secureRandomInt))
		}
	}
	return spinAppResult(activity.SpinWithConfig(cfg, dollars, hasJackpot, used, total, maxWon, force, secureRandomInt))
}

func spinAppResult(got activity.SpinResult) spinResult {
	return spinResult{
		Type:       got.Type,
		Dollars:    got.Dollars,
		Rank:       got.Rank,
		Label:      got.Label,
		Advertised: got.Advertised,
	}
}

func isSpinDollarTier(dollars float64) bool {
	cfg := SnapshotRuntimeConfig()
	return cfg.GameForTier(dollars) == activity.GameSlot && cfg.DrawCountForTier(dollars) > 0
}

func secureRandomInt(max int) int {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return int(binary.BigEndian.Uint32(b[:]) % uint32(max))
}
