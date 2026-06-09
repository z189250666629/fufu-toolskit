package main

import (
	"context"
	"encoding/json"
	"fufu/config"
	"fufu/newapi"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

func loadSiteLogs(site newapi.Site, typ int, start, end int64) ([]LogRow, string) {
	q := url.Values{}
	q.Set("p", "1")
	q.Set("page", "1")
	q.Set("size", strconv.Itoa(modelLogPageSize))
	q.Set("page_size", strconv.Itoa(modelLogPageSize))
	q.Set("type", strconv.Itoa(typ))
	q.Set("start_timestamp", strconv.FormatInt(start, 10))
	q.Set("end_timestamp", strconv.FormatInt(end, 10))
	paths := []string{"/api/log/self?" + q.Encode(), "/api/log/?" + q.Encode()}
	var last string
	for _, p := range paths {
		res := newAPIGet(context.Background(), site, p, 12*time.Second)
		if res.OK {
			rows := []LogRow{}
			for _, raw := range items(res.Data) {
				rows = append(rows, sanitizeLog(raw))
			}
			if len(rows) > modelLogMaxRowsPerType {
				rows = rows[:modelLogMaxRowsPerType]
			}
			return rows, ""
		}
		last = res.Error
	}
	return nil, last
}

func sanitizeLog(raw map[string]any) LogRow {
	return LogRow{ModelName: firstNonEmpty(str(raw["model_name"]), str(raw["modelName"]), str(raw["model"])), TokenName: firstNonEmpty(str(raw["token_name"]), str(raw["tokenName"]), str(raw["token"])), Group: firstNonEmpty(str(raw["group"]), str(raw["groups"])), RequestID: firstNonEmpty(str(raw["request_id"]), str(raw["requestId"])), Quota: toInt64(raw["quota"]), CreatedAt: firstInt(raw, "created_at", "createdAt", "created_time", "createdTime"), Status: toInt(raw["status"]), Raw: raw}
}

func firstInt(raw map[string]any, keys ...string) int64 {
	for _, k := range keys {
		if v, ok := raw[k]; ok && str(v) != "" {
			return toInt64(v)
		}
	}
	return 0
}

func loadSiteChannels(site newapi.Site) ([]Channel, string) {
	endpoints := []string{}
	if site.ChannelListEndpoint != "" {
		endpoints = append(endpoints, site.ChannelListEndpoint)
	}
	endpoints = append(endpoints, "/api/channel/search?keyword=&p=1&page_size=500", "/api/channel/?p=1&page_size=500", "/api/channel/search?keyword=&p=0&size=500", "/api/channel/?p=0&size=500")
	var last string
	for _, ep := range endpoints {
		res := newAPIGet(context.Background(), site, ep, 12*time.Second)
		if res.OK {
			out := []Channel{}
			for _, raw := range items(res.Data) {
				out = append(out, sanitizeChannel(raw))
			}
			return out, ""
		}
		last = res.Error
	}
	return nil, last
}

func sanitizeChannel(raw map[string]any) Channel {
	return Channel{ID: toInt(raw["id"]), Name: firstNonEmpty(str(raw["name"]), str(raw["channel_name"])), Status: toInt(raw["status"]), Models: parseList(firstNonEmpty(str(raw["models"]), str(raw["model"]))), Groups: parseList(firstNonEmpty(str(raw["group"]), str(raw["groups"]))), ResponseTime: firstInt(raw, "response_time", "responseTime", "test_time"), Raw: raw}
}

func parseList(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var arr []string
	if strings.HasPrefix(raw, "[") && json.Unmarshal([]byte(raw), &arr) == nil {
		return cleanList(arr)
	}
	return cleanList(strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == '\n' || r == '\r' || r == ' ' || r == '|' }))
}

func cleanList(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, item := range in {
		item = strings.TrimSpace(strings.Trim(item, "\"'"))
		if item != "" && !seen[item] {
			seen[item] = true
			out = append(out, item)
		}
	}
	sort.Strings(out)
	return out
}

func loadPricing(site newapi.Site) (map[string]Pricing, string) {
	res := newAPIGet(context.Background(), site, "/api/pricing", 12*time.Second)
	if !res.OK {
		return nil, res.Error
	}
	out := map[string]Pricing{}
	data := res.Data
	for _, raw := range items(data) {
		model := firstNonEmpty(str(raw["model_name"]), str(raw["modelName"]), str(raw["model"]), str(raw["name"]))
		if model == "" {
			continue
		}
		out[model] = pricingFromRaw(site, raw)
	}
	if len(out) == 0 {
		if models, ok := data["data"].(map[string]any); ok {
			for model, val := range models {
				if raw, ok := val.(map[string]any); ok {
					out[model] = pricingFromRaw(site, raw)
				}
			}
		}
	}
	return out, ""
}

func pricingFromRaw(site newapi.Site, raw map[string]any) Pricing {
	input := firstFloat(raw, "input", "prompt", "prompt_ratio", "model_ratio")
	output := firstFloat(raw, "output", "completion", "completion_ratio", "completionRatio")
	if output == 0 {
		output = input
	}
	return Pricing{Input: input * site.RechargeRatio, Output: output * site.RechargeRatio, Currency: site.Currency}
}

func firstFloat(raw map[string]any, keys ...string) float64 {
	for _, k := range keys {
		if v, ok := raw[k]; ok && str(v) != "" {
			return toFloat(v)
		}
	}
	return 0
}

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
				gs := buildCell(site.Name, model, groupChans, successByMG[model+"\x00"+g], errorByMG[model+"\x00"+g], pricing[model])
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

func keys(m map[string]bool) []string {
	out := []string{}
	for k := range m {
		if k != "" {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

func contains(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

func rate(s, f int) *float64 {
	total := s + f
	if total == 0 {
		return nil
	}
	r := float64(s) / float64(total)
	return &r
}

func statusFromCounts(s, f int) string {
	if s+f == 0 {
		return "unknown"
	}
	r := float64(s) / float64(s+f)
	if r >= 0.8 {
		return "operational"
	}
	if r > 0 {
		return "degraded"
	}
	return "down"
}

func siteStatusFromCounts(s, f int) string { return statusFromCounts(s, f) }

func modelRowStatus(cells []*ModelCell) string {
	configured := 0
	op := 0
	degraded := 0
	down := 0
	for _, c := range cells {
		if c.Configured {
			configured++
			switch c.Status {
			case "operational":
				op++
			case "degraded":
				degraded++
			case "down":
				down++
			}
		}
	}
	if configured == 0 {
		return "unknown"
	}
	if down > 0 {
		return "down"
	}
	if degraded > 0 {
		return "degraded"
	}
	if op > 0 {
		return "operational"
	}
	return "unknown"
}

func maxLogTime(rows []LogRow) int64 {
	var m int64
	for _, r := range rows {
		if r.CreatedAt > m {
			m = r.CreatedAt
		}
	}
	return m
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func groupLogs(rows []LogRow) map[string][]LogRow {
	m := map[string][]LogRow{}
	for _, r := range rows {
		if r.ModelName != "" {
			m[r.ModelName] = append(m[r.ModelName], r)
		}
	}
	return m
}

func groupLogsByModelGroup(rows []LogRow) map[string][]LogRow {
	m := map[string][]LogRow{}
	for _, r := range rows {
		for _, g := range parseList(r.Group) {
			if r.ModelName != "" && g != "" {
				m[r.ModelName+"\x00"+g] = append(m[r.ModelName+"\x00"+g], r)
			}
		}
	}
	return m
}
