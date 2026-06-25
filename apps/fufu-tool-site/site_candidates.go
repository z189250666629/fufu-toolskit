package main

import (
	"fufu/config"
	"fufu/connectivitycore"
	"fufu/newapi"
	"net/url"
	"strings"
)

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
			if kind := candidateConnectivityKind(site, targetURL); kind != "" {
				next.Category = kind
			}
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
		if kind := candidateConnectivityKind(site, targetURL); kind != "" {
			next.Category = kind
		}
		out = append(out, next)
	}
	return out
}

func candidateSiteURLs(site newapi.Site) []string {
	baseURL := config.NormalizeBaseURL(site.URL)
	if baseURL == "" {
		return nil
	}
	if _, ok := connectivitycore.PublicBrowserTargetOrigin(baseURL); !ok {
		return []string{baseURL}
	}

	urls := []string{}
	if kind := candidateConnectivityKind(site, baseURL); kind != "" {
		if explicit := explicitConnectivityURLs(kind); len(explicit) > 0 {
			urls = append(urls, explicit...)
		} else if isStandardFufuConnectivitySite(site, baseURL) {
			urls = append(urls, defaultConnectivityTargetURLs(kind)...)
		}
	}
	urls = append(urls, sameRuntimeSiteURLs(site)...)
	urls = append(urls, baseURL)
	return normalizeCandidateURLs(urls)
}

func sameRuntimeSiteURLs(site newapi.Site) []string {
	sites, _ := managedSitesForRuntime()
	out := []string{}
	for _, candidate := range sites {
		if !sameManagedSiteCredential(site, candidate) {
			continue
		}
		if u := config.NormalizeBaseURL(candidate.URL); u != "" {
			out = append(out, u)
		}
	}
	return out
}

func sameManagedSiteCredential(left, right newapi.Site) bool {
	if strings.TrimSpace(left.Token) == "" || strings.TrimSpace(left.Token) != strings.TrimSpace(right.Token) {
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(left.Category), strings.TrimSpace(right.Category)) {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(left.Name), strings.TrimSpace(right.Name))
}

func explicitConnectivityURLs(kind string) []string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "api":
		return connectivitycore.SplitPublicTargetList(firstNonEmpty(env("CONNECTIVITY_API_URLS"), env("FUFU_API_URLS")))
	case "token":
		return connectivitycore.SplitPublicTargetList(firstNonEmpty(env("CONNECTIVITY_TOKEN_URLS"), env("FUFU_TOKEN_URLS")))
	default:
		return nil
	}
}

func defaultConnectivityTargetURLs(kind string) []string {
	for _, category := range defaultNavigationLineCategories() {
		if strings.EqualFold(category.Kind, kind) {
			urls := make([]string, 0, len(category.Lines))
			for _, line := range category.Lines {
				urls = append(urls, line.URL)
			}
			return connectivitycore.PublicBrowserTargets(urls)
		}
	}
	return nil
}

func candidateConnectivityKind(site newapi.Site, rawURL string) string {
	if kind := fufuLogicalConnectivityKind(site, rawURL); kind != "" {
		return kind
	}
	category := strings.ToLower(strings.TrimSpace(site.Category))
	if category == "api" || category == "token" {
		return category
	}
	return ""
}

func fufuLogicalConnectivityKind(site newapi.Site, rawURL string) string {
	switch strings.TrimSpace(site.Name) {
	case "次数fufu":
		return "api"
	case "token-fufu":
		return "token"
	}
	origin, ok := connectivitycore.PublicBrowserTargetOrigin(rawURL)
	if !ok {
		return ""
	}
	for _, kind := range []string{"api", "token"} {
		if containsCandidateURL(defaultConnectivityTargetURLs(kind), origin) {
			return kind
		}
	}
	return ""
}

func isStandardFufuConnectivitySite(site newapi.Site, rawURL string) bool {
	return fufuLogicalConnectivityKind(site, rawURL) != ""
}

func normalizeCandidateURLs(urls []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, raw := range urls {
		normalized := config.NormalizeBaseURL(raw)
		if normalized == "" {
			continue
		}
		key := normalizedSiteCandidateKey(normalized)
		if key == "" {
			key = normalized
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, normalized)
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

func containsCandidateURL(items []string, want string) bool {
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item), strings.TrimSpace(want)) {
			return true
		}
	}
	return false
}
