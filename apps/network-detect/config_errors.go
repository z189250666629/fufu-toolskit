package main

import "strings"

func publicManagedSiteConfigError(message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return ""
	}
	for _, marker := range []string{" 不是有效 JSON", " 读取失败"} {
		if idx := strings.Index(message, marker); idx >= 0 {
			return message[:idx] + marker
		}
	}
	return message
}
