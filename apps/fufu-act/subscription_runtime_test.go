package activityapp

import (
	"testing"

	"fufu/newapi"
)

func TestActivitySubscriptionSiteAcceptsTokenFufuNameWithoutCategory(t *testing.T) {
	site, ok := activitySubscriptionSite([]newapi.Site{{
		Name:  "token-fufu",
		URL:   "https://token.example.test/",
		Token: "token-secret",
		Kind:  "api",
	}})
	if !ok {
		t.Fatal("expected token-fufu site to be selected")
	}
	if site.Name != "token-fufu" {
		t.Fatalf("site.Name=%q want %q", site.Name, "token-fufu")
	}
	if site.URL != "https://token.example.test" {
		t.Fatalf("site.URL=%q want trimmed URL", site.URL)
	}
}

func TestActivitySubscriptionSitePrefersExactTokenFufuName(t *testing.T) {
	site, ok := activitySubscriptionSite([]newapi.Site{
		{
			Name:     "subscription-main",
			Category: "token",
			URL:      "https://subs.example.test",
			Token:    "sub-token",
		},
		{
			Name:  "token-fufu",
			URL:   "https://token.example.test",
			Token: "token-secret",
		},
	})
	if !ok {
		t.Fatal("expected a token subscription site")
	}
	if site.Name != "token-fufu" {
		t.Fatalf("site.Name=%q want exact token-fufu winner", site.Name)
	}
}

func TestActivitySubscriptionSiteSkipsTokenLikeSiteWithoutCredentials(t *testing.T) {
	site, ok := activitySubscriptionSite([]newapi.Site{
		{
			Name: "token-fufu",
			URL:  "https://missing-token.example.test",
		},
		{
			Name:  "fallback-token-site",
			URL:   "https://fallback.example.test",
			Token: "fallback-token",
		},
	})
	if !ok {
		t.Fatal("expected fallback token-like site to be selected")
	}
	if site.Name != "fallback-token-site" {
		t.Fatalf("site.Name=%q want fallback token-like site", site.Name)
	}
}
