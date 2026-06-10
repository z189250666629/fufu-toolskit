package main

import (
	"context"
	"fmt"
	"fufu/newapi"
	"fufu/rawconv"
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
	if status > 0 && (status < 200 || status >= 300) {
		return fmt.Sprintf("NewAPI 请求失败（上游状态 %d）", status)
	}
	return "NewAPI 请求失败"
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
	return rawconv.Int64(v)
}

func toFloat(v any) float64 {
	return rawconv.Float64(v)
}

func toInt(v any) int { return rawconv.Int(v) }
