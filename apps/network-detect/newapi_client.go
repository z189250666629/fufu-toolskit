package main

import (
	"context"
	"encoding/json"
	"fmt"
	"fufu/newapi"
	"strconv"
	"strings"
	"time"
)

func newAPIGet(ctx context.Context, site newapi.Site, endpoint string, timeout time.Duration) apiResult {
	if !strings.HasPrefix(endpoint, "/api/") {
		return apiResult{OK: false, Status: 400, Error: "不允许的 NewAPI 路径"}
	}
	c := newapi.NewClient(site)
	c.HTTPClient.Timeout = timeout
	res, data, err := c.Get(ctx, endpoint)
	if err != nil {
		if res.StatusCode > 0 {
			return apiResult{OK: false, Status: res.StatusCode, Error: newapi.UpstreamStatusMessage(res, "NewAPI 响应不是有效 JSON")}
		}
		return apiResult{OK: false, Status: 0, Error: "NewAPI 请求失败"}
	}
	if !res.OK() {
		return apiResult{OK: false, Status: res.StatusCode, Error: upstreamError(data, res.StatusCode), Data: data}
	}
	if !newapi.IsSuccess(data) {
		return apiResult{OK: false, Status: res.StatusCode, Error: upstreamError(data, res.StatusCode), Data: data}
	}
	return apiResult{OK: true, Status: res.StatusCode, Data: data}
}

func upstreamError(data map[string]any, status int) string {
	return newapi.ErrorMessage(data, status, "NewAPI 请求失败")
}

func str(v any) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(v))
}

func items(data map[string]any) []map[string]any {
	return newapi.PayloadItems(data, "data", "items", "logs", "channels")
}

func toInt64(v any) int64 {
	switch x := v.(type) {
	case json.Number:
		return jsonNumberToInt64(x)
	case float64:
		return int64(x)
	case int:
		return int64(x)
	case int64:
		return x
	case string:
		return parseInt64String(x)
	default:
		return parseInt64String(fmt.Sprint(x))
	}
}

func parseInt64String(value string) int64 {
	value = strings.TrimSpace(value)
	n, err := strconv.ParseInt(value, 10, 64)
	if err == nil {
		return n
	}
	f, _ := strconv.ParseFloat(value, 64)
	return int64(f)
}

func jsonNumberToInt64(n json.Number) int64 {
	value, err := n.Int64()
	if err == nil {
		return value
	}
	f, _ := n.Float64()
	return int64(f)
}

func toFloat(v any) float64 {
	switch x := v.(type) {
	case json.Number:
		n, _ := x.Float64()
		return n
	case float64:
		return x
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case string:
		n, _ := strconv.ParseFloat(strings.TrimSpace(x), 64)
		return n
	default:
		n, _ := strconv.ParseFloat(fmt.Sprint(x), 64)
		return n
	}
}

func toInt(v any) int { return int(toInt64(v)) }
