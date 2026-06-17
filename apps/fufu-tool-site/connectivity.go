package main

import (
	"fufu/connectivitycore"
	"os"
	"strings"
)

func env(name string) string { return strings.TrimSpace(os.Getenv(name)) }

func connectivityGroups() []map[string]any {
	groups, _ := connectivityGroupsWithError()
	return groups
}

func connectivityGroupsWithError() ([]map[string]any, string) {
	if inline := env("CONNECTIVITY_TARGETS"); inline != "" {
		groups, err := connectivitycore.ParseGroupsJSON(inline)
		if err != nil {
			return nil, "CONNECTIVITY_TARGETS 不是有效 JSON"
		}
		if len(groups) > 0 {
			return groups, ""
		}
		return defaultConnectivityGroups(), ""
	}
	if groups := connectivityGroupsFromManagedSites(); len(groups) > 0 {
		return groups, ""
	}
	groups := []map[string]any{}
	if urls := connectivityTargetURLs("CONNECTIVITY_API_URLS", "FUFU_API_URLS", "NEWAPI_API_SITE_URL"); len(urls) > 0 {
		groups = append(groups, map[string]any{"id": "api", "name": firstNonEmpty(env("CONNECTIVITY_API_NAME"), "API 次数站"), "urls": urls})
	}
	if urls := connectivityTargetURLs("CONNECTIVITY_TOKEN_URLS", "FUFU_TOKEN_URLS", "NEWAPI_TOKEN_SITE_URL"); len(urls) > 0 {
		groups = append(groups, map[string]any{"id": "token", "name": firstNonEmpty(env("CONNECTIVITY_TOKEN_NAME"), "Token 站"), "urls": urls})
	}
	if len(groups) > 0 {
		return groups, ""
	}
	return defaultConnectivityGroups(), ""
}

func connectivityGroupsFromManagedSites() []map[string]any {
	sites, _ := managedSitesForRuntime()
	type groupAccumulator struct {
		id   string
		name string
		urls []string
		seen map[string]bool
	}
	groupsByKind := map[string]*groupAccumulator{}
	for _, category := range defaultNavigationLineCategories() {
		groupsByKind[category.Kind] = &groupAccumulator{
			id:   category.Kind,
			name: category.Name,
			seen: map[string]bool{},
		}
	}
	for _, site := range sites {
		u, ok := connectivitycore.PublicBrowserTargetOrigin(site.URL)
		if !ok {
			continue
		}
		kind := normalizedConnectivitySiteKind(site.Category, site.Name)
		group, ok := groupsByKind[kind]
		if !ok {
			continue
		}
		if !group.seen[u] {
			group.urls = append(group.urls, u)
			group.seen[u] = true
		}
	}
	out := []map[string]any{}
	for _, kind := range []string{"api", "token"} {
		group := groupsByKind[kind]
		if group != nil && len(group.urls) > 0 {
			out = append(out, map[string]any{"id": group.id, "name": group.name, "urls": group.urls})
		}
	}
	return out
}

func normalizedConnectivitySiteKind(category, name string) string {
	kind := strings.ToLower(strings.TrimSpace(category))
	if kind == "" {
		if strings.Contains(strings.ToLower(strings.TrimSpace(name)), "token") {
			return "token"
		}
		return "api"
	}
	switch kind {
	case "api", "token":
		return kind
	default:
		return ""
	}
}

func defaultConnectivityGroups() []map[string]any {
	inputs := []connectivitycore.GroupInput{}
	for _, category := range defaultNavigationLineCategories() {
		urls := make([]string, 0, len(category.Lines))
		for _, line := range category.Lines {
			urls = append(urls, line.URL)
		}
		inputs = append(inputs, connectivitycore.GroupInput{ID: category.Kind, Name: category.Name, URLs: urls})
	}
	return connectivitycore.BuildGroups(inputs)
}

func connectivityTargetURLs(explicitName, legacyName, fallbackName string) []string {
	return connectivitycore.TargetURLs(env(explicitName), env(legacyName), env(fallbackName))
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
