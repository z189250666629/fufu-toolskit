package main

import (
	"bytes"
	"encoding/json"
	"fufu/config"
	"fufu/connectivitycore"
	"fufu/newapi"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

func defaultConnectivityGroupInputs() []connectivitycore.GroupInput {
	return []connectivitycore.GroupInput{
		{ID: "api", Name: "API 次数站", URLs: []string{"https://api.fufuapi.top", "https://api.fufuapi.online", "https://api.fufuflower.top"}},
		{ID: "token", Name: "Token 站", URLs: []string{"https://token.fufuapi.top", "https://token.fufuapi.online", "https://token.fufuflower.top"}},
	}
}

func defaultConnectivityTargetsByKind() map[string][]string {
	out := map[string][]string{}
	for _, input := range defaultConnectivityGroupInputs() {
		out[input.ID] = append([]string(nil), input.URLs...)
	}
	return out
}

func managedConnectivityTargetsByKind() map[string][]string {
	out := map[string][]string{}
	sites := loadManagedSitesForConnectivity()
	for _, site := range sites {
		origin, ok := connectivitycore.PublicBrowserTargetOrigin(site.URL)
		if !ok {
			continue
		}
		kind := normalizedConnectivitySiteKind(site.Category, site.Name, site.URL)
		if kind == "" || containsString(out[kind], origin) {
			continue
		}
		out[kind] = append(out[kind], origin)
	}
	return out
}

func loadManagedSitesForConnectivity() []newapi.Site {
	if env("NEWAPI_MANAGED_API_SITES") != "" {
		sites, _ := config.LoadManagedSites(rootDir)
		return sites
	}
	if sites := config.DeploymentSitesFromEnv(); len(sites) > 0 {
		return sites
	}
	if configured := env("NEWAPI_MANAGED_API_CONFIG"); configured != "" {
		if !filepath.IsAbs(configured) && strings.TrimSpace(rootDir) != "" {
			configured = filepath.Join(rootDir, configured)
		}
		return loadManagedSitesFromFile(configured)
	}
	if strings.TrimSpace(rootDir) == "" {
		return nil
	}
	return loadManagedSitesFromFile(filepath.Join(rootDir, "newapi-managed-api-sites.json"))
}

func loadManagedSitesFromFile(path string) []newapi.Site {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var data any
	if err := decoder.Decode(&data); err != nil {
		return nil
	}
	sites, _ := config.NormalizeManagedSites(data, nil)
	return sites
}

func normalizedConnectivitySiteKind(category, name, rawURL string) string {
	switch strings.ToLower(strings.TrimSpace(category)) {
	case "api":
		return "api"
	case "token":
		return "token"
	}
	lowerName := strings.ToLower(strings.TrimSpace(name))
	if lowerName == "token-fufu" || strings.Contains(lowerName, "token") {
		return "token"
	}
	defaults := defaultConnectivityTargetsByKind()
	if origin, ok := connectivitycore.PublicBrowserTargetOrigin(rawURL); ok {
		if containsString(defaults["token"], origin) {
			return "token"
		}
		if containsString(defaults["api"], origin) {
			return "api"
		}
	}
	if strings.TrimSpace(name) != "" {
		return "api"
	}
	return ""
}

func fufuLogicalConnectivityKind(site newapi.Site) string {
	switch strings.TrimSpace(site.Name) {
	case "次数fufu":
		return "api"
	case "token-fufu":
		return "token"
	}
	defaults := defaultConnectivityTargetsByKind()
	if origin, ok := connectivitycore.PublicBrowserTargetOrigin(site.URL); ok {
		if containsString(defaults["api"], origin) {
			return "api"
		}
		if containsString(defaults["token"], origin) {
			return "token"
		}
	}
	return ""
}

func candidateSiteURLs(site newapi.Site) []string {
	baseURL := config.NormalizeBaseURL(site.URL)
	if baseURL == "" {
		return nil
	}
	if _, ok := connectivitycore.PublicBrowserTargetOrigin(baseURL); !ok {
		return []string{baseURL}
	}
	kind := fufuLogicalConnectivityKind(site)
	if kind == "" {
		return []string{baseURL}
	}
	urls := connectivityGroupURLs(kind, defaultConnectivityTargetsByKind()[kind], managedConnectivityTargetsByKind()[kind])
	if !containsString(urls, baseURL) {
		urls = append(urls, baseURL)
	}
	return urls
}

func expandSitesForModelStatus(sites []newapi.Site) []newapi.Site {
	expanded := make([]newapi.Site, 0, len(sites))
	for _, site := range sites {
		candidates := candidateSiteURLs(site)
		if len(candidates) == 0 {
			continue
		}
		for index, targetURL := range candidates {
			next := site
			next.URL = targetURL
			next.LineName = candidateSiteLineName(site, targetURL, index)
			next.Category = normalizedConnectivitySiteKind(site.Category, site.Name, targetURL)
			expanded = append(expanded, next)
		}
	}
	return expanded
}

func orderedManualTestSites(site newapi.Site, preferredURL string) []newapi.Site {
	urls := candidateSiteURLs(site)
	if len(urls) == 0 {
		return nil
	}
	normalizedPreferred := normalizedSiteCandidateKey(preferredURL)
	if normalizedPreferred != "" {
		reordered := make([]string, 0, len(urls))
		for _, targetURL := range urls {
			if normalizedSiteCandidateKey(targetURL) == normalizedPreferred {
				reordered = append(reordered, targetURL)
			}
		}
		for _, targetURL := range urls {
			if normalizedSiteCandidateKey(targetURL) != normalizedPreferred {
				reordered = append(reordered, targetURL)
			}
		}
		if len(reordered) == len(urls) {
			urls = reordered
		}
	}
	out := make([]newapi.Site, 0, len(urls))
	for index, targetURL := range urls {
		next := site
		next.URL = targetURL
		next.LineName = candidateSiteLineName(site, targetURL, index)
		next.Category = normalizedConnectivitySiteKind(site.Category, site.Name, targetURL)
		out = append(out, next)
	}
	return out
}

func candidateSiteLineName(site newapi.Site, targetURL string, index int) string {
	if strings.TrimSpace(site.LineName) != "" && index == 0 {
		return strings.TrimSpace(site.LineName)
	}
	if parsed, err := url.Parse(targetURL); err == nil && strings.TrimSpace(parsed.Host) != "" {
		return strings.TrimSpace(parsed.Host)
	}
	if strings.TrimSpace(site.LineName) != "" {
		return strings.TrimSpace(site.LineName)
	}
	return strings.TrimSpace(site.Name)
}

func normalizedSiteCandidateKey(raw string) string {
	raw = config.NormalizeBaseURL(raw)
	if raw == "" {
		return ""
	}
	if origin, ok := connectivitycore.PublicBrowserTargetOrigin(raw); ok {
		return origin
	}
	return raw
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item), strings.TrimSpace(want)) {
			return true
		}
	}
	return false
}
