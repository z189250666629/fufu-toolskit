package tokens

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

func FromRaw(raw map[string]any) Token {
	return Token{ID: toInt(raw["id"]), Key: EnsureFullKey(getString(raw, "key")), Name: getString(raw, "name"), RemainQuota: toInt64(raw["remain_quota"]), UsedQuota: toInt64(raw["used_quota"]), IntervalUnit: toInt(raw["interval_unit"]), IntervalQuota: toInt64(raw["interval_quota"]), Group: getString(raw, "group"), Status: statusOrDefault(toInt(raw["status"]), 1), CreatedTime: toInt64(raw["created_time"]), Raw: raw}
}

func DataList(data map[string]any) []map[string]any {
	candidates := []any{data["data"], data["items"], data["tokens"]}
	if nested, ok := data["data"].(map[string]any); ok {
		candidates = append(candidates, nested["data"], nested["items"], nested["tokens"])
	}
	for _, c := range candidates {
		if arr, ok := c.([]any); ok {
			out := []map[string]any{}
			for _, item := range arr {
				if obj, ok := item.(map[string]any); ok {
					out = append(out, obj)
				}
			}
			return out
		}
	}
	return nil
}

func getString(obj map[string]any, key string) string {
	if obj == nil {
		return ""
	}
	if v, ok := obj[key]; ok {
		return strings.TrimSpace(fmt.Sprint(v))
	}
	return ""
}

func statusOrDefault(v, fallback int) int {
	if v == 0 {
		return fallback
	}
	return v
}

func toInt(v any) int { return int(toInt64(v)) }

func toInt64(v any) int64 {
	switch x := v.(type) {
	case nil:
		return 0
	case int:
		return int64(x)
	case int64:
		return x
	case float64:
		return int64(x)
	case json.Number:
		n, _ := x.Int64()
		return n
	case string:
		n, _ := strconv.ParseInt(strings.TrimSpace(x), 10, 64)
		return n
	default:
		n, _ := strconv.ParseInt(fmt.Sprint(x), 10, 64)
		return n
	}
}
