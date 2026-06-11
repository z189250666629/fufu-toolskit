package activityapp

import (
	"sync"

	"fufu/activity"
)

var activityConfigMu sync.RWMutex
var runtimeActivityConfig = activity.DefaultConfig()

func SetRuntimeConfig(cfg activity.Config) {
	cfg = activity.CloneConfig(cfg)
	activityConfigMu.Lock()
	defer activityConfigMu.Unlock()
	runtimeActivityConfig = cfg
	spinMap = activity.CloneConfig(cfg).SpinMap
	prizePool = activity.CloneConfig(cfg).PrizePool
	scratchRewards = append([]int(nil), cfg.ScratchRewards...)
}

func SnapshotRuntimeConfig() activity.Config {
	activityConfigMu.RLock()
	defer activityConfigMu.RUnlock()
	return activity.CloneConfig(runtimeActivityConfig)
}

func activityWindow() activity.Config {
	return SnapshotRuntimeConfig()
}
