package combine

import "strings"

func normalizeGenerateGroup(group string) string {
	value := strings.TrimSpace(group)
	if value == "" {
		return "mix"
	}
	return value
}

func buildGeneratedTokenCreateBody(uniqueName string, totalQuota int64, group string, intervalUnit int) map[string]any {
	return buildTokenCreateBody(uniqueName, totalQuota, group, intervalUnit)
}
