package main

import (
	"fmt"
	"sort"
	"strings"
)

type modelManualProjection struct {
	Record        any
	HasRecord     bool
	NextAllowedAt int64
}

type modelManualProjectionLookup func(siteName, model, group string) modelManualProjection

func projectModelStatus(plan modelStatusBuildPlan, siteData []modelStatusSiteData, lookup modelManualProjectionLookup) *ModelStatus {
	status := newModelStatusResponse(plan)
	modelRows := map[string]*ModelRow{}
	for _, data := range mergeModelStatusSiteDataByLogicalSite(siteData) {
		built := buildPerSiteModelStatus(data)
		status.Sites = append(status.Sites, built.Status)
		addSiteStatusTotals(status, built.Status)
		appendSiteModelRows(modelRows, data.Site.Name, built, lookup)
	}
	status.Totals["siteCount"] = len(status.Sites)
	status.Models = sortedModelRows(modelRows, status)
	status.Totals["modelCount"] = len(status.Models)
	return status
}

func newModelStatusResponse(plan modelStatusBuildPlan) *ModelStatus {
	return &ModelStatus{
		Configured:          len(plan.Sites) > 0,
		ConfigError:         plan.ConfigError,
		GeneratedAt:         plan.Now,
		ExpiresAt:           plan.Now + int64(plan.WindowSeconds),
		WindowSeconds:       plan.WindowSeconds,
		RefreshEverySeconds: plan.WindowSeconds,
		Totals: map[string]int{
			"siteCount":    len(plan.Sites),
			"modelCount":   0,
			"requestCount": 0,
			"successCount": 0,
			"failureCount": 0,
			"operational":  0,
			"degraded":     0,
			"down":         0,
			"unknown":      0,
		},
	}
}

func mergeModelStatusSiteDataByLogicalSite(siteData []modelStatusSiteData) []modelStatusSiteData {
	indexByKey := map[string]int{}
	merged := []modelStatusSiteData{}
	for _, data := range siteData {
		data = filterModelStatusSiteDataByFixedGroups(data)
		key := modelStatusLogicalSiteKey(data.Site.Name, data.Site.Category, data.Site.URL)
		if index, ok := indexByKey[key]; ok {
			merged[index] = mergeModelStatusSiteData(merged[index], data)
			continue
		}
		indexByKey[key] = len(merged)
		merged = append(merged, data)
	}
	return merged
}

func filterModelStatusSiteDataByFixedGroups(data modelStatusSiteData) modelStatusSiteData {
	groups := fixedModelStatusGroups(data.Site.Name)
	if len(groups) == 0 {
		return data
	}
	allowed := groupSet(groups)
	data.SuccessLogs = filterLogsByAllowedGroups(data.SuccessLogs, allowed)
	data.ErrorLogs = filterLogsByAllowedGroups(data.ErrorLogs, allowed)
	data.Channels = filterChannelsByAllowedGroups(data.Channels, allowed)
	return data
}

func fixedModelStatusGroups(siteName string) []string {
	if strings.TrimSpace(siteName) == "次数fufu" {
		return []string{"mix"}
	}
	return nil
}

func groupSet(groups []string) map[string]bool {
	out := map[string]bool{}
	for _, group := range groups {
		group = strings.TrimSpace(group)
		if group != "" {
			out[group] = true
		}
	}
	return out
}

func filterLogsByAllowedGroups(rows []LogRow, allowed map[string]bool) []LogRow {
	out := []LogRow{}
	for _, row := range rows {
		groups := intersectGroups(parseList(row.Group), allowed)
		if len(groups) == 0 {
			continue
		}
		row.Group = strings.Join(groups, " ")
		out = append(out, row)
	}
	return out
}

func filterChannelsByAllowedGroups(channels []Channel, allowed map[string]bool) []Channel {
	out := []Channel{}
	for _, channel := range channels {
		groups := intersectGroups(channel.Groups, allowed)
		if len(groups) == 0 {
			continue
		}
		channel.Groups = groups
		out = append(out, channel)
	}
	return out
}

func intersectGroups(groups []string, allowed map[string]bool) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, group := range groups {
		group = strings.TrimSpace(group)
		if group == "" || !allowed[group] || seen[group] {
			continue
		}
		seen[group] = true
		out = append(out, group)
	}
	sort.Strings(out)
	return out
}

func modelStatusLogicalSiteKey(name, category, url string) string {
	if name = strings.TrimSpace(name); name != "" {
		return name
	}
	if category = strings.TrimSpace(category); category != "" {
		return category
	}
	return strings.TrimSpace(url)
}

func mergeModelStatusSiteData(left, right modelStatusSiteData) modelStatusSiteData {
	left.SuccessLogs = appendUniqueLogs(left.SuccessLogs, right.SuccessLogs)
	left.ErrorLogs = appendUniqueLogs(left.ErrorLogs, right.ErrorLogs)
	left.Channels = appendUniqueChannels(left.Channels, right.Channels)
	left.Pricing = mergePricing(left.Pricing, right.Pricing)
	left.LogError = mergeModelStatusError(left.LogError, right.LogError)
	left.ChannelsError = mergeModelStatusError(left.ChannelsError, right.ChannelsError)
	left.PricingError = mergeModelStatusError(left.PricingError, right.PricingError)
	return left
}

func appendUniqueLogs(left, right []LogRow) []LogRow {
	seen := map[string]bool{}
	out := make([]LogRow, 0, len(left)+len(right))
	for _, row := range append(left, right...) {
		key := logDedupeKey(row)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, row)
	}
	return out
}

func logDedupeKey(row LogRow) string {
	if requestID := strings.TrimSpace(row.RequestID); requestID != "" {
		return "request:" + requestID
	}
	return fmt.Sprintf("log:%s\x00%s\x00%d\x00%d\x00%d", row.ModelName, row.Group, row.CreatedAt, row.Status, row.Quota)
}

func appendUniqueChannels(left, right []Channel) []Channel {
	seen := map[string]bool{}
	out := make([]Channel, 0, len(left)+len(right))
	for _, channel := range append(left, right...) {
		key := channelDedupeKey(channel)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, channel)
	}
	return out
}

func channelDedupeKey(channel Channel) string {
	if channel.ID != 0 {
		return fmt.Sprintf("id:%d", channel.ID)
	}
	return fmt.Sprintf("channel:%s\x00%d\x00%s\x00%s", channel.Name, channel.Status, strings.Join(channel.Models, "\x1f"), strings.Join(channel.Groups, "\x1f"))
}

func mergePricing(left, right map[string]Pricing) map[string]Pricing {
	if len(left) == 0 {
		return right
	}
	for model, price := range right {
		if _, exists := left[model]; !exists {
			left[model] = price
		}
	}
	return left
}

func mergeModelStatusError(left, right string) string {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if left == "" || left == right {
		return right
	}
	if right == "" {
		return left
	}
	return left + "; " + right
}

func addSiteStatusTotals(status *ModelStatus, site SiteStatus) {
	status.Totals["requestCount"] += site.RequestCount
	status.Totals["successCount"] += site.SuccessCount
	status.Totals["failureCount"] += site.FailureCount
}

func appendSiteModelRows(modelRows map[string]*ModelRow, siteName string, built builtModelStatusSite, lookup modelManualProjectionLookup) {
	for model, chans := range built.ChannelIndex.channelsByModel {
		row := modelRows[model]
		if row == nil {
			row = &ModelRow{Model: model, PerSite: map[string]*ModelCell{}}
			modelRows[model] = row
		}
		cell := buildCell(siteName, model, chans, built.SuccessByModel[model], built.ErrorByModel[model], built.Pricing[model])
		cell.GroupStats = buildGroupStats(siteName, model, chans, built, lookup)
		applyManualProjection(cell, lookupModelManualProjection(lookup, siteName, model, ""))
		row.PerSite[siteName] = cell
	}
}

func buildGroupStats(siteName, model string, chans []Channel, built builtModelStatusSite, lookup modelManualProjectionLookup) map[string]*ModelCell {
	groupStats := map[string]*ModelCell{}
	for _, group := range built.ChannelIndex.groups {
		groupChans := channelsForGroup(chans, group)
		if len(groupChans) == 0 {
			continue
		}
		logKey := modelGroupLogKey(model, group)
		cell := buildCell(siteName, model, groupChans, built.SuccessByModelGroup[logKey], built.ErrorByModelGroup[logKey], built.Pricing[model])
		cell.Groups = []string{group}
		applyManualProjection(cell, lookupModelManualProjection(lookup, siteName, model, group))
		groupStats[group] = cell
	}
	return groupStats
}

func channelsForGroup(chans []Channel, group string) []Channel {
	out := []Channel{}
	for _, ch := range chans {
		if contains(ch.Groups, group) {
			out = append(out, ch)
		}
	}
	return out
}

func sortedModelRows(modelRows map[string]*ModelRow, status *ModelStatus) []ModelRow {
	rows := []ModelRow{}
	for _, row := range modelRows {
		recomputeModelRowSummary(row)
		rows = append(rows, *row)
		status.Totals[row.Status]++
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Model < rows[j].Model })
	return rows
}

func applyManualProjection(cell *ModelCell, projection modelManualProjection) {
	if projection.HasRecord {
		cell.ManualTest = projection.Record
	}
	if projection.NextAllowedAt != 0 {
		cell.NextTestAllowedAt = projection.NextAllowedAt
	}
}

func lookupModelManualProjection(lookup modelManualProjectionLookup, siteName, model, group string) modelManualProjection {
	if lookup == nil {
		return modelManualProjection{}
	}
	return lookup(siteName, model, group)
}

func runtimeModelManualProjection(siteName, model, group string) modelManualProjection {
	key := modelManualKey(siteName, model, group)
	projection := modelManualProjection{}
	if rec, ok := testResults.Load(key); ok {
		projection.Record = rec
		projection.HasRecord = true
	}
	if until, ok := testCooldowns.Load(key); ok {
		if next, ok := until.(int64); ok {
			projection.NextAllowedAt = next
		}
	}
	return projection
}
