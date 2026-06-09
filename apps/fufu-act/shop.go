package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"fufu/config"
	"net/http"
	"strings"
)

func findShopPurchase(cardKey string) string {
	if config.Env("MCY_BASE_URL") == "" && config.Env("SHOP_BASE_URL") == "" {
		return ""
	}
	if mcyCookie == "" {
		_ = mcyLogin()
	}
	if mcyCookie == "" {
		return ""
	}
	data, err := mcyPost("/plugin/virtual-card-ship/card/get", map[string]any{"equal-card": cardKey, "page": 1, "limit": 1})
	if err != nil {
		return ""
	}
	if d, ok := data["data"].(map[string]any); ok {
		if arr, ok := d["list"].([]any); ok && len(arr) > 0 {
			if obj, ok := arr[0].(map[string]any); ok {
				return fmt.Sprint(obj["purchase_time"])
			}
		}
	}
	return ""
}

func mcyConfig() (string, string, string, string) {
	return strings.TrimRight(firstNonEmpty(config.Env("MCY_BASE_URL"), config.Env("SHOP_BASE_URL")), "/"), firstNonEmpty(config.Env("MCY_USERNAME"), config.Env("SHOP_USERNAME")), firstNonEmpty(config.Env("MCY_PASSWORD"), config.Env("SHOP_PASSWORD")), firstNonEmpty(config.Env("MCY_LOGIN_ENDPOINT"), "/admin/login")
}

func mcyLogin() error {
	base, user, pass, login := mcyConfig()
	if base == "" || user == "" || pass == "" {
		return fmt.Errorf("missing MCY config")
	}
	body, _ := json.Marshal(map[string]string{"username": user, "password": pass})
	resp, err := http.Post(base+login, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if sc := resp.Header.Get("Set-Cookie"); sc != "" {
		parts := []string{}
		for _, p := range strings.Split(sc, ",") {
			parts = append(parts, strings.Split(p, ";")[0])
		}
		mcyCookie = strings.Join(parts, "; ")
	}
	return nil
}

func mcyPost(endpoint string, payload any) (map[string]any, error) {
	base, _, _, _ := mcyConfig()
	b, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, base+endpoint, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cookie", mcyCookie)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var data map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&data)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return data, fmt.Errorf("MCY HTTP %d", resp.StatusCode)
	}
	return data, nil
}
