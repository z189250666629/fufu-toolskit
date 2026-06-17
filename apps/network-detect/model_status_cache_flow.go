package main

import (
	"context"
	"time"
)

var buildModelStatusForCache = buildModelStatus

func getModelStatus(ctx context.Context, force bool) *ModelStatus {
	ctx = contextOrBackground(ctx)
	cacheKey := modelStatusCacheKey(rootDir)
	for {
		now := time.Now()
		modelCache.Lock()
		cacheFresh := modelCache.Value != nil && modelCache.Key == cacheKey && now.Before(modelCache.Expires)
		if cacheFresh && (!force || now.Before(modelCache.ForceRefreshAfter)) {
			v := modelCache.Value
			modelCache.Unlock()
			return v
		}
		if modelCache.Inflight != nil {
			if call := modelCache.Inflight[cacheKey]; call != nil {
				done := call.done
				modelCache.Unlock()
				select {
				case <-done:
					if call.status != nil {
						return call.status
					}
					force = false
					continue
				case <-ctx.Done():
					return buildModelStatus(ctx)
				}
			}
		}
		if modelCache.Inflight == nil {
			modelCache.Inflight = map[string]*modelStatusBuildCall{}
		}
		call := &modelStatusBuildCall{done: make(chan struct{})}
		modelCache.Inflight[cacheKey] = call
		modelCache.Unlock()

		var status *ModelStatus
		defer func() {
			modelCache.Lock()
			call.status = status
			delete(modelCache.Inflight, cacheKey)
			if status != nil && ctx.Err() == nil {
				cachedAt := time.Now()
				modelCache.Value = status
				modelCache.Expires = cachedAt.Add(modelStatusCacheTTL)
				modelCache.Key = cacheKey
				modelCache.ForceRefreshAfter = cachedAt.Add(modelStatusForceRefreshCooldown)
			}
			close(call.done)
			modelCache.Unlock()
		}()
		status = buildModelStatusForCache(ctx)
		return status
	}
}
