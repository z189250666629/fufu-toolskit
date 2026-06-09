package combine

import (
	"encoding/json"
	"fmt"
	"strconv"
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

func toInt(v any) int { return int(toInt64(v)) }

func toIntDefault(v any, fallback int) int {
	if v == nil {
		return fallback
	}
	return toInt(v)
}

func toInt64(v any) int64 {
	switch x := v.(type) {
	case nil:
		return 0
	case int:
		return int64(x)
	case int64:
		return x
	case int32:
		return int64(x)
	case float64:
		return int64(x)
	case float32:
		return int64(x)
	case json.Number:
		if i, err := x.Int64(); err == nil {
			return i
		}
		if f, err := x.Float64(); err == nil {
			return int64(f)
		}
	case string:
		s := strings.TrimSpace(x)
		if i, err := strconv.ParseInt(s, 10, 64); err == nil {
			return i
		}
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return int64(f)
		}
	}
	return 0
}

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
