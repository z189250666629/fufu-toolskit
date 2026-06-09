package main

import (
	"fufu/config"
	"sort"
	"time"
)

func getModelStatus(force bool) *ModelStatus {
	now := time.Now()
	modelCache.Lock()
	if !force && modelCache.Value != nil && now.Before(modelCache.Expires) {
		v := modelCache.Value
		modelCache.Unlock()
		return v
	}
	modelCache.Unlock()
	status := buildModelStatus()
	modelCache.Lock()
	modelCache.Value = status
	modelCache.Expires = now.Add(modelStatusCacheTTL)
	modelCache.Unlock()
	return status
}

func buildModelStatus() *ModelStatus {
	sites, msg := config.LoadManagedSites(rootDir)
	now := time.Now().Unix()
	status := &ModelStatus{Configured: len(sites) > 0, ConfigError: msg, GeneratedAt: now, ExpiresAt: now + modelStatusWindowSeconds, WindowSeconds: modelStatusWindowSeconds, RefreshEverySeconds: modelStatusWindowSeconds, Totals: map[string]int{"siteCount": len(sites), "modelCount": 0, "requestCount": 0, "successCount": 0, "failureCount": 0, "operational": 0, "degraded": 0, "down": 0, "unknown": 0}}
	modelRows := map[string]*ModelRow{}
	start := now - modelStatusWindowSeconds
	for _, site := range sites {
		successLogs, logErr := loadSiteLogs(site, logTypeConsume, start, now)
		errorLogs, errErr := loadSiteLogs(site, logTypeError, start, now)
		if logErr == "" {
			logErr = errErr
		}
		channels, chErr := loadSiteChannels(site)
		pricing, priceErr := loadPricing(site)
		groupSet := map[string]bool{}
		modelChannelStats := map[string][]Channel{}
		for _, ch := range channels {
			for _, g := range ch.Groups {
				groupSet[g] = true
			}
			for _, m := range ch.Models {
				modelChannelStats[m] = append(modelChannelStats[m], ch)
			}
		}
		groups := keys(groupSet)
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
		for model, chans := range modelChannelStats {
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
				cell.GroupStats[g] = gs
			}
			if rec, ok := testResults.Load(site.Name + "\x00" + model); ok {
				cell.ManualTest = rec
			}
			if until, ok := testCooldowns.Load(site.Name + "\x00" + model); ok {
				cell.NextTestAllowedAt = until.(int64)
			}
			row.PerSite[site.Name] = cell
		}
	}
	rows := []ModelRow{}
	for _, row := range modelRows {
		cells := []*ModelCell{}
		for _, c := range row.PerSite {
			cells = append(cells, c)
		}
		row.Status = modelRowStatus(cells)
		for _, c := range cells {
			if c.Configured {
				row.ConfiguredSites++
				if c.Status == "operational" {
					row.OperationalSites++
				}
			}
		}
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
