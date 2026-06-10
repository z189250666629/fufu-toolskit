package main

import (
	"context"
	"fufu/config"
	"sort"
	"time"
)

func getModelStatus(ctx context.Context, force bool) *ModelStatus {
	ctx = contextOrBackground(ctx)
	cacheKey := modelStatusCacheKey(rootDir)
	for {
		now := time.Now()
		modelCache.Lock()
		if !force && modelCache.Value != nil && modelCache.Key == cacheKey && now.Before(modelCache.Expires) {
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

		status := buildModelStatus(ctx)

		modelCache.Lock()
		call.status = status
		delete(modelCache.Inflight, cacheKey)
		if ctx.Err() == nil {
			modelCache.Value = status
			modelCache.Expires = time.Now().Add(modelStatusCacheTTL)
			modelCache.Key = cacheKey
		}
		close(call.done)
		modelCache.Unlock()
		return status
	}
}

func buildModelStatus(ctx context.Context) *ModelStatus {
	ctx = contextOrBackground(ctx)
	sites, msg := config.LoadManagedSites(rootDir)
	publicMsg := publicManagedSiteConfigError(msg)
	now := time.Now().Unix()
	pruneManualTestCache(now)
	status := &ModelStatus{Configured: len(sites) > 0, ConfigError: publicMsg, GeneratedAt: now, ExpiresAt: now + modelStatusWindowSeconds, WindowSeconds: modelStatusWindowSeconds, RefreshEverySeconds: modelStatusWindowSeconds, Totals: map[string]int{"siteCount": len(sites), "modelCount": 0, "requestCount": 0, "successCount": 0, "failureCount": 0, "operational": 0, "degraded": 0, "down": 0, "unknown": 0}}
	modelRows := map[string]*ModelRow{}
	start := now - modelStatusWindowSeconds
	for _, site := range sites {
		successLogs, logErr := loadSiteLogs(ctx, site, logTypeConsume, start, now)
		errorLogs, errErr := loadSiteLogs(ctx, site, logTypeError, start, now)
		if logErr == "" {
			logErr = errErr
		}
		channels, chErr := loadSiteChannels(ctx, site)
		pricing, priceErr := loadPricing(ctx, site)
		channelIndex := indexChannelsForModelStatus(channels)
		groups := channelIndex.groups
		ss := SiteStatus{Site: site.Public(), Groups: groups, LogError: logErr, ChannelsError: chErr, PricingError: priceErr}
		ss.SuccessCount = len(successLogs)
		ss.FailureCount = len(errorLogs)
		ss.RequestCount = ss.SuccessCount + ss.FailureCount
		ss.SuccessRate = rate(ss.SuccessCount, ss.FailureCount)
		ss.LastSeenAt = maxLogTime(append(successLogs, errorLogs...))
		ss.Status = siteStatusFromCounts(ss.SuccessCount, ss.FailureCount)
		status.Sites = append(status.Sites, ss)
		status.Totals["requestCount"] += ss.RequestCount
		status.Totals["successCount"] += ss.SuccessCount
		status.Totals["failureCount"] += ss.FailureCount
		successByModel := groupLogs(successLogs)
		errorByModel := groupLogs(errorLogs)
		successByMG := groupLogsByModelGroup(successLogs)
		errorByMG := groupLogsByModelGroup(errorLogs)
		for model, chans := range channelIndex.channelsByModel {
			row := modelRows[model]
			if row == nil {
				row = &ModelRow{Model: model, PerSite: map[string]*ModelCell{}}
				modelRows[model] = row
			}
			cell := buildCell(site.Name, model, chans, successByModel[model], errorByModel[model], pricing[model])
			cell.GroupStats = map[string]*ModelCell{}
			for _, g := range groups {
				groupChans := []Channel{}
				for _, ch := range chans {
					if contains(ch.Groups, g) {
						groupChans = append(groupChans, ch)
					}
				}
				if len(groupChans) == 0 {
					continue
				}
				logKey := modelGroupLogKey(model, g)
				gs := buildCell(site.Name, model, groupChans, successByMG[logKey], errorByMG[logKey], pricing[model])
				gs.Groups = []string{g}
				if rec, ok := testResults.Load(modelManualKey(site.Name, model, g)); ok {
					gs.ManualTest = rec
				}
				if until, ok := testCooldowns.Load(modelManualKey(site.Name, model, g)); ok {
					gs.NextTestAllowedAt = until.(int64)
				}
				cell.GroupStats[g] = gs
			}
			if rec, ok := testResults.Load(modelManualKey(site.Name, model, "")); ok {
				cell.ManualTest = rec
			}
			if until, ok := testCooldowns.Load(modelManualKey(site.Name, model, "")); ok {
				cell.NextTestAllowedAt = until.(int64)
			}
			row.PerSite[site.Name] = cell
		}
	}
	rows := []ModelRow{}
	for _, row := range modelRows {
		recomputeModelRowSummary(row)
		rows = append(rows, *row)
		status.Totals[row.Status]++
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Model < rows[j].Model })
	status.Models = rows
	status.Totals["modelCount"] = len(rows)
	return status
}

func buildCell(siteName, model string, chans []Channel, success, errorRows []LogRow, pricing Pricing) *ModelCell {
	enabled := 0
	groups := map[string]bool{}
	for _, ch := range chans {
		if ch.Status == channelStatusEnabled {
			enabled++
		}
		for _, g := range ch.Groups {
			groups[g] = true
		}
	}
	cell := &ModelCell{Configured: true, SiteName: siteName, Model: model, RequestCount: len(success) + len(errorRows), SuccessCount: len(success), FailureCount: len(errorRows), SuccessRate: rate(len(success), len(errorRows)), LastSuccessAt: maxLogTime(success), LastFailureAt: maxLogTime(errorRows), TotalChannelCount: len(chans), EnabledChannelCount: enabled, Groups: keys(groups)}
	cell.LastSeenAt = maxInt64(cell.LastSuccessAt, cell.LastFailureAt)
	cell.Status = statusFromCounts(cell.SuccessCount, cell.FailureCount)
	if pricing.Input != 0 || pricing.Output != 0 {
		p := pricing
		cell.Pricing = &p
	}
	return cell
}
