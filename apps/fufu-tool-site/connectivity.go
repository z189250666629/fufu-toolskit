package main

import (
	"encoding/json"
	"fmt"
	"fufu/config"
	"net"
	"net/url"
	"os"
	"strings"
)

func env(name string) string { return strings.TrimSpace(os.Getenv(name)) }

func splitList(v string) []string {
	parts := strings.FieldsFunc(v, func(r rune) bool { return r == ',' || r == '\n' || r == '\r' || r == '\t' })
	out := []string{}
	for _, p := range parts {
		if u, ok := publicBrowserTargetOrigin(p); ok {
			out = append(out, u)
		}
	}
	return out
}

func connectivityGroups() []map[string]any {
	groups, _ := connectivityGroupsWithError()
	return groups
}

func connectivityGroupsWithError() ([]map[string]any, string) {
	if inline := env("CONNECTIVITY_TARGETS"); inline != "" {
		var arr []map[string]any
		if err := json.Unmarshal([]byte(inline), &arr); err != nil {
			return nil, "CONNECTIVITY_TARGETS 不是有效 JSON"
		}
		if groups := sanitizeConnectivityGroups(arr); len(groups) > 0 {
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
		u, ok := publicBrowserTargetOrigin(site.URL)
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
	groups := []map[string]any{}
	for _, category := range defaultNavigationLineCategories() {
		urls := make([]string, 0, len(category.Lines))
		for _, line := range category.Lines {
			urls = append(urls, line.URL)
		}
		groups = append(groups, map[string]any{"id": category.Kind, "name": category.Name, "urls": urls})
	}
	return groups
}

func connectivityTargetURLs(explicitName, legacyName, fallbackName string) []string {
	if urls := splitList(firstNonEmpty(env(explicitName), env(legacyName))); len(urls) > 0 {
		return urls
	}
	return publicBrowserTargets(splitList(env(fallbackName)))
}

func sanitizeConnectivityGroups(groups []map[string]any) []map[string]any {
	out := []map[string]any{}
	for _, group := range groups {
		urls := []string{}
		for _, raw := range rawConnectivityURLs(group["urls"]) {
			if u, ok := publicBrowserTargetOrigin(raw); ok {
				urls = append(urls, u)
			}
		}
		if len(urls) == 0 {
			continue
		}
		id := strings.TrimSpace(fmt.Sprint(group["id"]))
		name := strings.TrimSpace(fmt.Sprint(group["name"]))
		if id == "" || id == "<nil>" {
			id = name
		}
		if name == "" || name == "<nil>" {
			name = id
		}
		if id == "" || id == "<nil>" {
			id = "custom"
		}
		if name == "" || name == "<nil>" {
			name = "URL 组"
		}
		out = append(out, map[string]any{"id": id, "name": name, "urls": urls})
	}
	return out
}

func rawConnectivityURLs(value any) []string {
	switch v := value.(type) {
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			out = append(out, fmt.Sprint(item))
		}
		return out
	case []string:
		return v
	default:
		return nil
	}
}

func publicBrowserTargets(urls []string) []string {
	out := []string{}
	for _, target := range urls {
		if u, ok := publicBrowserTargetOrigin(target); ok {
			out = append(out, u)
		}
	}
	return out
}

func isPublicBrowserTarget(target string) bool {
	_, ok := publicBrowserTargetOrigin(target)
	return ok
}

func publicBrowserTargetOrigin(target string) (string, bool) {
	target = config.NormalizeBaseURL(target)
	if target == "" {
		return "", false
	}
	u, err := url.Parse(target)
	if err != nil {
		return "", false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", false
	}
	host := strings.ToLower(strings.Trim(u.Hostname(), "."))
	if host == "" || host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return "", false
	}
	ip := net.ParseIP(host)
	if ip != nil && (ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified()) {
		return "", false
	}
	return (&url.URL{Scheme: u.Scheme, Host: u.Host}).String(), true
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
