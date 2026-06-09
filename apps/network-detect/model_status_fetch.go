package main

import (
	"context"
	"fufu/newapi"
	"net/url"
	"strconv"
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
