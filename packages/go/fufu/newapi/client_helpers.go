package newapi

import (
	"net/http"
	"strings"
)

func normalizeSite(site Site) Site {
	site.URL = strings.TrimRight(strings.TrimSpace(site.URL), "/")
	site.Token = strings.TrimSpace(site.Token)
	site.UserID = strings.TrimSpace(site.UserID)
	site.Currency = strings.TrimSpace(site.Currency)
	if site.UserID == "" {
		site.UserID = "1"
	}
	if site.QuotaUnit <= 0 {
		site.QuotaUnit = DefaultQuotaUnit
	}
	if site.Currency == "" {
		site.Currency = "$"
	}
	if site.RechargeRatio <= 0 {
		site.RechargeRatio = 1
	}
	return site
}

func requestEndpoint(endpoint string) string {
	if strings.HasPrefix(endpoint, "/") {
		return endpoint
	}
	return "/" + endpoint
}

func shouldSetJSONContentType(method string, body any) bool {
	if body == nil {
		return false
	}
	upper := strings.ToUpper(method)
	return upper != http.MethodGet && upper != http.MethodHead
}
