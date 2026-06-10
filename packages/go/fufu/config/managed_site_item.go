package config

import (
	"strings"

	"fufu/newapi"
)

func normalizeManagedSiteItem(item map[string]any) (newapi.Site, bool) {
	name := stringValue(item, "name")
	if name == "" {
		return newapi.Site{}, false
	}
	url := NormalizeBaseURL(stringValue(item, "url"))
	token := tokenValue(item)
	if url == "" || token == "" {
		return newapi.Site{}, false
	}
	kind, ok := normalizeManagedSiteKind(stringValue(item, "kind", "role", "siteType", "site_type"))
	if !ok {
		return newapi.Site{}, false
	}
	return newapi.Site{
		Name:                name,
		URL:                 url,
		Token:               token,
		UserID:              stringOrDefault(stringValue(item, "userId", "user_id"), "1"),
		Kind:                kind,
		SkipUserHeader:      boolValue(item["skipUserHeader"]) || boolValue(item["skip_user_header"]),
		QuotaUnit:           int64Value(first(item, "quotaUnit", "quota_unit"), newapi.DefaultQuotaUnit),
		Currency:            stringOrDefault(stringValue(item, "currency"), "$"),
		RechargeRatio:       floatValue(first(item, "rechargeRatio", "recharge_ratio", "exchangeRate", "exchange_rate"), 1),
		ChannelListEndpoint: stringValue(item, "channelListEndpoint", "channel_list_endpoint"),
		Note:                stringValue(item, "note"),
	}, true
}

func normalizeManagedSiteKind(kind string) (string, bool) {
	kind = strings.ToLower(strings.TrimSpace(kind))
	if kind == "" {
		return "api", true
	}
	switch kind {
	case "api", "managed-api", "managed_api", "admin":
		return kind, true
	default:
		return "", false
	}
}
