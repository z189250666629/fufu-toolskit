package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"fufu/newapi"
)

func LoadPrimarySite(rootDir string) (newapi.Site, error) {
	if site, ok := fufuCombinePrimarySiteFromEnv(); ok {
		return site, nil
	}
	if site, ok := fufuAPIPrimarySiteFromEnv(); ok {
		return site, nil
	}
	if site, ok := managedSiteFromEnv(managedSiteEnvDef{
		Prefix:                   "NEWAPI_API_SITE",
		DefaultName:              "次数fufu",
		DefaultRatio:             "0.1",
		DefaultChannelListPath:   "/api/channel/search?keyword=&p=1&page_size=500",
		AllowAccessTokenFallback: true,
	}); ok {
		return site, nil
	}
	sites, managedMsg := LoadManagedSites(rootDir)
	if len(sites) > 0 {
		return sites[0], nil
	}
	var legacyErr error
	for _, path := range []string{filepath.Join(rootDir, "config.json")} {
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var cfg struct {
			Name, URL, Token, UserID string
			QuotaUnit                int64
		}
		if err := json.Unmarshal(raw, &cfg); err != nil {
			legacyErr = fmt.Errorf("%s 不是有效 JSON: %w", filepath.Base(path), err)
			continue
		}
		if NormalizeBaseURL(cfg.URL) == "" || strings.TrimSpace(cfg.Token) == "" {
			legacyErr = fmt.Errorf("%s 缺少有效的 URL 或 token", filepath.Base(path))
			continue
		}
		if cfg.UserID == "" {
			cfg.UserID = "1"
		}
		if cfg.QuotaUnit <= 0 {
			cfg.QuotaUnit = newapi.DefaultQuotaUnit
		}
		return newapi.Site{Name: stringOrDefault(cfg.Name, "次数fufu"), URL: NormalizeBaseURL(cfg.URL), Token: strings.TrimSpace(cfg.Token), UserID: cfg.UserID, QuotaUnit: cfg.QuotaUnit, Currency: "$", RechargeRatio: 1}, nil
	}
	if legacyErr != nil {
		return newapi.Site{}, legacyErr
	}
	if managedMsg != "" {
		return newapi.Site{}, fmt.Errorf("%s", managedMsg)
	}
	return newapi.Site{}, fmt.Errorf("missing NewAPI primary site config")
}

func fufuCombinePrimarySiteFromEnv() (newapi.Site, bool) {
	return primarySiteFromEnv(primarySiteEnvDef{
		URL:       Env("FUFU_COMBINE_API_URL"),
		Token:     Env("FUFU_COMBINE_API_TOKEN"),
		Name:      Env("FUFU_COMBINE_NAME"),
		UserID:    Env("FUFU_COMBINE_USER_ID"),
		QuotaUnit: Env("FUFU_COMBINE_QUOTA_UNIT"),
	})
}

func fufuAPIPrimarySiteFromEnv() (newapi.Site, bool) {
	return primarySiteFromEnv(primarySiteEnvDef{
		URL:       Env("FUFU_API_BASE_URL"),
		Token:     Env("FUFU_API_TOKEN"),
		Name:      Env("FUFU_COMBINE_NAME"),
		UserID:    Env("FUFU_API_USER_ID"),
		QuotaUnit: Env("FUFU_QUOTA_UNIT"),
	})
}

type primarySiteEnvDef struct {
	URL       string
	Token     string
	Name      string
	UserID    string
	QuotaUnit string
}

func primarySiteFromEnv(def primarySiteEnvDef) (newapi.Site, bool) {
	url := NormalizeBaseURL(def.URL)
	token := strings.TrimSpace(def.Token)
	if url == "" || token == "" {
		return newapi.Site{}, false
	}
	return newapi.Site{
		Name:          stringOrDefault(def.Name, "次数fufu"),
		URL:           url,
		Token:         token,
		UserID:        stringOrDefault(def.UserID, "1"),
		QuotaUnit:     int64Value(def.QuotaUnit, newapi.DefaultQuotaUnit),
		Currency:      "$",
		RechargeRatio: 1,
	}, true
}
