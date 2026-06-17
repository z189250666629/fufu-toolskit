package main

import (
	"context"

	"fufu/newapi"
)

type modelStatusBuildPlan struct {
	Sites         []newapi.Site
	ConfigError   string
	Now           int64
	WindowSeconds int
}

type modelStatusSiteData struct {
	Site          newapi.Site
	SuccessLogs   []LogRow
	ErrorLogs     []LogRow
	LogError      string
	Channels      []Channel
	ChannelsError string
	Pricing       map[string]Pricing
	PricingError  string
}

func fetchModelStatusSiteData(ctx context.Context, plan modelStatusBuildPlan) []modelStatusSiteData {
	start := plan.Now - int64(plan.WindowSeconds)
	out := make([]modelStatusSiteData, 0, len(plan.Sites))
	for _, site := range plan.Sites {
		successLogs, logErr := loadSiteLogs(ctx, site, logTypeConsume, start, plan.Now)
		errorLogs, errErr := loadSiteLogs(ctx, site, logTypeError, start, plan.Now)
		if logErr == "" {
			logErr = errErr
		}
		channels, chErr := loadSiteChannels(ctx, site)
		pricing, priceErr := loadPricing(ctx, site)
		out = append(out, modelStatusSiteData{
			Site:          site,
			SuccessLogs:   successLogs,
			ErrorLogs:     errorLogs,
			LogError:      logErr,
			Channels:      channels,
			ChannelsError: chErr,
			Pricing:       pricing,
			PricingError:  priceErr,
		})
	}
	return out
}
