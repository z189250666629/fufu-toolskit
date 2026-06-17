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
		tokenSvc = nil
		tokenConfigErr = errors.New("NewAPI 未配置")
		return
	}
	tokenSvc = tokens.NewService(newapi.NewClient(site))
	tokenConfigErr = nil
}
