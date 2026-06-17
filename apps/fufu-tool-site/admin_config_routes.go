package main

import (
	"encoding/json"
	"fufu/activity"
	"fufu/admincore"
	"net/http"
)

func isUnifiedAdminConfigAPI(path string) bool {
	return path == "/api/admin/config"
}

func handleUnifiedAdminConfigAPI(w http.ResponseWriter, r *http.Request) {
	if !requireUnifiedAdminToken(w, r) {
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, adminConfigResponse(unifiedConfig.Snapshot()))
	case http.MethodPut:
		var patch adminConfigPatch
		if err := readJSON(r, &patch); err != nil {
			writeJSONError(w, http.StatusBadRequest, "配置格式错误")
			return
		}
		cfg, newAPIChanged, err := unifiedConfig.SavePatch(patch)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		applyToolConfigSnapshot(cfg)
		if newAPIChanged {
			rebuildCombine()
		}
		writeJSON(w, http.StatusOK, adminConfigResponse(cfg))
	default:
		w.Header().Set("Allow", "GET, PUT")
		writeJSONError(w, http.StatusMethodNotAllowed, "Only GET, PUT")
	}
}

func adminConfigResponse(cfg ToolConfig) map[string]any {
	activityPayload := map[string]any{}
	if raw, err := json.Marshal(cfg.Activity); err == nil {
		_ = json.Unmarshal(raw, &activityPayload)
	}
	balanced := activity.BalancedPrizePoolForGame(cfg.Activity, activity.GameSlot)
	activityPayload["targetPerDrawExpectedValue"] = balanced.TargetPerDrawExpectedValue
	activityPayload["balancedExpectedValue"] = balanced.ActualExpectedValue
	return map[string]any{
		"newapi": map[string]any{
			"sites": adminSiteResponses(cfg.NewAPI.Sites),
		},
		"navigation": map[string]any{
			"cards": navigationCardResponses(cfg.Navigation.Cards),
		},
		"activity": activityPayload,
		"mcy": map[string]any{
			"baseUrl":        cfg.MCY.BaseURL,
			"username":       cfg.MCY.Username,
			"loginEndpoint":  cfg.MCY.LoginEndpoint,
			"uploadEndpoint": cfg.MCY.UploadEndpoint,
			"passwordSet":    cfg.MCY.Password != "",
			"passwordMasked": maskSecret(cfg.MCY.Password),
		},
	}
}

func adminSiteResponses(sites []ManagedAPISiteConfig) []map[string]any {
	out := make([]map[string]any, 0, len(sites))
	for _, site := range sites {
		urls := make([]map[string]any, 0, len(site.URLs))
		for _, u := range site.URLs {
			urls = append(urls, map[string]any{"name": u.Name, "url": u.URL})
		}
		out = append(out, map[string]any{
			"name":                site.Name,
			"category":            site.Category,
			"urls":                urls,
			"url":                 site.URL,
			"userId":              site.UserID,
			"kind":                site.Kind,
			"skipUserHeader":      site.SkipUserHeader,
			"quotaUnit":           site.QuotaUnit,
			"currency":            site.Currency,
			"rechargeRatio":       site.RechargeRatio,
			"channelListEndpoint": site.ChannelListEndpoint,
			"note":                site.Note,
			"tokenSet":            site.Token != "",
			"tokenMasked":         maskSecret(site.Token),
		})
	}
	return out
}

func maskSecret(secret string) string {
	return admincore.MaskSecret(secret)
}
