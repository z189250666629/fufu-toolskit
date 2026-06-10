package config

import (
	"testing"

	"fufu/newapi"
)

func TestManagedSiteFromEnvBuildsDefaultsAndFallbacks(t *testing.T) {
	t.Setenv("NEWAPI_SAMPLE_URL", "https://sample.example.test/")
	t.Setenv("NEWAPI_SAMPLE_ACCESS_TOKEN", "sample-token")
	t.Setenv("NEWAPI_SAMPLE_QUOTA_UNIT", "700000")
	t.Setenv("NEWAPI_SAMPLE_SKIP_USER_HEADER", "true")

	site, ok := managedSiteFromEnv(managedSiteEnvDef{
		Prefix:                   "NEWAPI_SAMPLE",
		DefaultName:              "sample-default",
		DefaultRatio:             "0.5",
		DefaultChannelListPath:   "/api/channel/search?keyword=&p=1&page_size=500",
		AllowAccessTokenFallback: true,
	})

	if !ok {
		t.Fatalf("expected env site to be configured")
	}
	if site.Name != "sample-default" || site.URL != "https://sample.example.test" || site.Token != "sample-token" {
		t.Fatalf("bad site identity: %#v", site)
	}
	if site.UserID != "1" || site.Kind != "api" || site.Currency != "$" {
		t.Fatalf("bad site defaults: %#v", site)
	}
	if site.QuotaUnit != 700000 || site.RechargeRatio != 0.5 {
		t.Fatalf("bad numeric defaults: %#v", site)
	}
	if !site.SkipUserHeader || site.ChannelListEndpoint != "/api/channel/search?keyword=&p=1&page_size=500" {
		t.Fatalf("bad optional fields: %#v", site)
	}
}

func TestManagedSiteFromEnvRequiresURLAndToken(t *testing.T) {
	t.Setenv("NEWAPI_MISSING_URL", "https://missing.example.test")

	if site, ok := managedSiteFromEnv(managedSiteEnvDef{Prefix: "NEWAPI_MISSING"}); ok || site != (newapi.Site{}) {
		t.Fatalf("expected missing token to skip site, got ok=%v site=%#v", ok, site)
	}
}

func TestManagedSiteFromEnvRejectsUnsupportedKind(t *testing.T) {
	t.Setenv("NEWAPI_SAMPLE_URL", "https://sample.example.test")
	t.Setenv("NEWAPI_SAMPLE_TOKEN", "sample-token")
	t.Setenv("NEWAPI_SAMPLE_KIND", "token")

	if site, ok := managedSiteFromEnv(managedSiteEnvDef{Prefix: "NEWAPI_SAMPLE"}); ok || site != (newapi.Site{}) {
		t.Fatalf("expected unsupported kind to skip site, got ok=%v site=%#v", ok, site)
	}
}

func TestManagedSiteFromEnvAllowsManagedAPIKindAlias(t *testing.T) {
	t.Setenv("NEWAPI_SAMPLE_URL", "https://sample.example.test")
	t.Setenv("NEWAPI_SAMPLE_TOKEN", "sample-token")
	t.Setenv("NEWAPI_SAMPLE_KIND", "managed_api")

	site, ok := managedSiteFromEnv(managedSiteEnvDef{Prefix: "NEWAPI_SAMPLE"})
	if !ok || site.Kind != "managed_api" {
		t.Fatalf("expected managed_api kind, got ok=%v site=%#v", ok, site)
	}
}
