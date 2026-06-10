package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"fufu/config"
	"net/http"
	"strings"
	"time"
)

var mcyHTTPClient = &http.Client{Timeout: 15 * time.Second}

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
		if !isMCYAuthError(err) {
			return ""
		}
		mcyCookie = ""
		if err := mcyLogin(); err != nil || mcyCookie == "" {
			return ""
		}
		data, err = mcyPost("/plugin/virtual-card-ship/card/get", map[string]any{"equal-card": cardKey, "page": 1, "limit": 1})
		if err != nil {
			return ""
		}
	}
	if d, ok := data["data"].(map[string]any); ok {
		return extractPurchaseTime(d)
	}
	return ""
}

func extractPurchaseTime(data map[string]any) string {
	arr, ok := data["list"].([]any)
	if !ok || len(arr) == 0 {
		return ""
	}
	obj, ok := arr[0].(map[string]any)
	if !ok {
		return ""
	}
	purchaseTime, ok := obj["purchase_time"]
	if !ok || purchaseTime == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(purchaseTime))
}

func mcyConfig() (string, string, string, string) {
	return strings.TrimRight(firstNonEmpty(config.Env("MCY_BASE_URL"), config.Env("SHOP_BASE_URL")), "/"), firstNonEmpty(config.Env("MCY_USERNAME"), config.Env("SHOP_USERNAME")), firstNonEmpty(config.Env("MCY_PASSWORD"), config.Env("SHOP_PASSWORD")), firstNonEmpty(config.Env("MCY_LOGIN_ENDPOINT"), "/admin/login")
}

type mcyHTTPError struct {
	status int
}

func (e mcyHTTPError) Error() string {
	return fmt.Sprintf("MCY HTTP %d", e.status)
}

func isMCYAuthError(err error) bool {
	var httpErr mcyHTTPError
	return errors.As(err, &httpErr) && (httpErr.status == http.StatusUnauthorized || httpErr.status == http.StatusForbidden)
}

func mcyLogin() error {
	base, user, pass, login := mcyConfig()
	if base == "" || user == "" || pass == "" {
		return fmt.Errorf("missing MCY config")
	}
	body, _ := json.Marshal(map[string]string{"username": user, "password": pass})
	req, err := http.NewRequest(http.MethodPost, base+login, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := mcyHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return mcyHTTPError{status: resp.StatusCode}
	}
	cookies := resp.Cookies()
	if len(cookies) > 0 {
		parts := make([]string, 0, len(cookies))
		for _, cookie := range cookies {
			if cookie.Name == "" {
				continue
			}
			parts = append(parts, cookie.Name+"="+cookie.Value)
		}
		if len(parts) > 0 {
			mcyCookie = strings.Join(parts, "; ")
		}
	}
	return nil
}

func mcyPost(endpoint string, payload any) (map[string]any, error) {
	base, _, _, _ := mcyConfig()
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, base+endpoint, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cookie", mcyCookie)
	resp, err := mcyHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var data map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("MCY JSON decode failed: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return data, mcyHTTPError{status: resp.StatusCode}
	}
	return data, nil
}
