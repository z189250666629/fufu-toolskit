package main

import (
	"fmt"
	activityapp "fufu-act"
	"fufu/config"
	"fufu/newapi"
	"strings"
	"time"
)

func applyToolConfigSnapshot(cfg ToolConfig) {
	activityapp.SetRuntimeConfig(cfg.Activity)
	activityapp.SetMCYRuntimeConfig(activityapp.MCYRuntimeConfig{
		BaseURL:        cfg.MCY.BaseURL,
		Username:       cfg.MCY.Username,
		Password:       cfg.MCY.Password,
		Cookie:         cfg.MCY.Cookie,
		LoginEndpoint:  cfg.MCY.LoginEndpoint,
		UploadEndpoint: cfg.MCY.UploadEndpoint,
	})
	activityapp.SetNewAPIRuntimeSite(newAPIRuntimeSiteForActivity(cfg))
	resetModelStatusCache()
}

func newAPIRuntimeSiteForActivity(cfg ToolConfig) newapi.Site {
	for _, site := range newAPISitesForActivity(cfg) {
		if strings.EqualFold(site.Category, "api") {
			return site
		}
	}
	sites := newAPISitesForActivity(cfg)
	if len(sites) == 0 {
		return newapi.Site{}
	}
	return sites[0]
}

func newAPISitesForActivity(cfg ToolConfig) []newapi.Site {
	sites := make([]newapi.Site, 0, len(cfg.NewAPI.Sites))
	for _, site := range cfg.NewAPI.Sites {
		expanded := newAPISitesFromManagedConfig(site)
		if len(expanded) > 0 {
			sites = append(sites, expanded[0])
		}
	}
	return sites
}

func managedSitesForRuntime() ([]newapi.Site, string) {
	if unifiedConfig != nil {
		return unifiedConfig.ManagedSites(), ""
	}
	return config.LoadManagedSites(rootDir)
}

func primarySiteForCombine() (newapi.Site, error) {
	sites, msg := managedSitesForRuntime()
	for _, site := range sites {
		if strings.EqualFold(site.Category, "api") {
			return site, nil
		}
	}
	if len(sites) > 0 {
		return sites[0], nil
	}
	if msg != "" {
		return newapi.Site{}, fmt.Errorf("%s", msg)
	}
	return newapi.Site{}, fmt.Errorf("missing NewAPI primary site config")
}

func resetModelStatusCache() {
	modelCache.Lock()
	defer modelCache.Unlock()
	modelCache.Value = nil
	modelCache.Expires = time.Time{}
	modelCache.Key = ""
	modelCache.ForceRefreshAfter = time.Time{}
}
