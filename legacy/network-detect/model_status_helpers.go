package main

import (
	"fufu/modelcore"
	"fufu/newapi"
)

func sanitizeLog(raw map[string]any) LogRow { return modelcore.SanitizeLog(raw) }

func firstInt(raw map[string]any, keys ...string) int64 { return modelcore.FirstInt(raw, keys...) }

func sanitizeChannel(raw map[string]any) Channel { return modelcore.SanitizeChannel(raw) }

func parseList(raw string) []string { return modelcore.ParseList(raw) }

func parseListValue(values ...any) []string { return modelcore.ParseListValue(values...) }

func cleanList(in []string) []string { return modelcore.CleanList(in) }

func pricingFromRaw(site newapi.Site, raw map[string]any) Pricing {
	return modelcore.PricingFromRaw(modelcore.PricingSite{Currency: site.Currency, RechargeRatio: site.RechargeRatio}, raw)
}

func publicModelSite(site newapi.Site) PublicSite {
	return PublicSite{
		Name:           site.Name,
		Category:       site.Category,
		DisplayURL:     "地址已隐藏",
		UserID:         site.UserID,
		Kind:           site.Kind,
		SkipUserHeader: site.SkipUserHeader,
		QuotaUnit:      site.QuotaUnit,
		Currency:       site.Currency,
		RechargeRatio:  site.RechargeRatio,
	}
}

func firstFloat(raw map[string]any, keys ...string) float64 {
	return modelcore.FirstFloat(raw, keys...)
}

func keys(m map[string]bool) []string { return modelcore.Keys(m) }

func contains(xs []string, v string) bool { return modelcore.Contains(xs, v) }

func rate(s, f int) *float64 { return modelcore.Rate(s, f) }

func statusFromCounts(s, f int) string { return modelcore.StatusFromCounts(s, f) }

func siteStatusFromCounts(s, f int) string { return modelcore.StatusFromCounts(s, f) }

func modelRowStatus(cells []*ModelCell) string { return modelcore.ModelRowStatus(cells) }

func recomputeModelRowSummary(row *ModelRow) { modelcore.RecomputeModelRowSummary(row) }

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

func maxLogTime(rows []LogRow) int64 { return modelcore.MaxLogTime(rows) }

func maxInt64(a, b int64) int64 { return modelcore.MaxInt64(a, b) }
