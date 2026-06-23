package activityapp

import (
	"net/url"
	"strings"
	"sync"

	"fufu/newapi"
)

var subscriptionRuntimeMu sync.RWMutex
var subscriptionRuntimeSite newapi.Site
var subscriptionClient *newapi.Client
var subscriptionConfigErr error

func setSubscriptionRuntime(site newapi.Site, configErr error) {
	subscriptionRuntimeMu.Lock()
	defer subscriptionRuntimeMu.Unlock()

	subscriptionConfigErr = configErr
	if configErr != nil {
		subscriptionRuntimeSite = newapi.Site{}
		subscriptionClient = nil
		return
	}

	site = normalizeSubscriptionRuntimeSite(site)
	subscriptionRuntimeSite = site
	if site.URL == "" || site.Token == "" {
		subscriptionClient = nil
		return
	}
	subscriptionClient = newapi.NewClient(site)
}

func snapshotSubscriptionRuntime() (newapi.Site, *newapi.Client, error) {
	subscriptionRuntimeMu.RLock()
	defer subscriptionRuntimeMu.RUnlock()
	return subscriptionRuntimeSite, subscriptionClient, subscriptionConfigErr
}

func activitySubscriptionSite(sites []newapi.Site) (newapi.Site, bool) {
	bestIndex := -1
	bestScore := -1
	for i, site := range sites {
		site = normalizeSubscriptionRuntimeSite(site)
		if site.URL == "" || site.Token == "" {
			continue
		}
		score := subscriptionRuntimeSiteScore(site)
		if score < 0 {
			continue
		}
		if score > bestScore {
			bestIndex = i
			bestScore = score
		}
	}
	if bestIndex < 0 {
		return newapi.Site{}, false
	}
	return normalizeSubscriptionRuntimeSite(sites[bestIndex]), true
}

func subscriptionRuntimeSiteScore(site newapi.Site) int {
	category := normalizedSubscriptionRuntimeSiteField(site.Category)
	kind := normalizedSubscriptionRuntimeSiteField(site.Kind)
	name := normalizedSubscriptionRuntimeSiteField(site.Name)
	lineName := normalizedSubscriptionRuntimeSiteField(site.LineName)
	host := subscriptionRuntimeSiteHost(site.URL)

	switch {
	case name == "token-fufu":
		return 100
	case strings.Contains(name, "token-fufu"):
		return 90
	case category == "token":
		return 80
	case kind == "token":
		return 70
	case strings.Contains(name, "token"):
		return 60
	case strings.Contains(lineName, "token"):
		return 50
	case strings.Contains(host, "token"):
		return 40
	default:
		return -1
	}
}

func normalizedSubscriptionRuntimeSiteField(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func subscriptionRuntimeSiteHost(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return normalizedSubscriptionRuntimeSiteField(rawURL)
	}
	return normalizedSubscriptionRuntimeSiteField(parsed.Hostname())
}

func normalizeSubscriptionRuntimeSite(site newapi.Site) newapi.Site {
	site.URL = strings.TrimRight(strings.TrimSpace(site.URL), "/")
	site.Token = strings.TrimSpace(site.Token)
	site.UserID = strings.TrimSpace(site.UserID)
	return site
}
