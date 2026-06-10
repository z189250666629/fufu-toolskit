package main

import (
	"context"
	"errors"
	"fufu/config"
	"fufu/newapi"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

func handleModelTest(w http.ResponseWriter, r *http.Request) {
	var body struct {
		SiteName string `json:"siteName"`
		Model    string `json:"model"`
		Group    string `json:"group"`
	}
	if err := readJSON(r, &body); err != nil {
		if errors.Is(err, errRequestBodyTooLarge) {
			writeJSONError(w, http.StatusRequestEntityTooLarge, "请求体过大")
			return
		}
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
	result, err := runModelTest(contextWithModelTestClient(r.Context(), clientIP(r)), body.SiteName, body.Model, body.Group)
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

type modelTestClientContextKey struct{}

func contextWithModelTestClient(ctx context.Context, client string) context.Context {
	client = strings.TrimSpace(client)
	if client == "" {
		return ctx
	}
	return context.WithValue(ctx, modelTestClientContextKey{}, client)
}

func modelTestClientFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	client, _ := ctx.Value(modelTestClientContextKey{}).(string)
	return strings.TrimSpace(client)
}

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
	now := time.Now().Unix()
	pruneManualTestCache(now)
	next := now + int64(modelTestCooldown/time.Second)
	clientCooldownKey := ""
	if client := modelTestClientFromContext(ctx); client != "" {
		clientCooldownKey = modelManualClientKey(siteName, client)
		if until, ok := reserveModelTestCooldown(&testClientCooldowns, clientCooldownKey, now, next); !ok {
			return nil, &httpError{Status: 429, Message: "该站点模型测试仍在冷却中", NextAllowedAt: until}
		}
	}
	key := modelManualKey(siteName, model, group)
	if until, ok := reserveModelTestCooldown(&testCooldowns, key, now, next); !ok {
		return nil, &httpError{Status: 429, Message: "该模型测试仍在冷却中", NextAllowedAt: until}
	}
	channels, errMsg := loadSiteChannels(ctx, *site)
	if errMsg != "" {
		clearModelTestCooldownReservation(key, next)
		clearClientModelTestCooldownReservation(clientCooldownKey, next)
		return nil, &httpError{Status: 502, Message: errMsg}
	}
	candidates := selectModelTestChannels(channels, model, group)
	if len(candidates) == 0 {
		clearModelTestCooldownReservation(key, next)
		return nil, &httpError{Status: 400, Message: "当前单元格没有启用通道可测试"}
	}
	if err := ctx.Err(); err != nil {
		clearModelTestCooldownReservation(key, next)
		clearClientModelTestCooldownReservation(clientCooldownKey, next)
		return nil, err
	}
	stream := supportsStream(model)
	var res apiResult
	for _, ch := range candidates {
		res = newAPIGet(ctx, *site, channelTestEndpoint(ch.ID, model, stream), 45*time.Second)
		if res.OK {
			break
		}
	}
	if err := ctx.Err(); err != nil {
		testCooldowns.Delete(key)
		if clientCooldownKey != "" {
			testClientCooldowns.Delete(clientCooldownKey)
		}
		return nil, err
	}
	rec := testRecord{OK: res.OK, Status: map[bool]string{true: "operational", false: "down"}[res.OK], Group: group, Stream: stream, TestedAt: time.Now().Unix(), Message: truncate(testMessage(res), 180), NextAllowedAt: next}
	testResults.Store(key, rec)
	applyManualToCachedStatus(siteName, model, group, rec, next)
	return map[string]any{"siteName": siteName, "model": model, "group": group, "test": rec}, nil
}

func modelManualClientKey(siteName, client string) string {
	return strings.TrimSpace(siteName) + "\x00" + strings.TrimSpace(client)
}

func reserveModelTestCooldown(cache *sync.Map, key string, now, next int64) (int64, bool) {
	if key == "" {
		return next, true
	}
	if v, ok := cache.Load(key); ok {
		if until, ok := v.(int64); ok && until > now {
			return until, false
		}
	}
	if existing, loaded := cache.LoadOrStore(key, next); loaded {
		if until, ok := existing.(int64); ok && until > now {
			return until, false
		}
		cache.Store(key, next)
	}
	return next, true
}

func clearModelTestCooldownReservation(key string, next int64) {
	clearManualTestCooldownReservation(&testCooldowns, key, next)
}

func clearClientModelTestCooldownReservation(key string, next int64) {
	clearManualTestCooldownReservation(&testClientCooldowns, key, next)
}

func clearManualTestCooldownReservation(cache *sync.Map, key string, next int64) {
	if key == "" {
		return
	}
	if v, ok := cache.Load(key); ok {
		until, ok := v.(int64)
		if !ok || until != next {
			return
		}
		cache.Delete(key)
	}
}

func pruneManualTestCache(now int64) {
	testCooldowns.Range(func(key, value any) bool {
		until, ok := value.(int64)
		if !ok || until <= now {
			testCooldowns.Delete(key)
			testResults.Delete(key)
		}
		return true
	})
	testResults.Range(func(key, value any) bool {
		rec, ok := value.(testRecord)
		if !ok || rec.NextAllowedAt <= now {
			testResults.Delete(key)
			testCooldowns.Delete(key)
		}
		return true
	})
	testClientCooldowns.Range(func(key, value any) bool {
		until, ok := value.(int64)
		if !ok || until <= now {
			testClientCooldowns.Delete(key)
		}
		return true
	})
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

func applyManualToCachedStatus(siteName, model, group string, rec testRecord, next int64) {
	modelCache.Lock()
	defer modelCache.Unlock()
	if modelCache.Value == nil {
		return
	}
	nextStatus := cloneModelStatus(modelCache.Value)
	applyManual(nextStatus, siteName, model, group, rec, next)
	modelCache.Value = nextStatus
}

func cloneModelStatus(status *ModelStatus) *ModelStatus {
	if status == nil {
		return nil
	}
	clone := *status
	if status.Sites != nil {
		clone.Sites = make([]SiteStatus, len(status.Sites))
		for i, site := range status.Sites {
			clone.Sites[i] = site
			clone.Sites[i].Groups = append([]string(nil), site.Groups...)
		}
	}
	if status.Models != nil {
		clone.Models = make([]ModelRow, len(status.Models))
		for i := range status.Models {
			clone.Models[i] = cloneModelRow(status.Models[i])
		}
	}
	if status.Totals != nil {
		clone.Totals = make(map[string]int, len(status.Totals))
		for key, value := range status.Totals {
			clone.Totals[key] = value
		}
	}
	return &clone
}

func cloneModelRow(row ModelRow) ModelRow {
	clone := row
	if row.PerSite != nil {
		clone.PerSite = make(map[string]*ModelCell, len(row.PerSite))
		for key, cell := range row.PerSite {
			clone.PerSite[key] = cloneModelCell(cell)
		}
	}
	return clone
}

func cloneModelCell(cell *ModelCell) *ModelCell {
	if cell == nil {
		return nil
	}
	clone := *cell
	clone.Groups = append([]string(nil), cell.Groups...)
	if cell.Pricing != nil {
		price := *cell.Pricing
		clone.Pricing = &price
	}
	if cell.GroupStats != nil {
		clone.GroupStats = make(map[string]*ModelCell, len(cell.GroupStats))
		for key, groupCell := range cell.GroupStats {
			clone.GroupStats[key] = cloneModelCell(groupCell)
		}
	}
	return &clone
}

func buildOverview(ctx context.Context, q url.Values) map[string]any {
	status := getModelStatus(ctx, false)
	return map[string]any{"configured": status.Configured, "configError": status.ConfigError, "generatedAt": status.GeneratedAt, "sites": status.Sites, "totals": status.Totals, "modelAvailability": status.Models, "allLogs": []any{}}
}
