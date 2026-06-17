package activityapp

import (
	"context"
	"fmt"
	"fufu/config"
	"strings"
	"sync"
)

var mcyCookieMu sync.RWMutex
var mcyLoginMu sync.Mutex

func mcyConfig() (string, string, string, string) {
	c := getMCYRuntimeConfig()
	base := strings.TrimRight(firstNonEmpty(c.BaseURL, config.Env("MCY_BASE_URL"), config.Env("SHOP_BASE_URL")), "/")
	user := firstNonEmpty(c.Username, config.Env("MCY_USERNAME"), config.Env("SHOP_USERNAME"))
	pass := firstNonEmpty(c.Password, config.Env("MCY_PASSWORD"), config.Env("SHOP_PASSWORD"))
	login := firstNonEmpty(c.LoginEndpoint, config.Env("MCY_LOGIN_ENDPOINT"), "/admin/login")
	return base, user, pass, login
}

func getMCYCookie() string {
	mcyCookieMu.RLock()
	defer mcyCookieMu.RUnlock()
	return mcyCookie
}

func setMCYCookie(value string) {
	mcyCookieMu.Lock()
	mcyCookie = value
	mcyCookieMu.Unlock()
}

func ensureMCYCookie(ctx context.Context) error {
	if getMCYCookie() != "" {
		return nil
	}
	mcyLoginMu.Lock()
	defer mcyLoginMu.Unlock()
	if getMCYCookie() != "" {
		return nil
	}
	if err := mcyLogin(ctx); err != nil {
		return err
	}
	if getMCYCookie() == "" {
		return ErrShopLoginFailed
	}
	return nil
}

func refreshMCYCookie(ctx context.Context, staleCookie string) error {
	mcyLoginMu.Lock()
	defer mcyLoginMu.Unlock()
	if current := getMCYCookie(); current != "" && current != staleCookie {
		return nil
	}
	setMCYCookie("")
	if err := mcyLogin(ctx); err != nil {
		return err
	}
	if getMCYCookie() == "" {
		return ErrShopLoginFailed
	}
	return nil
}

func missingMCYConfigError() error {
	return fmt.Errorf("missing MCY config")
}
