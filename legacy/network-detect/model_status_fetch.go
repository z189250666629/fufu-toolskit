package main

import (
	"context"
	"fufu/newapi"
	"net/url"
	"strconv"
	"time"
)

func loadSiteLogs(ctx context.Context, site newapi.Site, typ int, start, end int64) ([]LogRow, string) {
	ctx = contextOrBackground(ctx)
	paths := []string{"/api/log/self", "/api/log/"}
	var last string
	for _, p := range paths {
		rows := []LogRow{}
		for page := 1; len(rows) < modelLogMaxRowsPerType; page++ {
			res := newAPIGet(ctx, site, logPathForPage(p, typ, start, end, page), 12*time.Second)
			if !res.OK {
				last = res.Error
				if len(rows) > 0 {
					return rows, last
				}
				break
			}
			before := len(rows)
			for _, raw := range items(res.Data) {
				rows = append(rows, sanitizeLog(raw))
			}
			if len(rows) > modelLogMaxRowsPerType {
				rows = rows[:modelLogMaxRowsPerType]
			}
			if len(rows)-before < modelLogPageSize {
				return rows, ""
			}
		}
		if len(rows) > 0 {
			return rows, ""
		}
	}
	return nil, last
}

func logPathForPage(path string, typ int, start, end int64, page int) string {
	q := url.Values{}
	q.Set("p", strconv.Itoa(page))
	q.Set("page", strconv.Itoa(page))
	q.Set("size", strconv.Itoa(modelLogPageSize))
	q.Set("page_size", strconv.Itoa(modelLogPageSize))
	q.Set("type", strconv.Itoa(typ))
	q.Set("start_timestamp", strconv.FormatInt(start, 10))
	q.Set("end_timestamp", strconv.FormatInt(end, 10))
	return path + "?" + q.Encode()
}

func loadSiteChannels(ctx context.Context, site newapi.Site) ([]Channel, string) {
	ctx = contextOrBackground(ctx)
	endpoints := []string{}
	if site.ChannelListEndpoint != "" {
		endpoints = append(endpoints, site.ChannelListEndpoint)
	}
	endpoints = append(endpoints, "/api/channel/search?keyword=&p=1&page_size=500", "/api/channel/?p=1&page_size=500", "/api/channel/search?keyword=&p=0&size=500", "/api/channel/?p=0&size=500")
	var last string
	for _, ep := range endpoints {
		res := newAPIGet(ctx, site, ep, 12*time.Second)
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

func loadPricing(ctx context.Context, site newapi.Site) (map[string]Pricing, string) {
	ctx = contextOrBackground(ctx)
	res := newAPIGet(ctx, site, "/api/pricing", 12*time.Second)
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

func contextOrBackground(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}
