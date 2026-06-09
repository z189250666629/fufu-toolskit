package config

import (
	"os"
	"path/filepath"
	"testing"

	"fufu/newapi"
)

func TestNormalizeManagedSitesFiltersAndDefaults(t *testing.T) {
	t.Setenv("SITE_ONE_TOKEN", "secret-from-env")

	data := map[string]any{
		"managedApiSites": []any{
			map[string]any{
				"name":          "primary",
				"url":           "https://api.example.test/",
				"tokenEnv":      "SITE_ONE_TOKEN",
				"quotaUnit":     "600000",
				"rechargeRatio": "0.25",
			},
			map[string]any{
				"name":  "primary",
				"url":   "https://duplicate.example.test",
				"token": "duplicate",
			},
			map[string]any{
				"name":  "bad-kind",
				"url":   "https://bad.example.test",
				"token": "bad",
				"kind":  "token",
			},
		},
	}

	sites, msg := NormalizeManagedSites(data, map[string]bool{"primary": true, "bad-kind": true})
	if msg != "" {
		t.Fatalf("unexpected message: %s", msg)
	}
	if len(sites) != 1 {
		t.Fatalf("sites = %#v", sites)
	}
	got := sites[0]
	if got.Name != "primary" || got.URL != "https://api.example.test" || got.Token != "secret-from-env" {
		t.Fatalf("bad normalized site: %#v", got)
	}
	if got.UserID != "1" || got.Kind != "api" || got.Currency != "$" {
		t.Fatalf("defaults not applied: %#v", got)
	}
	if got.QuotaUnit != 600000 || got.RechargeRatio != 0.25 {
		t.Fatalf("numeric values not parsed: %#v", got)
	}
}

func TestDeploymentSitesFromEnv(t *testing.T) {
	t.Setenv("NEWAPI_API_SITE_URL", "https://api.example.test/")
	t.Setenv("NEWAPI_API_SITE_TOKEN", "api-token")
	t.Setenv("NEWAPI_TOKEN_SITE_URL", "https://token.example.test")
	t.Setenv("NEWAPI_TOKEN_SITE_TOKEN", "token-token")
	t.Setenv("NEWAPI_MANAGED_SITE_1_NAME", "extra")
	t.Setenv("NEWAPI_MANAGED_SITE_1_URL", "https://extra.example.test")
	t.Setenv("NEWAPI_MANAGED_SITE_1_TOKEN", "extra-token")

	sites := DeploymentSitesFromEnv()
	if len(sites) != 3 {
		t.Fatalf("sites = %#v", sites)
	}
	if sites[0].Name != "次数fufu" || sites[0].URL != "https://api.example.test" || sites[0].RechargeRatio != 0.1 {
		t.Fatalf("bad API site defaults: %#v", sites[0])
	}
	if sites[1].Name != "token-fufu" || sites[1].RechargeRatio != 1 {
		t.Fatalf("bad token site defaults: %#v", sites[1])
	}
	if sites[2].Name != "extra" || sites[2].Token != "extra-token" {
		t.Fatalf("bad managed site: %#v", sites[2])
	}
}

func TestLoadPrimarySitePrefersEnvThenConfigFile(t *testing.T) {
	t.Run("env", func(t *testing.T) {
		t.Setenv("FUFU_COMBINE_API_URL", "https://combine.example.test/")
		t.Setenv("FUFU_COMBINE_API_TOKEN", "combine-token")
		t.Setenv("FUFU_COMBINE_USER_ID", "9")
		t.Setenv("FUFU_COMBINE_QUOTA_UNIT", "700000")

		site, err := LoadPrimarySite(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		if site.URL != "https://combine.example.test" || site.Token != "combine-token" || site.UserID != "9" || site.QuotaUnit != 700000 {
			t.Fatalf("bad env primary site: %#v", site)
		}
	})

	t.Run("config file", func(t *testing.T) {
		clearPrimaryEnv(t)
		root := t.TempDir()
		t.Setenv("NEWAPI_MANAGED_API_CONFIG", filepath.Join(root, "missing-managed-sites.json"))
		path := filepath.Join(root, "config.json")
		if err := os.WriteFile(path, []byte(`{"name":"file-site","url":"https://file.example.test/","token":"file-token"}`), 0644); err != nil {
			t.Fatal(err)
		}

		site, err := LoadPrimarySite(root)
		if err != nil {
			t.Fatal(err)
		}
		if site.Name != "file-site" || site.URL != "https://file.example.test" || site.Token != "file-token" {
			t.Fatalf("bad file primary site: %#v", site)
		}
		if site.UserID != "1" || site.QuotaUnit != newapi.DefaultQuotaUnit {
			t.Fatalf("file defaults not applied: %#v", site)
		}
	})
}

func clearPrimaryEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		"FUFU_COMBINE_API_URL",
		"FUFU_COMBINE_API_TOKEN",
		"FUFU_COMBINE_USER_ID",
		"FUFU_COMBINE_QUOTA_UNIT",
		"FUFU_COMBINE_NAME",
		"FUFU_API_BASE_URL",
		"FUFU_API_TOKEN",
		"FUFU_API_USER_ID",
		"FUFU_QUOTA_UNIT",
		"NEWAPI_API_SITE_URL",
		"NEWAPI_API_SITE_TOKEN",
		"NEWAPI_API_SITE_ACCESS_TOKEN",
		"NEWAPI_TOKEN_SITE_URL",
		"NEWAPI_TOKEN_SITE_TOKEN",
		"NEWAPI_TOKEN_SITE_ACCESS_TOKEN",
		"NEWAPI_MANAGED_API_SITES",
		"NEWAPI_MANAGED_API_CONFIG",
	} {
		t.Setenv(name, "")
	}
	for i := 1; i <= 10; i++ {
		prefix := "NEWAPI_MANAGED_SITE_" + string(rune('0'+i))
		if i == 10 {
			prefix = "NEWAPI_MANAGED_SITE_10"
		}
		t.Setenv(prefix+"_URL", "")
		t.Setenv(prefix+"_TOKEN", "")
	}
}
