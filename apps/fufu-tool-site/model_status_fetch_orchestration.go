package main

import (
	"context"
	"sync"

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
	out := make([]modelStatusSiteData, len(plan.Sites))
	var wg sync.WaitGroup
	for i, site := range plan.Sites {
		i, site := i, site
		wg.Add(1)
		go func() {
			defer wg.Done()
			out[i] = fetchSingleModelStatusSiteData(ctx, site, start, plan.Now)
		}()
	}
	wg.Wait()
	return out
}

func fetchSingleModelStatusSiteData(ctx context.Context, site newapi.Site, start, end int64) modelStatusSiteData {
	successLogs, logErr := loadSiteLogs(ctx, site, logTypeConsume, start, end)
	errorLogs, errErr := loadSiteLogs(ctx, site, logTypeError, start, end)
	if logErr == "" {
		logErr = errErr
	}
	channels, chErr := loadSiteChannels(ctx, site)
	pricing, priceErr := loadPricing(ctx, site)
	return modelStatusSiteData{
		Site:          site,
		SuccessLogs:   successLogs,
		ErrorLogs:     errorLogs,
		LogError:      logErr,
		Channels:      channels,
		ChannelsError: chErr,
		Pricing:       pricing,
		PricingError:  priceErr,
	}
}
