package main

import (
	"encoding/json"
	"fufu/config"
	"os"
	"strings"
)

func env(name string) string { return strings.TrimSpace(os.Getenv(name)) }

func splitList(v string) []string {
	parts := strings.FieldsFunc(v, func(r rune) bool { return r == ',' || r == '\n' || r == '\r' || r == '\t' })
	out := []string{}
	for _, p := range parts {
		if u := config.NormalizeBaseURL(p); u != "" {
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
			return nil, "CONNECTIVITY_TARGETS 不是有效 JSON: " + err.Error()
		}
		return arr, ""
	}
	groups := []map[string]any{}
	if urls := splitList(firstNonEmpty(env("CONNECTIVITY_API_URLS"), env("FUFU_API_URLS"), env("NEWAPI_API_SITE_URL"))); len(urls) > 0 {
		groups = append(groups, map[string]any{"id": "api", "name": firstNonEmpty(env("CONNECTIVITY_API_NAME"), "API 次数站"), "urls": urls})
	}
	if urls := splitList(firstNonEmpty(env("CONNECTIVITY_TOKEN_URLS"), env("FUFU_TOKEN_URLS"), env("NEWAPI_TOKEN_SITE_URL"))); len(urls) > 0 {
		groups = append(groups, map[string]any{"id": "token", "name": firstNonEmpty(env("CONNECTIVITY_TOKEN_NAME"), "Token 站"), "urls": urls})
	}
	if len(groups) > 0 {
		return groups, ""
	}
	return []map[string]any{
		{"id": "api", "name": "API 次数站", "urls": []string{"https://api.fufuapi.top", "https://api.fufuapi.online", "https://api.fufuflower.top"}},
		{"id": "token", "name": "Token 站", "urls": []string{"https://token.fufuapi.top", "https://token.fufuapi.online", "https://token.fufuflower.top"}},
	}, ""
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
