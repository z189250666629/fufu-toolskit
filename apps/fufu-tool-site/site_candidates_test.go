package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"fufu/newapi"
)

func TestExpandSitesForModelStatusUsesConnectivityOverridesForStandardSite(t *testing.T) {
	isolateManagedSiteRuntime(t)
	t.Setenv("NEWAPI_MANAGED_API_SITES", `[]`)
	t.Setenv("CONNECTIVITY_API_URLS", "https://api-a.example.test, https://api-b.example.test")

	expanded := expandSitesForModelStatus([]newapi.Site{
		{Name: "次数fufu", Category: "api", URL: "https://api-a.example.test/v1", Token: "token", UserID: "1"},
	})

	if len(expanded) != 2 {
		t.Fatalf("expanded sites = %#v", expanded)
	}
	if expanded[0].URL != "https://api-a.example.test" || expanded[1].URL != "https://api-b.example.test" {
		t.Fatalf("expanded URLs = %#v", []string{expanded[0].URL, expanded[1].URL})
	}
	if expanded[0].Name != "次数fufu" || expanded[1].Name != "次数fufu" {
		t.Fatalf("expanded names = %#v", []string{expanded[0].Name, expanded[1].Name})
	}
}

func TestOrderedManualTestSitesMovesPreferredURLFirst(t *testing.T) {
	isolateManagedSiteRuntime(t)
	t.Setenv("NEWAPI_MANAGED_API_SITES", `[]`)
	t.Setenv("CONNECTIVITY_API_URLS", "https://api-a.example.test,https://api-b.example.test")

	ordered := orderedManualTestSites(
		newapi.Site{Name: "次数fufu", Category: "api", URL: "https://api-a.example.test", Token: "token", UserID: "1"},
		"https://api-b.example.test/v1/models",
	)

	if len(ordered) != 2 {
		t.Fatalf("ordered sites = %#v", ordered)
	}
	if ordered[0].URL != "https://api-b.example.test" || ordered[1].URL != "https://api-a.example.test" {
		t.Fatalf("ordered URLs = %#v", []string{ordered[0].URL, ordered[1].URL})
	}
}

func TestTestModelFallsBackToNextConnectivityCandidate(t *testing.T) {
	isolateManagedSiteRuntime(t)
	siteName := "次数fufu"
	modelName := "gpt-fallback"
	group := "mix"
	key := modelManualKey(siteName, modelName, group)
	testCooldowns.Delete(key)
	testResults.Delete(key)
	t.Cleanup(func() {
		testCooldowns.Delete(key)
		testResults.Delete(key)
	})

	firstURL := "https://api-a.example.test"
	secondURL := "https://api-b.example.test"
	var testedPath string
	oldTransport := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "api-a.example.test" {
			return jsonResponse(http.StatusBadGateway, map[string]any{"success": false, "error": "first line is down"}), nil
		}
		if r.URL.Host != "api-b.example.test" {
			t.Fatalf("unexpected host: %s", r.URL.Host)
		}
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/channel/search"):
			return jsonResponse(http.StatusOK, map[string]any{"success": true, "data": []any{
				map[string]any{"id": 7, "status": channelStatusEnabled, "models": []any{modelName}, "groups": []any{group}},
			}}), nil
		case strings.HasPrefix(r.URL.Path, "/api/channel/test/"):
			testedPath = r.URL.String()
			return jsonResponse(http.StatusOK, map[string]any{"success": true}), nil
		default:
			t.Fatalf("unexpected path on fallback line: %s", r.URL.String())
		}
		return jsonResponse(http.StatusNotFound, map[string]any{"success": false}), nil
	})
	t.Cleanup(func() { http.DefaultTransport = oldTransport })

	t.Setenv("NEWAPI_MANAGED_API_SITES", managedSiteConfigJSON(t, siteName, firstURL))
	t.Setenv("CONNECTIVITY_API_URLS", firstURL+","+secondURL)

	result, err := testModel(contextWithModelTestPreferredURL(t.Context(), secondURL), siteName, modelName, group)
	if err != nil {
		t.Fatalf("testModel fallback err = %v", err)
	}
	if result["siteName"] != siteName || result["model"] != modelName {
		t.Fatalf("unexpected result = %#v", result)
	}
	if !strings.Contains(testedPath, "/api/channel/test/7") {
		t.Fatalf("fallback line was not tested, path=%q", testedPath)
	}
}

func TestTestModelAllowsParallelDifferentModelsFromSameClient(t *testing.T) {
	isolateManagedSiteRuntime(t)
	siteName := "parallel-site"
	models := []string{"model-a", "model-b"}
	for _, modelName := range models {
		key := modelManualKey(siteName, modelName, "")
		testCooldowns.Delete(key)
		testResults.Delete(key)
		t.Cleanup(func() {
			testCooldowns.Delete(key)
			testResults.Delete(key)
		})
	}

	var probeHits atomic.Int32
	bothProbesStarted := make(chan struct{})
	var closeBoth sync.Once
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/channel/search"):
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": []any{
				map[string]any{"id": 11, "status": channelStatusEnabled, "models": []any{"model-a"}, "groups": []any{"default"}},
				map[string]any{"id": 12, "status": channelStatusEnabled, "models": []any{"model-b"}, "groups": []any{"default"}},
			}})
		case strings.HasPrefix(r.URL.Path, "/api/channel/test/"):
			if probeHits.Add(1) == 2 {
				closeBoth.Do(func() { close(bothProbesStarted) })
			}
			select {
			case <-bothProbesStarted:
				_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
			case <-time.After(2 * time.Second):
				http.Error(w, "parallel probe did not start", http.StatusGatewayTimeout)
			}
		default:
			t.Fatalf("unexpected path: %s", r.URL.String())
		}
	}))
	t.Cleanup(server.Close)

	t.Setenv("NEWAPI_MANAGED_API_SITES", managedSiteConfigJSON(t, siteName, server.URL))
	type response struct {
		code int
		body string
	}
	responses := make(chan response, len(models))
	for _, modelName := range models {
		modelName := modelName
		go func() {
			req := httptest.NewRequest(http.MethodPost, "/api/newapi/model-status/test", strings.NewReader(`{"siteName":"`+siteName+`","model":"`+modelName+`"}`))
			req.RemoteAddr = "203.0.113.55:5000"
			rec := httptest.NewRecorder()
			handleModelTest(rec, req)
			responses <- response{code: rec.Code, body: rec.Body.String()}
		}()
	}
	for range models {
		select {
		case res := <-responses:
			if res.code != http.StatusOK {
				t.Fatalf("parallel model test should succeed, status=%d body=%s", res.code, res.body)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("parallel model tests did not finish")
		}
	}
	if got := probeHits.Load(); got != 2 {
		t.Fatalf("parallel model tests should probe both models, got %d", got)
	}
}

func TestExpandSitesForModelStatusDoesNotExpandNonPublicPrivateSite(t *testing.T) {
	isolateManagedSiteRuntime(t)
	expanded := expandSitesForModelStatus([]newapi.Site{
		{Name: "private-site", Category: "api", URL: "http://127.0.0.1:3000", Token: "token", UserID: "1"},
	})

	if len(expanded) != 1 {
		t.Fatalf("expanded sites = %#v", expanded)
	}
	if expanded[0].URL != "http://127.0.0.1:3000" {
		t.Fatalf("expanded URL = %q", expanded[0].URL)
	}
}

func isolateManagedSiteRuntime(t *testing.T) {
	t.Helper()
	oldRootDir := rootDir
	oldUnifiedConfig := unifiedConfig
	rootDir = t.TempDir()
	unifiedConfig = nil
	t.Cleanup(func() {
		rootDir = oldRootDir
		unifiedConfig = oldUnifiedConfig
	})
	for _, name := range []string{
		"NEWAPI_MANAGED_API_SITES",
		"NEWAPI_MANAGED_API_CONFIG",
		"NEWAPI_API_SITE_URL",
		"NEWAPI_API_SITE_TOKEN",
		"NEWAPI_API_SITE_ACCESS_TOKEN",
		"NEWAPI_TOKEN_SITE_URL",
		"NEWAPI_TOKEN_SITE_TOKEN",
		"NEWAPI_TOKEN_SITE_ACCESS_TOKEN",
		"CONNECTIVITY_API_URLS",
		"FUFU_API_URLS",
		"CONNECTIVITY_TOKEN_URLS",
		"FUFU_TOKEN_URLS",
	} {
		t.Setenv(name, "")
	}
}

func managedSiteConfigJSON(t *testing.T, name, rawURL string) string {
	t.Helper()
	payload := map[string]any{
		"managedApiSites": []map[string]any{
			{
				"name":                name,
				"category":            "api",
				"url":                 rawURL,
				"token":               "token",
				"userId":              "1",
				"channelListEndpoint": "/api/channel/search?keyword=&p=1&page_size=500",
				"quotaUnit":           500000,
				"currency":            "$",
				"rechargeRatio":       1,
				"skipUserHeader":      false,
				"note":                "test",
			},
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func jsonResponse(status int, payload map[string]any) *http.Response {
	raw, _ := json.Marshal(payload)
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(raw)),
	}
}
