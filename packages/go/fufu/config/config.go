package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"fufu/newapi"
)

func Env(name string) string { return strings.TrimSpace(os.Getenv(name)) }

func NormalizeBaseURL(value string) string {
	value = strings.TrimRight(strings.TrimSpace(value), "/")
	if value == "" {
		return ""
	}
	if !strings.HasPrefix(value, "http://") && !strings.HasPrefix(value, "https://") {
		return ""
	}
	return value
}

func boolValue(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		v = strings.TrimSpace(strings.ToLower(v))
		return v == "1" || v == "true" || v == "yes" || v == "on"
	default:
		return false
	}
}

func int64Value(value any, fallback int64) int64 {
	switch v := value.(type) {
	case json.Number:
		if n, err := v.Int64(); err == nil && n > 0 {
			return n
		}
	case float64:
		if v > 0 {
			return int64(v)
		}
	case int64:
		if v > 0 {
			return v
		}
	case int:
		if v > 0 {
			return int64(v)
		}
	case string:
		if n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}

func floatValue(value any, fallback float64) float64 {
	switch v := value.(type) {
	case json.Number:
		if n, err := v.Float64(); err == nil && n > 0 {
			return n
		}
	case float64:
		if v > 0 {
			return v
		}
	case int:
		if v > 0 {
			return float64(v)
		}
	case string:
		if n, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}

func stringValue(item map[string]any, names ...string) string {
	for _, name := range names {
		if v, ok := item[name]; ok {
			if s := strings.TrimSpace(fmt.Sprint(v)); s != "" && s != "<nil>" {
				return s
			}
		}
	}
	return ""
}

func tokenValue(item map[string]any) string {
	if tokenEnv := stringValue(item, "tokenEnv", "token_env", "accessTokenEnv", "access_token_env"); tokenEnv != "" {
		if token := Env(tokenEnv); token != "" {
			return token
		}
	}
	return stringValue(item, "token", "accessToken", "access_token")
}

func coerceItems(data any) []map[string]any {
	if obj, ok := data.(map[string]any); ok {
		for _, key := range []string{"managedApiSites", "managedSites", "managed_sites", "sources", "admin_instances", "instances"} {
			if arr, ok := obj[key].([]any); ok {
				return toMaps(arr)
			}
		}
	}
	if arr, ok := data.([]any); ok {
		return toMaps(arr)
	}
	return nil
}

func toMaps(arr []any) []map[string]any {
	items := []map[string]any{}
	for _, item := range arr {
		if obj, ok := item.(map[string]any); ok {
			items = append(items, obj)
		}
	}
	return items
}

func NormalizeManagedSites(data any, allowedNames map[string]bool) ([]newapi.Site, string) {
	items := coerceItems(data)
	seen := map[string]bool{}
	sites := []newapi.Site{}
	for _, item := range items {
		name := stringValue(item, "name")
		if name == "" || (allowedNames != nil && !allowedNames[name]) || seen[name] {
			continue
		}
		url := NormalizeBaseURL(stringValue(item, "url"))
		token := tokenValue(item)
		if url == "" || token == "" {
			continue
		}
		kind := strings.ToLower(stringValue(item, "kind", "role", "siteType", "site_type"))
		if kind == "" {
			kind = "api"
		}
		if kind != "api" && kind != "managed-api" && kind != "managed_api" && kind != "admin" {
			continue
		}
		seen[name] = true
		sites = append(sites, newapi.Site{
			Name:                name,
			URL:                 url,
			Token:               token,
			UserID:              stringOrDefault(stringValue(item, "userId", "user_id"), "1"),
			Kind:                kind,
			SkipUserHeader:      boolValue(item["skipUserHeader"]) || boolValue(item["skip_user_header"]),
			QuotaUnit:           int64Value(first(item, "quotaUnit", "quota_unit"), newapi.DefaultQuotaUnit),
			Currency:            stringOrDefault(stringValue(item, "currency"), "$"),
			RechargeRatio:       floatValue(first(item, "rechargeRatio", "recharge_ratio", "exchangeRate", "exchange_rate"), 1),
			ChannelListEndpoint: stringValue(item, "channelListEndpoint", "channel_list_endpoint"),
			Note:                stringValue(item, "note"),
		})
	}
	if len(sites) == 0 && len(items) > 0 {
		return nil, "配置文件中没有可用的 API 站点"
	}
	return sites, ""
}

func first(item map[string]any, keys ...string) any {
	for _, key := range keys {
		if v, ok := item[key]; ok {
			return v
		}
	}
	return nil
}
func stringOrDefault(v, fallback string) string {
	if strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return fallback
}

func DeploymentSitesFromEnv() []newapi.Site {
	defs := []struct{ Prefix, Name, URL, Ratio string }{
		{"NEWAPI_API_SITE", "次数fufu", Env("NEWAPI_API_SITE_URL"), "0.1"},
		{"NEWAPI_TOKEN_SITE", "token-fufu", Env("NEWAPI_TOKEN_SITE_URL"), "1"},
	}
	var sites []newapi.Site
	for _, def := range defs {
		url := NormalizeBaseURL(Env(def.Prefix + "_URL"))
		if url == "" {
			url = NormalizeBaseURL(def.URL)
		}
		token := Env(def.Prefix + "_TOKEN")
		if token == "" {
			token = Env(def.Prefix + "_ACCESS_TOKEN")
		}
		if url == "" || token == "" {
			continue
		}
		sites = append(sites, newapi.Site{Name: stringOrDefault(Env(def.Prefix+"_NAME"), def.Name), URL: url, Token: token, UserID: stringOrDefault(Env(def.Prefix+"_USER_ID"), "1"), Kind: stringOrDefault(Env(def.Prefix+"_KIND"), "api"), QuotaUnit: int64Value(Env(def.Prefix+"_QUOTA_UNIT"), newapi.DefaultQuotaUnit), Currency: stringOrDefault(Env(def.Prefix+"_CURRENCY"), "$"), RechargeRatio: floatValue(stringOrDefault(Env(def.Prefix+"_RECHARGE_RATIO"), def.Ratio), 1), ChannelListEndpoint: stringOrDefault(Env(def.Prefix+"_CHANNEL_LIST_ENDPOINT"), "/api/channel/search?keyword=&p=1&page_size=500"), SkipUserHeader: boolValue(Env(def.Prefix + "_SKIP_USER_HEADER")), Note: Env(def.Prefix + "_NOTE")})
	}
	for i := 1; i <= 10; i++ {
		prefix := fmt.Sprintf("NEWAPI_MANAGED_SITE_%d", i)
		url := NormalizeBaseURL(Env(prefix + "_URL"))
		token := Env(prefix + "_TOKEN")
		if url == "" || token == "" {
			continue
		}
		sites = append(sites, newapi.Site{Name: stringOrDefault(Env(prefix+"_NAME"), fmt.Sprintf("managed-site-%d", i)), URL: url, Token: token, UserID: stringOrDefault(Env(prefix+"_USER_ID"), "1"), Kind: stringOrDefault(Env(prefix+"_KIND"), "api"), QuotaUnit: int64Value(Env(prefix+"_QUOTA_UNIT"), newapi.DefaultQuotaUnit), Currency: stringOrDefault(Env(prefix+"_CURRENCY"), "$"), RechargeRatio: floatValue(Env(prefix+"_RECHARGE_RATIO"), 1), ChannelListEndpoint: Env(prefix + "_CHANNEL_LIST_ENDPOINT"), SkipUserHeader: boolValue(Env(prefix + "_SKIP_USER_HEADER")), Note: Env(prefix + "_NOTE")})
	}
	return sites
}

func LoadManagedSites(rootDir string) ([]newapi.Site, string) {
	if inline := Env("NEWAPI_MANAGED_API_SITES"); inline != "" {
		var data any
		dec := json.NewDecoder(strings.NewReader(inline))
		dec.UseNumber()
		if err := dec.Decode(&data); err != nil {
			return nil, "NEWAPI_MANAGED_API_SITES 不是有效 JSON: " + err.Error()
		}
		return NormalizeManagedSites(data, nil)
	}
	if sites := DeploymentSitesFromEnv(); len(sites) > 0 {
		return sites, ""
	}
	candidates := []string{}
	if configured := Env("NEWAPI_MANAGED_API_CONFIG"); configured != "" {
		candidates = append(candidates, configured)
	} else {
		candidates = append(candidates, filepath.Join(rootDir, "newapi-managed-api-sites.json"))
		if home, err := os.UserHomeDir(); err == nil {
			candidates = append(candidates, filepath.Join(home, "Downloads", "newapi-manager-config-2026-05-06.json"))
		}
	}
	var lastErr string
	for _, path := range candidates {
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var data any
		dec := json.NewDecoder(strings.NewReader(string(raw)))
		dec.UseNumber()
		if err := dec.Decode(&data); err != nil {
			lastErr = filepath.Base(path) + " 不是有效 JSON: " + err.Error()
			continue
		}
		sites, msg := NormalizeManagedSites(data, nil)
		if len(sites) > 0 || Env("NEWAPI_MANAGED_API_CONFIG") != "" {
			return sites, msg
		}
		lastErr = msg
	}
	return nil, lastErr
}

func LoadPrimarySite(rootDir string) (newapi.Site, error) {
	url := NormalizeBaseURL(Env("FUFU_COMBINE_API_URL"))
	if url == "" {
		url = NormalizeBaseURL(Env("FUFU_API_BASE_URL"))
		if url == "" {
			url = NormalizeBaseURL(Env("NEWAPI_API_SITE_URL"))
		}
	}
	token := Env("FUFU_COMBINE_API_TOKEN")
	if token == "" {
		token = Env("FUFU_API_TOKEN")
		if token == "" {
			token = Env("NEWAPI_API_SITE_TOKEN")
		}
	}
	if url != "" && token != "" {
		return newapi.Site{Name: stringOrDefault(Env("FUFU_COMBINE_NAME"), "次数fufu"), URL: url, Token: token, UserID: stringOrDefault(Env("FUFU_COMBINE_USER_ID"), stringOrDefault(Env("FUFU_API_USER_ID"), "1")), QuotaUnit: int64Value(stringOrDefault(Env("FUFU_COMBINE_QUOTA_UNIT"), Env("FUFU_QUOTA_UNIT")), newapi.DefaultQuotaUnit), Currency: "$", RechargeRatio: 1}, nil
	}
	if sites, _ := LoadManagedSites(rootDir); len(sites) > 0 {
		return sites[0], nil
	}
	for _, path := range []string{filepath.Join(rootDir, "config.json")} {
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var cfg struct {
			Name, URL, Token, UserID string
			QuotaUnit                int64
		}
		if json.Unmarshal(raw, &cfg) == nil && NormalizeBaseURL(cfg.URL) != "" && strings.TrimSpace(cfg.Token) != "" {
			if cfg.UserID == "" {
				cfg.UserID = "1"
			}
			if cfg.QuotaUnit <= 0 {
				cfg.QuotaUnit = newapi.DefaultQuotaUnit
			}
			return newapi.Site{Name: stringOrDefault(cfg.Name, "次数fufu"), URL: NormalizeBaseURL(cfg.URL), Token: strings.TrimSpace(cfg.Token), UserID: cfg.UserID, QuotaUnit: cfg.QuotaUnit, Currency: "$", RechargeRatio: 1}, nil
		}
	}
	return newapi.Site{}, fmt.Errorf("missing NewAPI primary site config")
}
