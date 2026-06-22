package activityapp

import (
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
		if !strings.EqualFold(strings.TrimSpace(site.Category), "token") {
			continue
		}
		site = normalizeSubscriptionRuntimeSite(site)
		if site.URL == "" || site.Token == "" {
			continue
		}
		score := 1
		name := strings.ToLower(strings.TrimSpace(site.Name))
		switch {
		case name == "token-fufu":
			score = 3
		case strings.Contains(name, "token-fufu"):
			score = 2
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

func normalizeSubscriptionRuntimeSite(site newapi.Site) newapi.Site {
	site.URL = strings.TrimRight(strings.TrimSpace(site.URL), "/")
	site.Token = strings.TrimSpace(site.Token)
	site.UserID = strings.TrimSpace(site.UserID)
	return site
}
