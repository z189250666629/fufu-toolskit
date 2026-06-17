package activityapp

import (
	"testing"

	"fufu/activity"
)

func TestSpinCoreConfigUsesCardTotalSpinsForBalancing(t *testing.T) {
	cfg := activity.DefaultConfig()
	cfg.GameConfigs = []activity.GameConfig{{Game: activity.GameSlot, TargetExpectedValue: 25, ActualExpectedValue: 20}}

	got := activity.SpinCoreConfigForPoolBalance(cfg, activity.GameSlot, 10, 0)

	if got.DrawCount != 10 {
		t.Fatalf("spin core draw count=%d, want current card total spins", got.DrawCount)
	}
	if got.TargetExpectedValue != 25 || got.ActualExpectedValue != 20 {
		t.Fatalf("spin expected value config=%#v", got)
	}
}
