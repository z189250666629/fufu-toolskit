package activityapp

import (
	"errors"
	"strings"

	"fufu/newapi"
	"fufu/tokens"
)

// SetNewAPIRuntimeSite applies the unified admin NewAPI site used for card
// validation, credit issuance, and sale-card generation.
func SetNewAPIRuntimeSite(site newapi.Site) {
	site.URL = strings.TrimRight(strings.TrimSpace(site.URL), "/")
	site.Token = strings.TrimSpace(site.Token)
	if site.URL == "" || site.Token == "" {
		setTokenRuntime(nil, errors.New("NewAPI 未配置"))
		return
	}
	setTokenRuntime(tokens.NewService(newapi.NewClient(site)), nil)
}

// SetSubscriptionRuntimeSite applies the token-site NewAPI runtime used for
// subscription-based activity authentication and subscription prize crediting.
func SetSubscriptionRuntimeSite(site newapi.Site) {
	site = normalizeSubscriptionRuntimeSite(site)
	if site.URL == "" || site.Token == "" {
		setSubscriptionRuntime(newapi.Site{}, errors.New("Token 站未配置"))
		return
	}
	setSubscriptionRuntime(site, nil)
}
