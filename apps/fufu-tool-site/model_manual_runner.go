package main

import (
	"context"
	"fufu/newapi"
	"strings"
	"sync"
	"time"
)

func (e *httpError) Error() string { return e.Message }

var runModelTest = testModel

type modelTestPreferredURLContextKey struct{}

func contextWithModelTestPreferredURL(ctx context.Context, preferredURL string) context.Context {
	preferredURL = strings.TrimSpace(preferredURL)
	if preferredURL == "" {
		return ctx
	}
	return context.WithValue(ctx, modelTestPreferredURLContextKey{}, preferredURL)
}

func modelTestPreferredURLFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	preferredURL, _ := ctx.Value(modelTestPreferredURLContextKey{}).(string)
	return strings.TrimSpace(preferredURL)
}

func testModel(ctx context.Context, siteName, model, group string) (map[string]any, error) {
	sites, configMsg := managedSitesForRuntime()
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
	key := modelManualKey(siteName, model, group)
	if until, ok := reserveModelTestCooldown(&testCooldowns, key, now, next); !ok {
		return nil, &httpError{Status: 429, Message: "该模型测试仍在冷却中", NextAllowedAt: until}
	}
	stream := supportsStream(model)
	candidateSites := orderedManualTestSites(*site, modelTestPreferredURLFromContext(ctx))
	if len(candidateSites) == 0 {
		candidateSites = []newapi.Site{*site}
	}
	var res apiResult
	var lastChannelError string
	loadedChannels := false
	foundTestableChannel := false
	for _, candidateSite := range candidateSites {
		channels, errMsg := loadSiteChannels(ctx, candidateSite)
		if errMsg != "" {
			lastChannelError = errMsg
			if err := ctx.Err(); err != nil {
				clearModelTestCooldownReservation(key, next)
				return nil, err
			}
			continue
		}
		loadedChannels = true
		candidates := selectModelTestChannels(channels, model, group)
		if len(candidates) == 0 {
			continue
		}
		foundTestableChannel = true
		for _, ch := range candidates {
			res = newAPIGet(ctx, candidateSite, channelTestEndpoint(ch.ID, model, stream), 45*time.Second)
			if res.OK {
				break
			}
		}
		if err := ctx.Err(); err != nil {
			clearModelTestCooldownReservation(key, next)
			return nil, err
		}
		if res.OK {
			break
		}
	}
	if !loadedChannels {
		clearModelTestCooldownReservation(key, next)
		return nil, &httpError{Status: 502, Message: lastChannelError}
	}
	if !foundTestableChannel {
		clearModelTestCooldownReservation(key, next)
		return nil, &httpError{Status: 400, Message: "当前单元格没有启用通道可测试"}
	}
	if err := ctx.Err(); err != nil {
		testCooldowns.Delete(key)
		return nil, err
	}
	rec := testRecord{OK: res.OK, Status: map[bool]string{true: "operational", false: "down"}[res.OK], Group: group, Stream: stream, TestedAt: time.Now().Unix(), Message: truncate(testMessage(res), 180), NextAllowedAt: next}
	testResults.Store(key, rec)
	applyManualToCachedStatus(siteName, model, group, rec, next)
	return map[string]any{"siteName": siteName, "model": model, "group": group, "test": rec}, nil
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
