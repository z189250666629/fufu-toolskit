package main

import (
	"fmt"
	"fufu/activity"
	"fufu/admincore"
	"fufu/newapi"
	"strings"
)

func normalizeToolConfig(cfg ToolConfig, previous ToolConfig) (ToolConfig, error) {
	sites, err := normalizeManagedAPISiteConfigs(cfg.NewAPI.Sites, previous.NewAPI.Sites)
	if err != nil {
		return ToolConfig{}, err
	}
	cfg.NewAPI.Sites = sites
	cfg.Navigation = normalizeNavigationConfig(cfg.Navigation)
	cfg.Activity = activity.CloneConfig(cfg.Activity)
	cfg.MCY = normalizeMCYConfig(cfg.MCY, previous.MCY)
	return cfg, nil
}

func normalizeMCYBaseURL(raw string) string {
	return admincore.NormalizeHTTPSOrigin(raw)
}

func normalizeMCYConfig(c, previous MCYAdminConfig) MCYAdminConfig {
	c.BaseURL = normalizeMCYBaseURL(c.BaseURL)
	c.Username = strings.TrimSpace(c.Username)
	c.Password = strings.TrimSpace(c.Password)
	c.Cookie = strings.TrimSpace(c.Cookie)
	c.LoginEndpoint = strings.TrimSpace(c.LoginEndpoint)
	c.UploadEndpoint = strings.TrimSpace(c.UploadEndpoint)
	if c.Password == "" {
		c.Password = strings.TrimSpace(previous.Password)
	}
	return c
}

func cloneToolConfig(cfg ToolConfig) ToolConfig {
	out := ToolConfig{Navigation: cloneNavigationConfig(cfg.Navigation), Activity: activity.CloneConfig(cfg.Activity), MCY: cfg.MCY}
	out.NewAPI.Sites = make([]ManagedAPISiteConfig, len(cfg.NewAPI.Sites))
	for i, site := range cfg.NewAPI.Sites {
		site.URLs = append([]ManagedSiteURL(nil), site.URLs...)
		out.NewAPI.Sites[i] = site
	}
	return out
}

func managedSiteConfigsFromSites(sites []newapi.Site) []ManagedAPISiteConfig {
	out := []ManagedAPISiteConfig{}
	index := map[string]int{}
	for _, site := range sites {
		category := normalizeAdminSiteCategory(site.Category, site.Name)
		lineName := strings.TrimSpace(site.LineName)
		if lineName == "" {
			lineName = site.Name
		}
		url := ManagedSiteURL{Name: lineName, URL: site.URL}
		key := category + "\x00" + site.Token
		if at, ok := index[key]; ok {
			out[at].URLs = append(out[at].URLs, url)
			out[at].URL = out[at].URLs[0].URL
			continue
		}
		index[key] = len(out)
		out = append(out, ManagedAPISiteConfig{
			Name:                site.Name,
			Category:            category,
			URLs:                []ManagedSiteURL{url},
			URL:                 site.URL,
			Token:               site.Token,
			UserID:              site.UserID,
			Kind:                site.Kind,
			SkipUserHeader:      site.SkipUserHeader,
			QuotaUnit:           site.QuotaUnit,
			Currency:            site.Currency,
			RechargeRatio:       site.RechargeRatio,
			ChannelListEndpoint: site.ChannelListEndpoint,
			Note:                site.Note,
		})
	}
	return out
}

func normalizeManagedAPISiteConfigs(sites, previous []ManagedAPISiteConfig) ([]ManagedAPISiteConfig, error) {
	normalized := []ManagedAPISiteConfig{}
	for i, site := range sites {
		site.Name = strings.TrimSpace(site.Name)
		site.Token = strings.TrimSpace(site.Token)
		site.UserID = strings.TrimSpace(site.UserID)
		site.Kind = strings.TrimSpace(site.Kind)
		site.Currency = strings.TrimSpace(site.Currency)
		site.ChannelListEndpoint = strings.TrimSpace(site.ChannelListEndpoint)
		site.Note = strings.TrimSpace(site.Note)
		site.Category = normalizeAdminSiteCategory(site.Category, site.Name)
		if site.Category != "api" && site.Category != "token" {
			return nil, fmt.Errorf("第 %d 个站点类别不支持（只能 api 或 token）: %s", i+1, site.Category)
		}

		urls := normalizeManagedSiteURLs(site.URLs, site.URL)
		if len(urls) == 0 {
			return nil, fmt.Errorf("第 %d 个 NewAPI 站点至少需要一个 base_url", i+1)
		}
		site.URLs = urls
		site.URL = urls[0].URL

		if site.Token == "" {
			site.Token = matchingSiteToken(site, previous)
		}
		if site.Name == "" {
			return nil, fmt.Errorf("第 %d 个 NewAPI 站点缺少名称", i+1)
		}
		if site.Token == "" {
			return nil, fmt.Errorf("第 %d 个 NewAPI 站点缺少 token", i+1)
		}
		if site.UserID == "" {
			site.UserID = "1"
		}
		if site.Kind == "" {
			site.Kind = "api"
		}
		if !isSupportedAdminSiteKind(site.Kind) {
			return nil, fmt.Errorf("第 %d 个 NewAPI 站点 kind 不支持: %s", i+1, site.Kind)
		}
		if site.QuotaUnit <= 0 {
			site.QuotaUnit = newapi.DefaultQuotaUnit
		}
		if site.Currency == "" {
			site.Currency = "$"
		}
		if site.RechargeRatio <= 0 {
			site.RechargeRatio = 1
		}
		normalized = append(normalized, site)
	}

	merged := mergeManagedSiteConfigsByToken(normalized)
	seen := map[string]bool{}
	for _, site := range merged {
		nameKey := site.Category + "\x00" + site.Name
		if seen[nameKey] {
			return nil, fmt.Errorf("%s 类站点名称重复: %s", site.Category, site.Name)
		}
		seen[nameKey] = true
	}
	return merged, nil
}

func normalizeAdminSiteCategory(category, name string) string {
	category = strings.ToLower(strings.TrimSpace(category))
	if category != "" {
		return category
	}
	if strings.Contains(strings.ToLower(name), "token") {
		return "token"
	}
	return "api"
}

func normalizeManagedSiteURLs(urls []ManagedSiteURL, legacyURL string) []ManagedSiteURL {
	generic := make([]admincore.NamedURL, 0, len(urls))
	for _, u := range urls {
		generic = append(generic, admincore.NamedURL{Name: u.Name, URL: u.URL})
	}
	normalized := admincore.NormalizeNamedURLs(generic, legacyURL, func(index int) string {
		return fmt.Sprintf("线路 %d", index+1)
	})
	out := make([]ManagedSiteURL, 0, len(normalized))
	for _, u := range normalized {
		out = append(out, ManagedSiteURL{Name: u.Name, URL: u.URL})
	}
	return out
}

func mergeManagedSiteConfigsByToken(sites []ManagedAPISiteConfig) []ManagedAPISiteConfig {
	out := []ManagedAPISiteConfig{}
	index := map[string]int{}
	for _, site := range sites {
		key := site.Category + "\x00" + site.Token
		if at, ok := index[key]; ok {
			out[at].URLs = mergeManagedSiteURLs(out[at].URLs, site.URLs)
			out[at].URL = out[at].URLs[0].URL
			continue
		}
		index[key] = len(out)
		out = append(out, site)
	}
	return out
}

func mergeManagedSiteURLs(existing, extra []ManagedSiteURL) []ManagedSiteURL {
	seen := map[string]bool{}
	for _, entry := range existing {
		seen[entry.URL] = true
	}
	for _, entry := range extra {
		if seen[entry.URL] {
			continue
		}
		seen[entry.URL] = true
		existing = append(existing, entry)
	}
	return existing
}

func matchingSiteToken(site ManagedAPISiteConfig, previous []ManagedAPISiteConfig) string {
	for _, candidate := range previous {
		if strings.TrimSpace(candidate.Name) == site.Name && strings.EqualFold(strings.TrimSpace(candidate.Category), site.Category) {
			if token := strings.TrimSpace(candidate.Token); token != "" {
				return token
			}
		}
	}
	return ""
}

func isSupportedAdminSiteKind(kind string) bool {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "api", "managed-api", "managed_api", "admin":
		return true
	default:
		return false
	}
}

func newAPISitesFromManagedConfig(site ManagedAPISiteConfig) []newapi.Site {
	out := make([]newapi.Site, 0, len(site.URLs))
	for _, u := range site.URLs {
		out = append(out, newapi.Site{
			Name:                site.Name,
			Category:            site.Category,
			LineName:            u.Name,
			URL:                 u.URL,
			Token:               site.Token,
			UserID:              site.UserID,
			Kind:                site.Kind,
			SkipUserHeader:      site.SkipUserHeader,
			QuotaUnit:           site.QuotaUnit,
			Currency:            site.Currency,
			RechargeRatio:       site.RechargeRatio,
			ChannelListEndpoint: site.ChannelListEndpoint,
			Note:                site.Note,
		})
	}
	return out
}
