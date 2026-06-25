package config

import (
	"os"
	"path/filepath"
	"strings"
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

func TestLoadManagedSitesReportsExplicitConfigReadError(t *testing.T) {
	clearPrimaryEnv(t)
	root := t.TempDir()
	configPath := filepath.Join(root, "missing-managed-sites.json")
	t.Setenv("NEWAPI_MANAGED_API_CONFIG", configPath)

	sites, msg := LoadManagedSites(root)

	if len(sites) != 0 {
		t.Fatalf("sites = %#v", sites)
	}
	if !strings.Contains(msg, "missing-managed-sites.json") || msg == "" {
		t.Fatalf("expected explicit config read error, got %q", msg)
	}
}

func TestLoadManagedSitesResolvesRelativeExplicitConfigPathFromRoot(t *testing.T) {
	clearPrimaryEnv(t)
	root := t.TempDir()
	t.Chdir(t.TempDir())

	configPath := filepath.Join(root, "configs", "managed-sites.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte(`{"managedApiSites":[{"name":"root-site","url":"https://root.example.test","token":"root-token"}]}`), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("NEWAPI_MANAGED_API_CONFIG", filepath.Join("configs", "managed-sites.json"))

	sites, msg := LoadManagedSites(root)

	if msg != "" {
		t.Fatalf("msg = %q", msg)
	}
	if len(sites) != 1 || sites[0].Name != "root-site" || sites[0].URL != "https://root.example.test" {
		t.Fatalf("sites = %#v", sites)
	}
}

func TestLoadManagedSitesUsesConfigDirectoryBeforeLegacyRootFile(t *testing.T) {
	clearPrimaryEnv(t)
	root := t.TempDir()
	configDir := filepath.Join(root, "config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "newapi-managed-api-sites.json"), []byte(`{"managedApiSites":[{"name":"config-site","url":"https://config.example.test","token":"config-token"}]}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "newapi-managed-api-sites.json"), []byte(`{"managedApiSites":[{"name":"legacy-site","url":"https://legacy.example.test","token":"legacy-token"}]}`), 0644); err != nil {
		t.Fatal(err)
	}

	sites, msg := LoadManagedSites(root)

	if msg != "" {
		t.Fatalf("msg = %q", msg)
	}
	if len(sites) != 1 || sites[0].Name != "config-site" || sites[0].URL != "https://config.example.test" {
		t.Fatalf("sites = %#v", sites)
	}
}

func TestLoadManagedSitesKeepsLegacyRootFileFallback(t *testing.T) {
	clearPrimaryEnv(t)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "newapi-managed-api-sites.json"), []byte(`{"managedApiSites":[{"name":"legacy-site","url":"https://legacy.example.test","token":"legacy-token"}]}`), 0644); err != nil {
		t.Fatal(err)
	}

	sites, msg := LoadManagedSites(root)

	if msg != "" {
		t.Fatalf("msg = %q", msg)
	}
	if len(sites) != 1 || sites[0].Name != "legacy-site" || sites[0].URL != "https://legacy.example.test" {
		t.Fatalf("sites = %#v", sites)
	}
}

func TestLoadManagedSitesReportsInlineShapeError(t *testing.T) {
	clearPrimaryEnv(t)
	t.Setenv("NEWAPI_MANAGED_API_SITES", `{"managedApiSites":{}}`)

	sites, msg := LoadManagedSites(t.TempDir())

	if len(sites) != 0 {
		t.Fatalf("sites = %#v", sites)
	}
	if !strings.Contains(msg, "配置文件格式无效") {
		t.Fatalf("expected shape error, got %q", msg)
	}
}

func TestLoadManagedSitesReportsNonObjectEntries(t *testing.T) {
	clearPrimaryEnv(t)
	t.Setenv("NEWAPI_MANAGED_API_SITES", `["bad"]`)

	sites, msg := LoadManagedSites(t.TempDir())

	if len(sites) != 0 {
		t.Fatalf("sites = %#v", sites)
	}
	if !strings.Contains(msg, "配置文件格式无效") || !strings.Contains(msg, "第 1 项") {
		t.Fatalf("expected non-object item error, got %q", msg)
	}
}
