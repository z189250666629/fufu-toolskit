package activityapp

import (
	"fufu/config"
	"strings"
	"sync"
)

// MCYRuntimeConfig is the MCY shop configuration supplied by the admin panel and
// persisted in tool-config.db. Blank fields fall back to environment variables,
// so env stays a seed/override while the admin UI is the source of truth.
type MCYRuntimeConfig struct {
	BaseURL        string
	Username       string
	Password       string
	Cookie         string
	LoginEndpoint  string
	UploadEndpoint string
}

var mcyConfigMu sync.RWMutex
var mcyRuntimeConfig MCYRuntimeConfig

// SetMCYRuntimeConfig applies admin-supplied MCY config. Changing the shop base
// url / credentials drops the cached login cookie so the next request
// re-authenticates with the new values.
func SetMCYRuntimeConfig(c MCYRuntimeConfig) {
	c.BaseURL = strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	c.Username = strings.TrimSpace(c.Username)
	c.Password = strings.TrimSpace(c.Password)
	c.Cookie = strings.TrimSpace(c.Cookie)
	c.LoginEndpoint = strings.TrimSpace(c.LoginEndpoint)
	c.UploadEndpoint = strings.TrimSpace(c.UploadEndpoint)

	mcyConfigMu.Lock()
	credsChanged := mcyRuntimeConfig.BaseURL != c.BaseURL ||
		mcyRuntimeConfig.Username != c.Username ||
		mcyRuntimeConfig.Password != c.Password
	mcyRuntimeConfig = c
	mcyConfigMu.Unlock()

	switch {
	case c.Cookie != "":
		setMCYCookie(c.Cookie)
	case credsChanged:
		setMCYCookie("")
	}
}

func getMCYRuntimeConfig() MCYRuntimeConfig {
	mcyConfigMu.RLock()
	defer mcyConfigMu.RUnlock()
	return mcyRuntimeConfig
}

func mcyBaseURL() string {
	c := getMCYRuntimeConfig()
	return strings.TrimRight(firstNonEmpty(c.BaseURL, config.Env("MCY_BASE_URL"), config.Env("SHOP_BASE_URL")), "/")
}

// mcyConfigured reports whether an MCY shop base url is available (admin or env).
func mcyConfigured() bool {
	return mcyBaseURL() != ""
}

func mcyUploadEndpoint() string {
	c := getMCYRuntimeConfig()
	return firstNonEmpty(c.UploadEndpoint, config.Env("MCY_UPLOAD_ENDPOINT"), "/plugin/virtual-card-ship/card/add")
}
