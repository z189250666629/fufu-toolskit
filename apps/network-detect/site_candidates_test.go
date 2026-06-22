package main

import (
	"path/filepath"
	"testing"

	"fufu/newapi"
)

func TestExpandSitesForModelStatusExpandsStandardFufuSiteAcrossConnectivityOverrides(t *testing.T) {
	oldRootDir := rootDir
	t.Cleanup(func() { rootDir = oldRootDir })
	rootDir = t.TempDir()
	clearManagedSiteEnv(t)
	clearConnectivityEnv(t)
	t.Setenv("NEWAPI_MANAGED_API_CONFIG", filepath.Join(rootDir, "missing-managed-sites.json"))
	t.Setenv("CONNECTIVITY_API_URLS", "https://api-a.example.test, https://api-b.example.test")

	expanded := expandSitesForModelStatus([]newapi.Site{
		{Name: "次数fufu", Category: "api", URL: "https://api-a.example.test", Token: "token", UserID: "1"},
	})

	if len(expanded) != 2 {
		t.Fatalf("expanded sites = %#v", expanded)
	}
	if expanded[0].URL != "https://api-a.example.test" || expanded[1].URL != "https://api-b.example.test" {
		t.Fatalf("expanded URLs = %#v", []string{expanded[0].URL, expanded[1].URL})
	}
	if expanded[0].Name != "次数fufu" || expanded[1].Name != "次数fufu" {
		t.Fatalf("expanded names = %#v", []string{expanded[0].Name, expanded[1].Name})
	}
}

func TestOrderedManualTestSitesMovesPreferredURLFirst(t *testing.T) {
	oldRootDir := rootDir
	t.Cleanup(func() { rootDir = oldRootDir })
	rootDir = t.TempDir()
	clearManagedSiteEnv(t)
	clearConnectivityEnv(t)
	t.Setenv("NEWAPI_MANAGED_API_CONFIG", filepath.Join(rootDir, "missing-managed-sites.json"))
	t.Setenv("CONNECTIVITY_API_URLS", "https://api-a.example.test,https://api-b.example.test")

	ordered := orderedManualTestSites(
		newapi.Site{Name: "次数fufu", Category: "api", URL: "https://api-a.example.test", Token: "token", UserID: "1"},
		"https://api-b.example.test/v1/models",
	)

	if len(ordered) != 2 {
		t.Fatalf("ordered sites = %#v", ordered)
	}
	if ordered[0].URL != "https://api-b.example.test" || ordered[1].URL != "https://api-a.example.test" {
		t.Fatalf("ordered URLs = %#v", []string{ordered[0].URL, ordered[1].URL})
	}
}

func TestExpandSitesForModelStatusDoesNotExpandNonPublicPrivateSite(t *testing.T) {
	expanded := expandSitesForModelStatus([]newapi.Site{
		{Name: "private-site", Category: "api", URL: "http://127.0.0.1:3000", Token: "token", UserID: "1"},
	})

	if len(expanded) != 1 {
		t.Fatalf("expanded sites = %#v", expanded)
	}
	if expanded[0].URL != "http://127.0.0.1:3000" {
		t.Fatalf("expanded URL = %q", expanded[0].URL)
	}
}
