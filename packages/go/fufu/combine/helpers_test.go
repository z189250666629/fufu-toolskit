package combine

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTokenFromRawNormalizesDefaultsAndNumbers(t *testing.T) {
	raw := map[string]any{
		"id":            float64(12),
		"key":           "abc123",
		"name":          "merged",
		"remain_quota":  json.Number("500"),
		"used_quota":    "10",
		"interval_unit": float64(8),
		"group":         "",
		"status":        float64(0),
	}

	token := tokenFromRaw(raw)
	if token.ID != 12 || token.Key != "sk-abc123" || token.Name != "merged" {
		t.Fatalf("unexpected token identity: %#v", token)
	}
	if token.RemainQuota != 500 || token.UsedQuota != 10 || token.IntervalUnit != 8 {
		t.Fatalf("unexpected token counters: %#v", token)
	}
	if token.Group != "mix" || token.Status != 1 {
		t.Fatalf("unexpected token defaults: %#v", token)
	}
	if token.Raw["id"] != raw["id"] {
		t.Fatalf("raw map should be preserved")
	}
}

func TestDataLookupHelpers(t *testing.T) {
	data := map[string]any{
		"data": []any{
			map[string]any{"name": "first", "id": float64(1)},
			"skip",
			map[string]any{"name": "target", "id": float64(2)},
		},
	}

	list := dataList(data)
	if len(list) != 2 || getString(list[0], "name") != "first" {
		t.Fatalf("dataList = %#v", list)
	}
	if found := findTokenByName(data, "target"); toInt(found["id"]) != 2 {
		t.Fatalf("findTokenByName = %#v", found)
	}
	if found := findTokenByName(data, "missing"); found != nil {
		t.Fatalf("unexpected token = %#v", found)
	}
}

func TestConversionFallbacks(t *testing.T) {
	if got := getString(map[string]any{"value": json.Number("123")}, "value"); got != "123" {
		t.Fatalf("getString number = %q", got)
	}
	if got := toInt64(json.Number("42.9")); got != 42 {
		t.Fatalf("toInt64 float json number = %d", got)
	}
	if got := toInt64("8.9"); got != 8 {
		t.Fatalf("toInt64 float string = %d", got)
	}
	if got := toIntDefault(nil, 7); got != 7 {
		t.Fatalf("toIntDefault nil = %d", got)
	}
	if got := toIntDefault("0", 7); got != 0 {
		t.Fatalf("toIntDefault explicit zero = %d", got)
	}
	if got := intOrDefault(0, 7); got != 7 {
		t.Fatalf("intOrDefault = %d", got)
	}
	if got := stringOrDefault("  ", "fallback"); got != "fallback" {
		t.Fatalf("stringOrDefault = %q", got)
	}
}

func TestHTTPAndStatusHelpers(t *testing.T) {
	msg := upstreamStatusMessage(APIResponse{StatusCode: 502}, "失败")
	if !strings.Contains(msg, "502") {
		t.Fatalf("upstreamStatusMessage = %q", msg)
	}
	if got := upstreamStatusMessage(APIResponse{}, "失败"); got != "失败" {
		t.Fatalf("upstreamStatusMessage fallback = %q", got)
	}
	if got := statusOrDefault(204, 1); got != 204 {
		t.Fatalf("statusOrDefault valid = %d", got)
	}
	if got := statusOrDefault(0, 1); got != 1 {
		t.Fatalf("statusOrDefault fallback = %d", got)
	}
}
