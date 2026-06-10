package combine

import (
	"encoding/json"
	"fmt"
	"fufu/rawconv"
	"strings"
)

func getString(obj map[string]any, key string) string {
	if obj == nil || obj[key] == nil {
		return ""
	}
	switch v := obj[key].(type) {
	case string:
		return v
	case json.Number:
		return v.String()
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprint(v)
	}
}

func toInt(v any) int { return rawconv.Int(v) }

func toIntDefault(v any, fallback int) int { return rawconv.IntDefault(v, fallback) }

func toInt64(v any) int64 { return rawconv.Int64(v) }

func intOrDefault(v, fallback int) int {
	if v == 0 {
		return fallback
	}
	return v
}

func int64OrDefault(v, fallback int64) int64 {
	if v == 0 {
		return fallback
	}
	return v
}

func stringOrDefault(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}

func strp(v string) *string { return &v }

func intp(v int) *int { return &v }

func statusOrDefault(status, fallback int) int {
	if status >= 100 && status <= 599 {
		return status
	}
	return fallback
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
