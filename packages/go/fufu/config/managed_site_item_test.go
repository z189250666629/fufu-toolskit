package config

import (
	"testing"

	"fufu/newapi"
)

func TestNormalizeManagedSiteItemBuildsSiteFromAliases(t *testing.T) {
	t.Setenv("ITEM_TOKEN", "token-from-env")

	site, ok := normalizeManagedSiteItem(map[string]any{
		"name":                  "primary",
		"url":                   "https://api.example.test/",
		"access_token_env":      "ITEM_TOKEN",
		"role":                  "managed-api",
		"user_id":               "9",
		"skip_user_header":      "true",
		"quota_unit":            "700000",
		"exchange_rate":         "0.25",
		"channel_list_endpoint": "/api/channel/search",
		"note":                  "prod",
	})

	if !ok {
		t.Fatalf("expected item to normalize")
	}
	if site.Name != "primary" || site.URL != "https://api.example.test" || site.Token != "token-from-env" {
		t.Fatalf("identity = %#v", site)
	}
	if site.Kind != "managed-api" || site.UserID != "9" || !site.SkipUserHeader {
		t.Fatalf("settings = %#v", site)
	}
	if site.QuotaUnit != 700000 || site.RechargeRatio != 0.25 || site.Currency != "$" {
		t.Fatalf("numeric defaults = %#v", site)
	}
	if site.ChannelListEndpoint != "/api/channel/search" || site.Note != "prod" {
		t.Fatalf("metadata = %#v", site)
	}
}

func TestNormalizeManagedSiteItemSkipsInvalidInputs(t *testing.T) {
	for _, item := range []map[string]any{
		{"name": "", "url": "https://api.example.test", "token": "token"},
		{"name": "missing-url", "token": "token"},
		{"name": "missing-token", "url": "https://api.example.test"},
		{"name": "bad-kind", "url": "https://api.example.test", "token": "token", "kind": "token"},
	} {
		if site, ok := normalizeManagedSiteItem(item); ok || site != (newapi.Site{}) {
			t.Fatalf("expected invalid item to skip, got ok=%v site=%#v", ok, site)
		}
	}
}
