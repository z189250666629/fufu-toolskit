package config

import (
	"fmt"
	"os"
	"path/filepath"

	"fufu/newapi"
)

func tokenValue(item map[string]any) string {
	if tokenEnv := stringValue(item, "tokenEnv", "token_env", "accessTokenEnv", "access_token_env"); tokenEnv != "" {
		if token := Env(tokenEnv); token != "" {
			return token
		}
	}
	return stringValue(item, "token", "accessToken", "access_token")
}

func coerceItems(data any) []map[string]any {
	items, _ := coerceItemsWithMessage(data)
	return items
}

func coerceItemsWithMessage(data any) ([]map[string]any, string) {
	if obj, ok := data.(map[string]any); ok {
		for _, key := range []string{"managedApiSites", "managedSites", "managed_sites", "sources", "admin_instances", "instances"} {
			value, exists := obj[key]
			if !exists {
				continue
			}
			arr, ok := value.([]any)
			if !ok {
				return nil, "配置文件格式无效: " + key + " 应为数组"
			}
			return toMaps(arr)
		}
		return nil, "配置文件格式无效: 缺少 managedApiSites 数组"
	}
	if arr, ok := data.([]any); ok {
		return toMaps(arr)
	}
	return nil, "配置文件格式无效: 根节点应为数组或对象"
}

func toMaps(arr []any) ([]map[string]any, string) {
	items := []map[string]any{}
	for i, item := range arr {
		obj, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Sprintf("配置文件格式无效: 第 %d 项应为对象", i+1)
		}
		items = append(items, obj)
	}
	return items, ""
}

func NormalizeManagedSites(data any, allowedNames map[string]bool) ([]newapi.Site, string) {
	items, formatMsg := coerceItemsWithMessage(data)
	if formatMsg != "" {
		return nil, formatMsg
	}
	seen := map[string]bool{}
	sites := []newapi.Site{}
	for _, item := range items {
		name := stringValue(item, "name")
		if name == "" || (allowedNames != nil && !allowedNames[name]) || seen[name] {
			continue
		}
		site, ok := normalizeManagedSiteItem(item)
		if !ok {
			continue
		}
		seen[name] = true
		sites = append(sites, site)
	}
	if len(sites) == 0 && len(items) > 0 {
		return nil, "配置文件中没有可用的 API 站点"
	}
	return sites, ""
}

func DeploymentSitesFromEnv() []newapi.Site {
	defs := []managedSiteEnvDef{
		{
			Prefix:                   "NEWAPI_API_SITE",
			DefaultName:              "次数fufu",
			DefaultURL:               Env("NEWAPI_API_SITE_URL"),
			DefaultRatio:             "0.1",
			DefaultChannelListPath:   "/api/channel/search?keyword=&p=1&page_size=500",
			AllowAccessTokenFallback: true,
		},
		{
			Prefix:                   "NEWAPI_TOKEN_SITE",
			DefaultName:              "token-fufu",
			DefaultURL:               Env("NEWAPI_TOKEN_SITE_URL"),
			DefaultRatio:             "1",
			DefaultChannelListPath:   "/api/channel/search?keyword=&p=1&page_size=500",
			AllowAccessTokenFallback: true,
		},
	}
	var sites []newapi.Site
	for _, def := range defs {
		if site, ok := managedSiteFromEnv(def); ok {
			sites = append(sites, site)
		}
	}
	for i := 1; i <= 10; i++ {
		prefix := fmt.Sprintf("NEWAPI_MANAGED_SITE_%d", i)
		if site, ok := managedSiteFromEnv(managedSiteEnvDef{
			Prefix:       prefix,
			DefaultName:  fmt.Sprintf("managed-site-%d", i),
			DefaultRatio: "",
		}); ok {
			sites = append(sites, site)
		}
	}
	return sites
}

func LoadManagedSites(rootDir string) ([]newapi.Site, string) {
	if inline := Env("NEWAPI_MANAGED_API_SITES"); inline != "" {
		data, err := decodeManagedSitesJSON(inline)
		if err != nil {
			return nil, "NEWAPI_MANAGED_API_SITES 不是有效 JSON: " + err.Error()
		}
		return NormalizeManagedSites(data, nil)
	}
	if sites := DeploymentSitesFromEnv(); len(sites) > 0 {
		return sites, ""
	}
	candidates := []string{}
	configured := Env("NEWAPI_MANAGED_API_CONFIG")
	if configured != "" {
		if !filepath.IsAbs(configured) && rootDir != "" {
			configured = filepath.Join(rootDir, configured)
		}
		candidates = append(candidates, configured)
	} else {
		candidates = append(
			candidates,
			filepath.Join(rootDir, "config", "newapi-managed-api-sites.json"),
			filepath.Join(rootDir, "newapi-managed-api-sites.json"),
		)
		if home, err := os.UserHomeDir(); err == nil {
			candidates = append(candidates, filepath.Join(home, "Downloads", "newapi-manager-config-2026-05-06.json"))
		}
	}
	var lastErr string
	for _, path := range candidates {
		raw, err := os.ReadFile(path)
		if err != nil {
			if configured != "" {
				return nil, filepath.Base(path) + " 读取失败: " + err.Error()
			}
			continue
		}
		data, err := decodeManagedSitesJSON(string(raw))
		if err != nil {
			lastErr = filepath.Base(path) + " 不是有效 JSON: " + err.Error()
			continue
		}
		sites, msg := NormalizeManagedSites(data, nil)
		if len(sites) > 0 || configured != "" {
			return sites, msg
		}
		lastErr = msg
	}
	return nil, lastErr
}
