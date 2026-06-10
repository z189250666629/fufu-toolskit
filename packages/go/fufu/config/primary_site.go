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
