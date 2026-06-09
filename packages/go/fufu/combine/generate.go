package combine

import (
	"math"
	"strconv"
	"strings"
)

func validateGenerateParams(count int, quota float64, intervalUnit int) bool {
	return count >= 1 && count <= 100 && quota > 0 && intervalUnit != 0
}

func generateTotalQuota(quota float64, quotaUnit int64) int64 {
	return int64(math.Round(quota * float64(quotaUnit)))
}

func generateTokenFinalName(quota float64) string {
	return strconv.FormatFloat(quota, 'f', -1, 64)
}

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
