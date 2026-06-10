package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHandleModelTestMasksUnexpectedErrors(t *testing.T) {
	oldRunModelTest := runModelTest
	t.Cleanup(func() { runModelTest = oldRunModelTest })
	runModelTest = func(ctx context.Context, siteName, model, group string) (map[string]any, error) {
		return nil, errors.New("internal failure sk-secret http://10.0.0.5/config")
	}
	req := httptest.NewRequest(http.MethodPost, "/api/newapi/model-status/test", strings.NewReader(`{"siteName":"site-a","model":"gpt-test"}`))
	w := httptest.NewRecorder()

	handleModelTest(w, req)

	body := strings.TrimSpace(w.Body.String())
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("code=%d body=%s", w.Code, body)
	}
	if body != `{"error":"模型测试失败，请稍后重试"}` {
		t.Fatalf("unexpected safe error body: %s", body)
	}
	for _, leaked := range []string{"sk-secret", "10.0.0.5", "internal failure"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("unexpected model-test error leaked %q in %s", leaked, body)
		}
	}
}

func TestHandleModelTestPassesRequestContext(t *testing.T) {
	oldRunModelTest := runModelTest
	t.Cleanup(func() { runModelTest = oldRunModelTest })
	runModelTest = func(ctx context.Context, siteName, model, group string) (map[string]any, error) {
		if err := ctx.Err(); !errors.Is(err, context.Canceled) {
			t.Fatalf("runModelTest should receive request context cancellation, got %v", err)
		}
		return map[string]any{"ok": true}, nil
	}
	req := httptest.NewRequest(http.MethodPost, "/api/newapi/model-status/test", strings.NewReader(`{"siteName":"site-a","model":"gpt-test"}`))
	ctx, cancel := context.WithCancel(req.Context())
	cancel()
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handleModelTest(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestTestModelDoesNotStoreCooldownOrResultWhenCanceledDuringProbe(t *testing.T) {
	oldRootDir := rootDir
	t.Cleanup(func() { rootDir = oldRootDir })
	rootDir = t.TempDir()
	clearManagedSiteEnv(t)
	siteName := "cancel-site"
	modelName := "gpt-cancel-probe"
	key := modelManualKey(siteName, modelName, "")
	testCooldowns.Delete(key)
	testResults.Delete(key)
	t.Cleanup(func() {
		testCooldowns.Delete(key)
		testResults.Delete(key)
	})
	var cancel context.CancelFunc
	probeReached := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/channel/search"):
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": []any{
				map[string]any{"id": 7, "status": channelStatusEnabled, "models": []any{modelName}, "groups": []any{"default"}},
			}})
		case strings.HasPrefix(r.URL.Path, "/api/channel/test/"):
			close(probeReached)
			cancel()
			<-r.Context().Done()
		default:
			t.Fatalf("unexpected path: %s", r.URL.String())
		}
	}))
	t.Cleanup(server.Close)
	t.Setenv("NEWAPI_MANAGED_API_SITES", managedSiteConfigJSON(siteName, server.URL))
	ctx, cancelFn := context.WithCancel(context.Background())
	cancel = cancelFn

	_, err := testModel(ctx, siteName, modelName, "")

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %T %v", err, err)
	}
	select {
	case <-probeReached:
	default:
		t.Fatal("manual model probe was not reached")
	}
	if value, ok := testCooldowns.Load(key); ok {
		t.Fatalf("canceled probe should not leave cooldown: %#v", value)
	}
	if value, ok := testResults.Load(key); ok {
		t.Fatalf("canceled probe should not leave manual test result: %#v", value)
	}
}

func TestBuildModelStatusPrunesExpiredManualTestEntries(t *testing.T) {
	oldRootDir := rootDir
	t.Cleanup(func() { rootDir = oldRootDir })
	rootDir = t.TempDir()
	clearManagedSiteEnv(t)
	t.Setenv("NEWAPI_MANAGED_API_SITES", `[]`)
	key := modelManualKey("old-site", "old-model", "")
	expired := time.Now().Add(-time.Second).Unix()
	testCooldowns.Store(key, expired)
	testResults.Store(key, testRecord{OK: true, Status: "operational", TestedAt: expired, NextAllowedAt: expired})
	t.Cleanup(func() {
		testCooldowns.Delete(key)
		testResults.Delete(key)
	})

	_ = buildModelStatus(context.Background())

	if _, ok := testCooldowns.Load(key); ok {
		t.Fatal("expired manual-test cooldown should be pruned")
	}
	if _, ok := testResults.Load(key); ok {
		t.Fatal("expired manual-test result should be pruned")
	}
}

func TestPruneManualTestCacheDropsExpiredOrMalformedResults(t *testing.T) {
	now := time.Now().Unix()
	expiredKey := modelManualKey("site", "expired", "")
	futureKey := modelManualKey("site", "future", "")
	malformedKey := modelManualKey("site", "malformed", "")
	testResults.Store(expiredKey, testRecord{NextAllowedAt: now - 1})
	testResults.Store(futureKey, testRecord{NextAllowedAt: now + 60})
	testResults.Store(malformedKey, "bad")
	t.Cleanup(func() {
		for _, key := range []string{expiredKey, futureKey, malformedKey} {
			testCooldowns.Delete(key)
			testResults.Delete(key)
		}
	})

	pruneManualTestCache(now)

	if _, ok := testResults.Load(expiredKey); ok {
		t.Fatal("expired manual-test result should be pruned")
	}
	if _, ok := testResults.Load(malformedKey); ok {
		t.Fatal("malformed manual-test result should be pruned")
	}
	if _, ok := testResults.Load(futureKey); !ok {
		t.Fatal("future manual-test result should remain")
	}
}
