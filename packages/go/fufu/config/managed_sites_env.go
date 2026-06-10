package config

import "fufu/newapi"

type managedSiteEnvDef struct {
	Prefix                   string
	DefaultName              string
	DefaultURL               string
	DefaultRatio             string
	DefaultChannelListPath   string
	AllowAccessTokenFallback bool
}

func managedSiteFromEnv(def managedSiteEnvDef) (newapi.Site, bool) {
	url := NormalizeBaseURL(Env(def.Prefix + "_URL"))
	if url == "" {
		url = NormalizeBaseURL(def.DefaultURL)
	}
	token := Env(def.Prefix + "_TOKEN")
	if token == "" && def.AllowAccessTokenFallback {
		token = Env(def.Prefix + "_ACCESS_TOKEN")
	}
	if url == "" || token == "" {
		return newapi.Site{}, false
	}

	ratio := Env(def.Prefix + "_RECHARGE_RATIO")
	if ratio == "" {
		ratio = def.DefaultRatio
	}
	kind, ok := normalizeManagedSiteKind(Env(def.Prefix + "_KIND"))
	if !ok {
		return newapi.Site{}, false
	}
	return newapi.Site{
		Name:                stringOrDefault(Env(def.Prefix+"_NAME"), def.DefaultName),
		URL:                 url,
		Token:               token,
		UserID:              stringOrDefault(Env(def.Prefix+"_USER_ID"), "1"),
		Kind:                kind,
		QuotaUnit:           int64Value(Env(def.Prefix+"_QUOTA_UNIT"), newapi.DefaultQuotaUnit),
		Currency:            stringOrDefault(Env(def.Prefix+"_CURRENCY"), "$"),
		RechargeRatio:       floatValue(ratio, 1),
		ChannelListEndpoint: stringOrDefault(Env(def.Prefix+"_CHANNEL_LIST_ENDPOINT"), def.DefaultChannelListPath),
		SkipUserHeader:      boolValue(Env(def.Prefix + "_SKIP_USER_HEADER")),
		Note:                Env(def.Prefix + "_NOTE"),
	}, true
}
