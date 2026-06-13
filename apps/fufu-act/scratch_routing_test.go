package activityapp

import (
	"testing"

	"fufu/activity"
)

func TestIsScratchDollarTierFollowsRuntimeConfig(t *testing.T) {
	old := SnapshotRuntimeConfig()
	t.Cleanup(func() { SetRuntimeConfig(old) })

	// Configured: 月卡档 (100/300) play scratch, 特惠 (55) does not.
	SetRuntimeConfig(activity.Config{ScratchTiers: []int{100, 300}})
	if !isScratchDollarTier(100) || !isScratchDollarTier(300) {
		t.Fatal("configured scratch tiers should route to scratch")
	}
	if isScratchDollarTier(55) {
		t.Fatal("55 should not be scratch when not configured")
	}

	// Default (nil) → {55}.
	SetRuntimeConfig(activity.Config{})
	if !isScratchDollarTier(55) || isScratchDollarTier(100) {
		t.Fatal("default scratch tier should be 55")
	}

	// Explicit empty → no scratch (everything plays the slot machine).
	SetRuntimeConfig(activity.Config{ScratchTiers: []int{}})
	if isScratchDollarTier(55) {
		t.Fatal("empty scratch tiers should disable scratch")
	}
}
