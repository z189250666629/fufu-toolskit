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
	cfg.GameRoutes = []activity.GameRoute{{Dollars: 42, Game: activity.GameSlot}}
	cfg.GameConfigs = []activity.GameConfig{{Game: activity.GameSlot, TargetExpectedValue: 9, ActualExpectedValue: 9}}
	cfg.PrizePool = []activity.Prize{{Type: "win", Dollars: 9, Weight: 1, Rank: "jackpot", Label: "大奖", Advertised: true}}

	SetRuntimeConfig(cfg)

	if !isSpinDollarTier(42) {
		t.Fatalf("custom spin tier should be active")
	}
	if got := spin(42, false, 0, 3, 0, 0); got.Type != "win" || got.Dollars != 9 || got.Rank != "jackpot" || got.Label != "大奖" || !got.Advertised {
		t.Fatalf("spin should use runtime prize pool, got %#v", got)
	}
	window := activityWindow()
	if window.StartText != cfg.StartText || window.EndText != cfg.EndText || window.StartTS != cfg.StartTS || window.EndTS != cfg.EndTS {
		t.Fatalf("activity window = %#v, want %#v", window, cfg)
	}
}
