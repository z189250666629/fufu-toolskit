package main

import (
	"fufu/activity"
	"fufu/config"
	"os"
	"strings"
)

func defaultToolConfig(root string) ToolConfig {
	sites, _ := config.LoadManagedSites(root)
	return ToolConfig{
		NewAPI:     NewAPIAdminConfig{Sites: managedSiteConfigsFromSites(sites)},
		Navigation: defaultNavigationConfig(),
		Activity:   activity.DefaultConfig(),
		MCY:        mcyConfigFromEnv(),
	}
}

// mcyConfigFromEnv seeds the MCY admin config from environment variables on the
// first boot; after that the database is the source of truth.
func mcyConfigFromEnv() MCYAdminConfig {
	return MCYAdminConfig{
		BaseURL:        firstNonEmptyEnv("MCY_BASE_URL", "SHOP_BASE_URL"),
		Username:       firstNonEmptyEnv("MCY_USERNAME", "SHOP_USERNAME"),
		Password:       firstNonEmptyEnv("MCY_PASSWORD", "SHOP_PASSWORD"),
		Cookie:         os.Getenv("MCY_COOKIE"),
		LoginEndpoint:  os.Getenv("MCY_LOGIN_ENDPOINT"),
		UploadEndpoint: os.Getenv("MCY_UPLOAD_ENDPOINT"),
	}
}

func firstNonEmptyEnv(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}
