package main

import (
	"context"
	"errors"
	"fufu/config"
	"fufu/newapi"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func handleModelTest(w http.ResponseWriter, r *http.Request) {
	var body struct {
		SiteName string `json:"siteName"`
		Model    string `json:"model"`
		Group    string `json:"group"`
	}
	if err := readJSON(r, &body); err != nil {
		writeJSONError(w, 400, "请求体无效")
		return
	}
	body.SiteName = strings.TrimSpace(body.SiteName)
	body.Model = strings.TrimSpace(body.Model)
	body.Group = strings.TrimSpace(body.Group)
	if body.SiteName == "" || body.Model == "" {
		writeJSONError(w, 400, "siteName 和 model 必填")
		return
	}
	result, err := runModelTest(r.Context(), body.SiteName, body.Model, body.Group)
	if err != nil {
		var e *httpError
		if errors.As(err, &e) {
			writeJSON(w, e.Status, map[string]any{"error": e.Message, "nextAllowedAt": e.NextAllowedAt})
		} else {
			writeJSONError(w, 500, "模型测试失败，请稍后重试")
		}
		return
	}
	writeJSON(w, 200, result)
}

func (e *httpError) Error() string { return e.Message }

var runModelTest = testModel

func testModel(ctx context.Context, siteName, model, group string) (map[string]any, error) {
	sites, configMsg := config.LoadManagedSites(rootDir)
	if configMsg != "" && len(sites) == 0 {
		return nil, &httpError{Status: 500, Message: publicManagedSiteConfigError(configMsg)}
	}
	var site *newapi.Site
	for i := range sites {
		if sites[i].Name == siteName {
			site = &sites[i]
			break
		}
	}
	if site == nil {
		return nil, &httpError{Status: 404, Message: "站点不存在"}
	}
	key := modelManualKey(siteName, model, group)
	now := time.Now().Unix()
	if v, ok := testCooldowns.Load(key); ok && v.(int64) > now {
		return nil, &httpError{Status: 429, Message: "该模型测试仍在冷却中", NextAllowedAt: v.(int64)}
	}
	channels, errMsg := loadSiteChannels(*site)
	if errMsg != "" {
		return nil, &httpError{Status: 502, Message: errMsg}
	}
	candidates := selectModelTestChannels(channels, model, group)
	if len(candidates) == 0 {
		return nil, &httpError{Status: 400, Message: "当前单元格没有启用通道可测试"}
	}
	next := now + int64(modelTestCooldown/time.Second)
	testCooldowns.Store(key, next)
	stream := supportsStream(model)
	var res apiResult
	for _, ch := range candidates {
		res = newAPIGet(ctx, *site, channelTestEndpoint(ch.ID, model, stream), 45*time.Second)
		if res.OK {
			break
		}
	}
	rec := testRecord{OK: res.OK, Status: map[bool]string{true: "operational", false: "down"}[res.OK], Group: group, Stream: stream, TestedAt: time.Now().Unix(), Message: truncate(testMessage(res), 180), NextAllowedAt: next}
	testResults.Store(key, rec)
	modelCache.Lock()
	if modelCache.Value != nil {
		applyManual(modelCache.Value, siteName, model, group, rec, next)
	}
	modelCache.Unlock()
	return map[string]any{"siteName": siteName, "model": model, "group": group, "test": rec}, nil
}

func supportsStream(model string) bool {
	name := strings.ToLower(model)
	return !(strings.Contains(name, "rerank") || strings.Contains(name, "embedding") || strings.Contains(name, "embed") || strings.HasPrefix(name, "m3e") || strings.Contains(name, "bge-") || strings.Contains(name, "seedream"))
}

func testMessage(r apiResult) string {
	if r.OK {
		return "测试通过"
	}
	if r.Error != "" {
		return r.Error
	}
	return "测试失败"
}

func truncate(s string, n int) string {
	if len([]rune(s)) <= n {
		return s
	}
	return string([]rune(s)[:n])
}

func applyManualToCell(c *ModelCell, rec testRecord, next int64) {
	c.ManualTest = rec
	c.NextTestAllowedAt = next
	if rec.OK {
		c.SuccessCount++
		c.LastSuccessAt = rec.TestedAt
	} else {
		c.FailureCount++
		c.LastFailureAt = rec.TestedAt
	}
	c.RequestCount = c.SuccessCount + c.FailureCount
	c.SuccessRate = rate(c.SuccessCount, c.FailureCount)
	c.Status = statusFromCounts(c.SuccessCount, c.FailureCount)
	c.LastSeenAt = maxInt64(c.LastSuccessAt, c.LastFailureAt)
}

func applyManual(ms *ModelStatus, siteName, model, group string, rec testRecord, next int64) {
	for i := range ms.Models {
		if ms.Models[i].Model == model {
			if c := ms.Models[i].PerSite[siteName]; c != nil {
				oldStatus := ms.Models[i].Status
				if group != "" {
					if groupCell := c.GroupStats[group]; groupCell != nil {
						applyManualToCell(groupCell, rec, next)
					}
				} else {
					applyManualToCell(c, rec, next)
				}
				recomputeModelRowSummary(&ms.Models[i])
				updateModelStatusTotalsForRowStatus(ms, oldStatus, ms.Models[i].Status)
			}
		}
	}
}

func buildOverview(q url.Values) map[string]any {
	status := getModelStatus(false)
	return map[string]any{"configured": status.Configured, "configError": status.ConfigError, "generatedAt": status.GeneratedAt, "sites": status.Sites, "totals": status.Totals, "modelAvailability": status.Models, "allLogs": []any{}}
}
