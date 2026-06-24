package main

func activityFrontendEnabled() bool {
	if unifiedConfig == nil {
		return true
	}
	return unifiedConfig.Snapshot().Activity.IsEnabled()
}
