package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func Env(name string) string { return strings.TrimSpace(os.Getenv(name)) }

func NormalizeBaseURL(value string) string {
	value = strings.TrimRight(strings.TrimSpace(value), "/")
	if value == "" {
		return ""
	}
	if !strings.HasPrefix(value, "http://") && !strings.HasPrefix(value, "https://") {
		return ""
	}
	return value
}

func boolValue(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		v = strings.TrimSpace(strings.ToLower(v))
		return v == "1" || v == "true" || v == "yes" || v == "on"
	default:
		return false
	}
}

func int64Value(value any, fallback int64) int64 {
	switch v := value.(type) {
	case json.Number:
		if n, err := v.Int64(); err == nil && n > 0 {
			return n
		}
	case float64:
		if v > 0 {
			return int64(v)
		}
	case int64:
		if v > 0 {
			return v
		}
	case int:
		if v > 0 {
			return int64(v)
		}
	case string:
		if n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}

func floatValue(value any, fallback float64) float64 {
	switch v := value.(type) {
	case json.Number:
		if n, err := v.Float64(); err == nil && n > 0 {
			return n
		}
	case float64:
		if v > 0 {
			return v
		}
	case int:
		if v > 0 {
			return float64(v)
		}
	case string:
		if n, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}

func stringValue(item map[string]any, names ...string) string {
	for _, name := range names {
		if v, ok := item[name]; ok {
			if s := strings.TrimSpace(fmt.Sprint(v)); s != "" && s != "<nil>" {
				return s
			}
		}
	}
	return ""
}

func first(item map[string]any, keys ...string) any {
	for _, key := range keys {
		if v, ok := item[key]; ok {
			return v
		}
	}
	return nil
}

func stringOrDefault(v, fallback string) string {
	if strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return fallback
}
