package config

import (
	"fmt"
	"strings"

	"fufu/newapi"
)

func LoadPrimarySite(rootDir string) (newapi.Site, error) {
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
	if managedMsg != "" {
		return newapi.Site{}, fmt.Errorf("%s", managedMsg)
	}
	return newapi.Site{}, fmt.Errorf("missing NewAPI primary site config")
}

func fufuAPIPrimarySiteFromEnv() (newapi.Site, bool) {
	return primarySiteFromEnv(primarySiteEnvDef{
		URL:       Env("FUFU_API_BASE_URL"),
		Token:     Env("FUFU_API_TOKEN"),
		Name:      Env("FUFU_API_NAME"),
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
