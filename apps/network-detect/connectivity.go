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

func defaultConnectivityGroups() []map[string]any {
	return connectivitycore.BuildGroups([]connectivitycore.GroupInput{
		{ID: "api", Name: "API 次数站", URLs: []string{"https://api.fufuapi.top", "https://api.fufuapi.online", "https://api.fufuflower.top"}},
		{ID: "token", Name: "Token 站", URLs: []string{"https://token.fufuapi.top", "https://token.fufuapi.online", "https://token.fufuflower.top"}},
	})
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
