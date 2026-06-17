package main

import (
	"context"
	"time"
)

func buildModelStatus(ctx context.Context) *ModelStatus {
	ctx = contextOrBackground(ctx)
	plan := newModelStatusBuildPlan()
	return projectModelStatus(plan, fetchModelStatusSiteData(ctx, plan), runtimeModelManualProjection)
}

func newModelStatusBuildPlan() modelStatusBuildPlan {
	sites, msg := managedSitesForRuntime()
	now := time.Now().Unix()
	pruneManualTestCache(now)
	return modelStatusBuildPlan{
		Sites:         sites,
		ConfigError:   publicManagedSiteConfigError(msg),
		Now:           now,
		WindowSeconds: modelStatusWindowSeconds,
	}
}
