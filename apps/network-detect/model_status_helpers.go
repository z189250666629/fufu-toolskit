package main

import (
	"encoding/json"
	"fufu/newapi"
	"sort"
	"strings"
)

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

func sanitizeChannel(raw map[string]any) Channel {
	return Channel{ID: toInt(raw["id"]), Name: firstNonEmpty(str(raw["name"]), str(raw["channel_name"])), Status: toInt(raw["status"]), Models: parseListValue(raw["models"], raw["model"]), Groups: parseListValue(raw["group"], raw["groups"]), ResponseTime: firstInt(raw, "response_time", "responseTime", "test_time"), Raw: raw}
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

func parseListValue(values ...any) []string {
	for _, value := range values {
		switch x := value.(type) {
		case []string:
			if out := cleanList(x); len(out) > 0 {
				return out
			}
		case []any:
			items := make([]string, 0, len(x))
			for _, item := range x {
				items = append(items, str(item))
			}
			if out := cleanList(items); len(out) > 0 {
				return out
			}
		default:
			if out := parseList(str(value)); len(out) > 0 {
				return out
			}
		}
	}
	return nil
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

func recomputeModelRowSummary(row *ModelRow) {
	row.ConfiguredSites = 0
	row.OperationalSites = 0
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
}

func updateModelStatusTotalsForRowStatus(ms *ModelStatus, oldStatus, newStatus string) {
	if ms == nil || ms.Totals == nil || oldStatus == newStatus {
		return
	}
	if oldStatus != "" && ms.Totals[oldStatus] > 0 {
		ms.Totals[oldStatus]--
	}
	if newStatus != "" {
		ms.Totals[newStatus]++
	}
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
