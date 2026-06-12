package tokens

import (
	"fmt"
	"fufu/newapi"
	"fufu/rawconv"
	"strings"
)

func FromRaw(raw map[string]any) Token {
	return Token{ID: toInt(raw["id"]), Key: EnsureFullKey(getString(raw, "key")), Name: getString(raw, "name"), RemainQuota: toInt64(raw["remain_quota"]), UsedQuota: toInt64(raw["used_quota"]), IntervalUnit: toInt(raw["interval_unit"]), IntervalQuota: toInt64(raw["interval_quota"]), Group: getString(raw, "group"), Status: statusOrDefault(toInt(raw["status"]), 1), CreatedTime: toInt64(raw["created_time"]), Raw: raw}
}

func DataList(data map[string]any) []map[string]any {
	return newapi.PayloadItems(data, "data", "items", "tokens")
}

// payloadTotal reads the paginated total count from a NewAPI list/search
// payload. NewAPI wraps list responses as {data:{items,total,page,page_size}}
// (common.GetPageQuery / SetTotal), so the precise match count lives at
// data.total. Older flat {data:[...]} responses have no total; ok is false then.
func payloadTotal(data map[string]any) (int, bool) {
	if v, ok := data["total"]; ok {
		return toInt(v), true
	}
	if nested, ok := data["data"].(map[string]any); ok {
		if v, ok := nested["total"]; ok {
			return toInt(v), true
		}
	}
	return 0, false
}

func getString(obj map[string]any, key string) string {
	if obj == nil {
		return ""
	}
	if v, ok := obj[key]; ok {
		if v == nil {
			return ""
		}
		text := strings.TrimSpace(fmt.Sprint(v))
		if text == "<nil>" {
			return ""
		}
		return text
	}
	return ""
}

func statusOrDefault(v, fallback int) int {
	if v == 0 {
		return fallback
	}
	return v
}

func toInt(v any) int { return rawconv.Int(v) }

func toInt64(v any) int64 { return rawconv.Int64(v) }
