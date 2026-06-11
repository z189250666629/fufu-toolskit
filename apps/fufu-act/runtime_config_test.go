package activityapp

import (
	"testing"

	"fufu/activity"
)

func TestSetRuntimeConfigUpdatesSpinOddsAndActivityWindow(t *testing.T) {
	original := SnapshotRuntimeConfig()
	t.Cleanup(func() { SetRuntimeConfig(original) })

	cfg := activity.DefaultConfig()
	cfg.StartText = "2026-06-01 00:00:00"
	cfg.EndText = "2026-06-30 23:59:59"
	cfg.StartTS = 1780243200
	cfg.EndTS = 1782835199
	cfg.SpinMap = map[float64]int{42: 3}
	cfg.TierPools = map[int][]activity.Prize{}
	cfg.PrizePool = []activity.Prize{{Type: "win", Dollars: 9, Weight: 1}}
	cfg.PostJackpotPool = []activity.Prize{{Type: "win", Dollars: 1, Weight: 1}}

	SetRuntimeConfig(cfg)

	if !isSpinDollarTier(42) {
		t.Fatalf("custom spin tier should be active")
	}
	if got := spin(42, false, 0, 3, 0, 0); got.Type != "win" || got.Dollars != 9 {
		t.Fatalf("spin should use runtime prize pool, got %#v", got)
	}
	window := activityWindow()
	if window.StartText != cfg.StartText || window.EndText != cfg.EndText || window.StartTS != cfg.StartTS || window.EndTS != cfg.EndTS {
		t.Fatalf("activity window = %#v, want %#v", window, cfg)
	}
}
