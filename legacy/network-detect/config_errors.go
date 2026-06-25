package main

import "strings"

func publicManagedSiteConfigError(message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return ""
	}
	for _, marker := range []string{" 不是有效 JSON", " 读取失败"} {
		if idx := strings.Index(message, marker); idx >= 0 {
			prefix := strings.TrimSpace(message[:idx])
			if prefix == "NEWAPI_MANAGED_API_SITES" {
				return prefix + marker
			}
			return "NEWAPI_MANAGED_API_CONFIG" + marker
		}
	}
	return "NEWAPI_MANAGED_API_CONFIG 配置读取失败"
}
