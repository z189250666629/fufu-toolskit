package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestGetModelStatusInvalidatesWhenManagedSitesConfigChanges(t *testing.T) {
	oldRootDir := rootDir
	oldValue := modelCache.Value
	oldExpires := modelCache.Expires
	oldKey := modelCache.Key
	oldForceRefreshAfter := modelCache.ForceRefreshAfter
	oldInflight := modelCache.Inflight
	t.Cleanup(func() {
		rootDir = oldRootDir
		modelCache.Lock()
		modelCache.Value = oldValue
		modelCache.Expires = oldExpires
		modelCache.Key = oldKey
		modelCache.ForceRefreshAfter = oldForceRefreshAfter
		modelCache.Inflight = oldInflight
		modelCache.Unlock()
	})
	rootDir = t.TempDir()
	modelCache.Lock()
	modelCache.Value = nil
	modelCache.Expires = time.Time{}
	modelCache.Key = ""
	modelCache.ForceRefreshAfter = time.Time{}
	modelCache.Inflight = nil
	modelCache.Unlock()
	clearManagedSiteEnv(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"success": true, "data": []any{}})
	}))
	defer server.Close()

	t.Setenv("NEWAPI_MANAGED_API_SITES", managedSiteConfigJSON("site-a", server.URL))
	first := getModelStatus(context.Background(), false)
	if len(first.Sites) != 1 || first.Sites[0].Site.Name != "site-a" {
		t.Fatalf("first status sites = %#v", first.Sites)
	}

	t.Setenv("NEWAPI_MANAGED_API_SITES", managedSiteConfigJSON("site-b", server.URL))
	second := getModelStatus(context.Background(), false)
	if len(second.Sites) != 1 || second.Sites[0].Site.Name != "site-b" {
		t.Fatalf("model status cache should not survive managed-site config changes: %#v", second.Sites)
	}
}

func TestModelStatusCacheKeyIncludesConnectivityOverrides(t *testing.T) {
	oldRootDir := rootDir
	t.Cleanup(func() { rootDir = oldRootDir })
	rootDir = t.TempDir()
	clearManagedSiteEnv(t)
	clearConnectivityEnv(t)

	baseKey := modelStatusCacheKey(rootDir)
	t.Setenv("CONNECTIVITY_TARGETS", `[{"id":"api","name":"API","urls":["https://api-a.example.test"]}]`)
	inlineKey := modelStatusCacheKey(rootDir)
	if inlineKey == baseKey {
		t.Fatal("CONNECTIVITY_TARGETS should participate in the model-status cache key")
	}

	t.Setenv("CONNECTIVITY_TARGETS", "")
	t.Setenv("CONNECTIVITY_TOKEN_URLS", "https://token-a.example.test")
	tokenKey := modelStatusCacheKey(rootDir)
	if tokenKey == baseKey {
		t.Fatal("CONNECTIVITY_TOKEN_URLS should participate in the model-status cache key")
	}
}

func TestGetModelStatusCoalescesConcurrentColdLoads(t *testing.T) {
	oldRootDir := rootDir
	oldValue := modelCache.Value
	oldExpires := modelCache.Expires
	oldKey := modelCache.Key
	oldForceRefreshAfter := modelCache.ForceRefreshAfter
	oldInflight := modelCache.Inflight
	t.Cleanup(func() {
		rootDir = oldRootDir
		modelCache.Lock()
		modelCache.Value = oldValue
		modelCache.Expires = oldExpires
		modelCache.Key = oldKey
		modelCache.ForceRefreshAfter = oldForceRefreshAfter
		modelCache.Inflight = oldInflight
		modelCache.Unlock()
	})
	rootDir = t.TempDir()
	modelCache.Lock()
	modelCache.Value = nil
	modelCache.Expires = time.Time{}
	modelCache.Key = ""
	modelCache.ForceRefreshAfter = time.Time{}
	modelCache.Inflight = nil
	modelCache.Unlock()
	clearManagedSiteEnv(t)

	var requestCount atomic.Int32
	firstRequestStarted := make(chan struct{})
	releaseFirstRequest := make(chan struct{})
	var closeFirstStarted sync.Once
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(releaseFirstRequest) }) }
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requestCount.Add(1) == 1 {
			closeFirstStarted.Do(func() { close(firstRequestStarted) })
			<-releaseFirstRequest
		}
		writeJSON(w, http.StatusOK, map[string]any{"success": true, "data": []any{}})
	}))
	t.Cleanup(func() {
		release()
		server.Close()
	})
	t.Setenv("NEWAPI_MANAGED_API_SITES", managedSiteConfigJSON("site-a", server.URL))

	firstDone := make(chan *ModelStatus, 1)
	secondDone := make(chan *ModelStatus, 1)
	go func() { firstDone <- getModelStatus(context.Background(), false) }()
	select {
	case <-firstRequestStarted:
	case <-time.After(time.Second):
		t.Fatal("first model-status load did not reach upstream")
	}
	go func() { secondDone <- getModelStatus(context.Background(), false) }()

	time.Sleep(100 * time.Millisecond)
	if got := requestCount.Load(); got != 1 {
		t.Fatalf("concurrent cold loads should share the first build before it completes, got %d upstream requests", got)
	}
	release()

	first := <-firstDone
	second := <-secondDone
	if first == nil || second == nil || len(first.Sites) != 1 || len(second.Sites) != 1 {
		t.Fatalf("unexpected statuses: first=%#v second=%#v", first, second)
	}
}

func TestGetModelStatusReusesFreshCacheForRepeatedForcedRefreshes(t *testing.T) {
	oldRootDir := rootDir
	oldValue := modelCache.Value
	oldExpires := modelCache.Expires
	oldKey := modelCache.Key
	oldForceRefreshAfter := modelCache.ForceRefreshAfter
	oldInflight := modelCache.Inflight
	oldBuild := buildModelStatusForCache
	t.Cleanup(func() {
		rootDir = oldRootDir
		buildModelStatusForCache = oldBuild
		modelCache.Lock()
		modelCache.Value = oldValue
		modelCache.Expires = oldExpires
		modelCache.Key = oldKey
		modelCache.ForceRefreshAfter = oldForceRefreshAfter
		modelCache.Inflight = oldInflight
		modelCache.Unlock()
	})
	rootDir = t.TempDir()
	modelCache.Lock()
	modelCache.Value = nil
	modelCache.Expires = time.Time{}
	modelCache.Key = ""
	modelCache.ForceRefreshAfter = time.Time{}
	modelCache.Inflight = nil
	modelCache.Unlock()
	clearManagedSiteEnv(t)

	var builds atomic.Int32
	buildModelStatusForCache = func(ctx context.Context) *ModelStatus {
		n := builds.Add(1)
		return &ModelStatus{
			Configured:    true,
			GeneratedAt:   int64(n),
			ExpiresAt:     time.Now().Add(modelStatusCacheTTL).Unix(),
			WindowSeconds: modelStatusWindowSeconds,
			Totals:        map[string]int{"siteCount": 1},
		}
	}

	first := getModelStatus(context.Background(), false)
	second := getModelStatus(context.Background(), true)

	if got := builds.Load(); got != 1 {
		t.Fatalf("repeated forced refresh should reuse fresh cache instead of rebuilding immediately, got %d builds", got)
	}
	if second.GeneratedAt != first.GeneratedAt {
		t.Fatalf("forced refresh reused unexpected snapshot: first=%#v second=%#v", first, second)
	}

	modelCache.Lock()
	modelCache.ForceRefreshAfter = time.Now().Add(-time.Second)
	modelCache.Unlock()

	third := getModelStatus(context.Background(), true)
	if got := builds.Load(); got != 2 {
		t.Fatalf("forced refresh should rebuild after the cooldown, got %d builds", got)
	}
	if third.GeneratedAt == first.GeneratedAt {
		t.Fatalf("forced refresh after cooldown should return a fresh snapshot: first=%#v third=%#v", first, third)
	}
}

func TestGetModelStatusDoesNotCacheCanceledBuilds(t *testing.T) {
	oldRootDir := rootDir
	oldValue := modelCache.Value
	oldExpires := modelCache.Expires
	oldKey := modelCache.Key
	oldForceRefreshAfter := modelCache.ForceRefreshAfter
	oldInflight := modelCache.Inflight
	t.Cleanup(func() {
		rootDir = oldRootDir
		modelCache.Lock()
		modelCache.Value = oldValue
		modelCache.Expires = oldExpires
		modelCache.Key = oldKey
		modelCache.ForceRefreshAfter = oldForceRefreshAfter
		modelCache.Inflight = oldInflight
		modelCache.Unlock()
	})
	rootDir = t.TempDir()
	modelCache.Lock()
	modelCache.Value = nil
	modelCache.Expires = time.Time{}
	modelCache.Key = ""
	modelCache.ForceRefreshAfter = time.Time{}
	modelCache.Inflight = nil
	modelCache.Unlock()
	clearManagedSiteEnv(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	t.Setenv("NEWAPI_MANAGED_API_SITES", managedSiteConfigJSON("site-a", "http://127.0.0.1:1"))

	canceled := getModelStatus(ctx, false)
	if len(canceled.Sites) != 1 || canceled.Sites[0].LogError == "" {
		t.Fatalf("canceled build should return an uncached error snapshot: %#v", canceled)
	}
	modelCache.Lock()
	cached := modelCache.Value
	modelCache.Unlock()
	if cached != nil {
		t.Fatalf("canceled model-status build should not populate cache: %#v", cached)
	}
}

func TestGetModelStatusClearsInflightAfterBuildPanic(t *testing.T) {
	oldRootDir := rootDir
	oldValue := modelCache.Value
	oldExpires := modelCache.Expires
	oldKey := modelCache.Key
	oldForceRefreshAfter := modelCache.ForceRefreshAfter
	oldInflight := modelCache.Inflight
	oldBuild := buildModelStatusForCache
	t.Cleanup(func() {
		rootDir = oldRootDir
		buildModelStatusForCache = oldBuild
		modelCache.Lock()
		modelCache.Value = oldValue
		modelCache.Expires = oldExpires
		modelCache.Key = oldKey
		modelCache.ForceRefreshAfter = oldForceRefreshAfter
		modelCache.Inflight = oldInflight
		modelCache.Unlock()
	})
	rootDir = t.TempDir()
	modelCache.Lock()
	modelCache.Value = nil
	modelCache.Expires = time.Time{}
	modelCache.Key = ""
	modelCache.ForceRefreshAfter = time.Time{}
	modelCache.Inflight = nil
	modelCache.Unlock()
	clearManagedSiteEnv(t)
	buildModelStatusForCache = func(ctx context.Context) *ModelStatus {
		panic("boom")
	}

	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("expected getModelStatus to propagate build panic")
			}
		}()
		_ = getModelStatus(context.Background(), false)
	}()

	modelCache.Lock()
	defer modelCache.Unlock()
	if len(modelCache.Inflight) != 0 {
		t.Fatalf("panic during build should clear inflight calls, got %#v", modelCache.Inflight)
	}
	if modelCache.Value != nil {
		t.Fatalf("panic during build should not populate cache: %#v", modelCache.Value)
	}
}

func managedSiteConfigJSON(name, url string) string {
	return fmt.Sprintf(`[{"name":%q,"url":%q,"token":"token"}]`, name, strings.TrimRight(url, "/"))
}
