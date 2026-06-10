package main

import (
	"encoding/json"
	"errors"
	"fufu/combine"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHealth(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()
	handleAPI(w, req)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"ok":true`) {
		t.Fatalf("health code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestServeReturnsListenerErrors(t *testing.T) {
	listener, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	_, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	err = serve(port, http.NewServeMux())

	if err == nil {
		t.Fatal("serve should return bind errors instead of panicking")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "bind") && !strings.Contains(strings.ToLower(err.Error()), "address already in use") {
		t.Fatalf("serve returned unexpected error: %v", err)
	}
}

func TestHTTPServerHasTimeouts(t *testing.T) {
	server := newHTTPServer("8080", http.NewServeMux())
	if server.Addr != "0.0.0.0:8080" {
		t.Fatalf("Addr = %q", server.Addr)
	}
	if server.ReadHeaderTimeout <= 0 || server.ReadTimeout <= 0 || server.WriteTimeout <= 0 || server.IdleTimeout <= 0 {
		t.Fatalf("server timeouts should be set: %#v", server)
	}
}

func TestInitRuntimeReturnsDataDirectoryErrors(t *testing.T) {
	oldRoot, oldFrontend, oldCombine := rootDir, frontendDir, combineDir
	oldApp, oldErr := combineApp, combineConfigErr
	t.Cleanup(func() {
		rootDir, frontendDir, combineDir = oldRoot, oldFrontend, oldCombine
		combineApp, combineConfigErr = oldApp, oldErr
	})
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "data"), []byte("not a directory"), 0644); err != nil {
		t.Fatal(err)
	}

	err := initRuntime(tmp)

	if err == nil {
		t.Fatal("initRuntime should report data directory creation errors")
	}
	if !strings.Contains(err.Error(), "create data directory") {
		t.Fatalf("initRuntime error = %v, want create data directory context", err)
	}
}

func TestCombinePageServed(t *testing.T) {
	tmp := t.TempDir()
	rootDir = tmp
	frontendDir = filepath.Join(tmp, "frontend")
	combineDir = filepath.Join(tmp, "combine")
	if err := os.MkdirAll(combineDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(combineDir, "index.html"), []byte("combine page"), 0644); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/combine", nil)
	w := httptest.NewRecorder()
	route(w, req)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "combine page") {
		t.Fatalf("combine page code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestStaticMissingAssetsDoNotUseSPAFallback(t *testing.T) {
	oldRoot, oldFrontend, oldCombine := rootDir, frontendDir, combineDir
	t.Cleanup(func() {
		rootDir, frontendDir, combineDir = oldRoot, oldFrontend, oldCombine
	})
	tmp := t.TempDir()
	rootDir = tmp
	frontendDir = filepath.Join(tmp, "frontend")
	combineDir = filepath.Join(tmp, "combine")
	if err := os.MkdirAll(frontendDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(frontendDir, "index.html"), []byte("spa shell"), 0644); err != nil {
		t.Fatal(err)
	}

	assetReq := httptest.NewRequest(http.MethodGet, "/missing.js", nil)
	assetRec := httptest.NewRecorder()
	route(assetRec, assetReq)
	if assetRec.Code != http.StatusNotFound {
		t.Fatalf("missing asset code=%d body=%s", assetRec.Code, assetRec.Body.String())
	}

	spaReq := httptest.NewRequest(http.MethodGet, "/models", nil)
	spaRec := httptest.NewRecorder()
	route(spaRec, spaReq)
	if spaRec.Code != http.StatusOK || !strings.Contains(spaRec.Body.String(), "spa shell") {
		t.Fatalf("spa fallback code=%d body=%s", spaRec.Code, spaRec.Body.String())
	}
}

func TestStaticRouteRejectsTestArtifactsAndDotfiles(t *testing.T) {
	oldRoot, oldFrontend, oldCombine := rootDir, frontendDir, combineDir
	t.Cleanup(func() {
		rootDir, frontendDir, combineDir = oldRoot, oldFrontend, oldCombine
	})
	tmp := t.TempDir()
	rootDir = tmp
	frontendDir = filepath.Join(tmp, "frontend")
	combineDir = filepath.Join(tmp, "combine")
	if err := os.MkdirAll(frontendDir, 0755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"api.test.mjs":     "secret test marker",
		".env.local":       "secret env marker",
		"app.js":           "public app marker",
		"assets/style.css": "public css marker",
	} {
		file := filepath.Join(frontendDir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}

	for _, path := range []string{"/api.test.mjs", "/.env.local"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		route(w, req)
		if w.Code == http.StatusOK {
			t.Fatalf("%s should not be served: body=%s", path, w.Body.String())
		}
		if strings.Contains(w.Body.String(), "secret") {
			t.Fatalf("%s leaked secret marker: %s", path, w.Body.String())
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/app.js", nil)
	w := httptest.NewRecorder()
	route(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "public app marker") {
		t.Fatalf("normal asset should still be served: code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestStaticRouteRejectsUnsafeMethodsWithAllowHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	w := httptest.NewRecorder()

	route(w, req)

	if w.Code != http.StatusMethodNotAllowed || strings.TrimSpace(w.Body.String()) != `{"error":"Only GET is supported"}` {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Allow"); got != "GET, HEAD" {
		t.Fatalf("Allow = %q", got)
	}
}

func TestCombineAPIRoutes(t *testing.T) {
	for _, path := range []string{
		"/api/auth",
		"/api/session",
		"/api/search-keys",
		"/api/merge",
		"/api/public-merge",
		"/api/generate",
		"/api/merge-status/job-1",
		"/api/token/1",
	} {
		if !isCombineAPI(path) {
			t.Fatalf("%s should be routed to combine app", path)
		}
	}
	if isCombineAPI("/api/health") {
		t.Fatalf("network health endpoint should not be routed to combine app")
	}
	for _, path := range []string{"/api/auth", "/api/session", "/api/search-keys", "/api/merge", "/api/public-merge", "/api/generate", "/api/merge-status/job-1", "/api/token/1", "/api/health"} {
		if isCombineAPI(path) != combine.IsAPIPath(path) {
			t.Fatalf("%s route mismatch: network=%v combine=%v", path, isCombineAPI(path), combine.IsAPIPath(path))
		}
	}
}

func TestCombineAPIUnavailableDoesNotExposeConfigError(t *testing.T) {
	oldApp, oldErr := combineApp, combineConfigErr
	combineApp = nil
	combineConfigErr = errors.New(`secret token=sk-test C:\data\combine-trace.db`)
	t.Cleanup(func() {
		combineApp = oldApp
		combineConfigErr = oldErr
	})

	req := httptest.NewRequest(http.MethodPost, "/api/auth", strings.NewReader(`{}`))
	w := httptest.NewRecorder()

	handleAPI(w, req)

	body := strings.TrimSpace(w.Body.String())
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("code=%d body=%s", w.Code, body)
	}
	if body != `{"error":"combine is not configured"}` {
		t.Fatalf("unexpected public error body: %s", body)
	}
	for _, leaked := range []string{"sk-test", "token=", `C:\`, "combine-trace.db"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("public combine unavailable error leaked %q in %s", leaked, body)
		}
	}
}

func TestNetworkAPIRouteMethods(t *testing.T) {
	cases := []struct {
		path   string
		method string
	}{
		{path: "/api/health", method: http.MethodGet},
		{path: "/api/client", method: http.MethodGet},
		{path: "/api/connectivity/targets", method: http.MethodGet},
		{path: "/api/newapi/sites", method: http.MethodGet},
		{path: "/api/newapi/model-status", method: http.MethodGet},
		{path: "/api/newapi/overview", method: http.MethodGet},
		{path: "/api/newapi/model-status/test", method: http.MethodPost},
	}

	for _, tc := range cases {
		route, ok := findNetworkAPIPath(tc.path)
		if !ok {
			t.Fatalf("%s should be a network API route", tc.path)
		}
		if route.Method != tc.method {
			t.Fatalf("%s method=%s, want %s", tc.path, route.Method, tc.method)
		}
	}
	for _, path := range []string{"/api/auth", "/api/merge-status/job-1", "/api/unknown"} {
		if _, ok := findNetworkAPIPath(path); ok {
			t.Fatalf("%s should not be a network-owned API route", path)
		}
	}
}

func TestHandleAPIRejectsWrongNetworkMethodBeforeHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/newapi/sites", nil)
	w := httptest.NewRecorder()

	handleAPI(w, req)

	if w.Code != http.StatusMethodNotAllowed || strings.TrimSpace(w.Body.String()) != `{"error":"Only GET is supported"}` {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestConnectivityTargetsReturnsConfigErrorForInvalidInlineJSON(t *testing.T) {
	clearConnectivityEnv(t)
	t.Setenv("CONNECTIVITY_TARGETS", "not-json")
	req := httptest.NewRequest(http.MethodGet, "/api/connectivity/targets", nil)
	w := httptest.NewRecorder()

	handleAPI(w, req)

	body := strings.TrimSpace(w.Body.String())
	if w.Code != http.StatusInternalServerError || body != `{"error":"CONNECTIVITY_TARGETS 不是有效 JSON"}` {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	for _, leaked := range []string{"invalid character", "literal", "not-json"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("connectivity config error leaked %q in %s", leaked, body)
		}
	}
}

func TestConnectivityTargetsFallsBackWhenInlineJSONUnset(t *testing.T) {
	clearConnectivityEnv(t)
	req := httptest.NewRequest(http.MethodGet, "/api/connectivity/targets", nil)
	w := httptest.NewRecorder()

	handleAPI(w, req)

	body := w.Body.String()
	if w.Code != http.StatusOK || !strings.Contains(body, "api.fufuapi.top") || !strings.Contains(body, "api.fufuapi.online") || !strings.Contains(body, "token.fufuapi.online") {
		t.Fatalf("code=%d body=%s", w.Code, body)
	}
}

func TestNewAPISitesMasksManagedSiteConfigParseErrors(t *testing.T) {
	oldRootDir := rootDir
	t.Cleanup(func() { rootDir = oldRootDir })
	rootDir = t.TempDir()
	clearManagedSiteEnv(t)
	t.Setenv("NEWAPI_MANAGED_API_SITES", `not-json`)
	req := httptest.NewRequest(http.MethodGet, "/api/newapi/sites", nil)
	w := httptest.NewRecorder()

	handleAPI(w, req)

	body := strings.TrimSpace(w.Body.String())
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("code=%d body=%s", w.Code, body)
	}
	var payload struct {
		Configured bool   `json:"configured"`
		Error      string `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("invalid JSON response: %v body=%s", err, body)
	}
	if payload.Configured || payload.Error != "NEWAPI_MANAGED_API_SITES 不是有效 JSON" {
		t.Fatalf("unexpected payload: %#v body=%s", payload, body)
	}
	for _, leaked := range []string{"invalid character", "literal", "not-json"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("managed site config error leaked %q in %s", leaked, body)
		}
	}
}

func TestNewAPISitesMasksExplicitManagedSiteConfigPath(t *testing.T) {
	oldRootDir := rootDir
	t.Cleanup(func() { rootDir = oldRootDir })
	rootDir = t.TempDir()
	clearManagedSiteEnv(t)
	t.Setenv("NEWAPI_MANAGED_API_CONFIG", filepath.Join(t.TempDir(), "sk-secret-managed-sites.json"))
	req := httptest.NewRequest(http.MethodGet, "/api/newapi/sites", nil)
	w := httptest.NewRecorder()

	handleAPI(w, req)

	body := strings.TrimSpace(w.Body.String())
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("code=%d body=%s", w.Code, body)
	}
	var payload struct {
		Configured bool   `json:"configured"`
		Error      string `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("invalid JSON response: %v body=%s", err, body)
	}
	if payload.Configured || payload.Error != "NEWAPI_MANAGED_API_CONFIG 读取失败" {
		t.Fatalf("unexpected payload: %#v body=%s", payload, body)
	}
	for _, leaked := range []string{"sk-secret", "managed-sites.json", rootDir, "open ", "no such file"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("explicit config error leaked %q in %s", leaked, body)
		}
	}
}

func TestNewAPISitesDoesNotExposeRawManagedSiteURLs(t *testing.T) {
	oldRootDir := rootDir
	t.Cleanup(func() { rootDir = oldRootDir })
	rootDir = t.TempDir()
	clearManagedSiteEnv(t)
	rawURL := "http://10.0.0.5:3000/admin"
	t.Setenv("NEWAPI_MANAGED_API_SITES", `[{"name":"private-site","url":"`+rawURL+`","token":"sk-private"}]`)
	req := httptest.NewRequest(http.MethodGet, "/api/newapi/sites", nil)
	w := httptest.NewRecorder()

	handleAPI(w, req)

	body := w.Body.String()
	if w.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", w.Code, body)
	}
	for _, leaked := range []string{rawURL, "10.0.0.5", "3000", "sk-private"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("public managed-site response leaked %q in %s", leaked, body)
		}
	}
	if !strings.Contains(body, `"displayUrl"`) {
		t.Fatalf("response should provide a safe displayUrl instead of raw url: %s", body)
	}
}

func TestNewAPISitesDoesNotExposeManagedSiteMetadataSecrets(t *testing.T) {
	oldRootDir := rootDir
	t.Cleanup(func() { rootDir = oldRootDir })
	rootDir = t.TempDir()
	clearManagedSiteEnv(t)
	t.Setenv("NEWAPI_MANAGED_API_SITES", `[{"name":"private-site","url":"http://127.0.0.1:3000","token":"sk-private","channelListEndpoint":"/api/channel/search?token=sk-endpoint-secret","note":"internal note sk-note-secret http://10.0.0.9"}]`)
	req := httptest.NewRequest(http.MethodGet, "/api/newapi/sites", nil)
	w := httptest.NewRecorder()

	handleAPI(w, req)

	body := w.Body.String()
	if w.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", w.Code, body)
	}
	for _, leaked := range []string{"channelListEndpoint", "note", "sk-endpoint-secret", "sk-note-secret", "10.0.0.9", "/api/channel/search"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("public managed-site metadata leaked %q in %s", leaked, body)
		}
	}
}

func TestNewAPIStatusEndpointsDoNotExposeRawManagedSiteURLs(t *testing.T) {
	oldRootDir := rootDir
	oldValue := modelCache.Value
	oldExpires := modelCache.Expires
	oldKey := modelCache.Key
	oldInflight := modelCache.Inflight
	t.Cleanup(func() {
		rootDir = oldRootDir
		modelCache.Lock()
		modelCache.Value = oldValue
		modelCache.Expires = oldExpires
		modelCache.Key = oldKey
		modelCache.Inflight = oldInflight
		modelCache.Unlock()
	})
	rootDir = t.TempDir()
	modelCache.Lock()
	modelCache.Value = nil
	modelCache.Expires = time.Time{}
	modelCache.Key = ""
	modelCache.Inflight = nil
	modelCache.Unlock()
	clearManagedSiteEnv(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"success": true, "data": []any{}})
	}))
	defer server.Close()
	t.Setenv("NEWAPI_MANAGED_API_SITES", `[{"name":"private-site","url":"`+server.URL+`","token":"token","channelListEndpoint":"/api/channel/search?token=sk-status-endpoint","note":"internal status note sk-status-note http://10.0.0.9"}]`)

	for _, path := range []string{"/api/newapi/model-status", "/api/newapi/overview"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()

		handleAPI(w, req)

		body := w.Body.String()
		if w.Code != http.StatusOK {
			t.Fatalf("%s code=%d body=%s", path, w.Code, body)
		}
		for _, leaked := range []string{server.URL, "127.0.0.1", "localhost", "channelListEndpoint", "note", "sk-status-endpoint", "sk-status-note", "10.0.0.9"} {
			if strings.Contains(body, leaked) {
				t.Fatalf("%s leaked raw managed-site URL detail %q in %s", path, leaked, body)
			}
		}
	}
}

func TestConnectivityTargetsDoNotExposePrivateNewAPIFallbackURLs(t *testing.T) {
	clearConnectivityEnv(t)
	t.Setenv("NEWAPI_API_SITE_URL", "http://10.0.0.5:3000")
	req := httptest.NewRequest(http.MethodGet, "/api/connectivity/targets", nil)
	w := httptest.NewRecorder()

	handleAPI(w, req)

	body := w.Body.String()
	if w.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", w.Code, body)
	}
	for _, leaked := range []string{"10.0.0.5", "3000", "http://10.0.0.5:3000"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("connectivity targets leaked private NewAPI fallback URL %q in %s", leaked, body)
		}
	}
	if !strings.Contains(body, "api.fufuapi.top") {
		t.Fatalf("private NewAPI fallback should fall back to public defaults, got %s", body)
	}
}

func TestConnectivityTargetsKeepExplicitPrivateConnectivityURLs(t *testing.T) {
	clearConnectivityEnv(t)
	t.Setenv("CONNECTIVITY_API_URLS", "http://10.0.0.5:3000")
	req := httptest.NewRequest(http.MethodGet, "/api/connectivity/targets", nil)
	w := httptest.NewRecorder()

	handleAPI(w, req)

	body := w.Body.String()
	if w.Code != http.StatusOK || !strings.Contains(body, "10.0.0.5") {
		t.Fatalf("explicit connectivity URL should be preserved for browser checks: code=%d body=%s", w.Code, body)
	}
}

func clearConnectivityEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		"CONNECTIVITY_TARGETS",
		"CONNECTIVITY_API_URLS",
		"FUFU_API_URLS",
		"NEWAPI_API_SITE_URL",
		"CONNECTIVITY_TOKEN_URLS",
		"FUFU_TOKEN_URLS",
		"NEWAPI_TOKEN_SITE_URL",
		"CONNECTIVITY_API_NAME",
		"CONNECTIVITY_TOKEN_NAME",
	} {
		t.Setenv(name, "")
	}
}

func TestWriteJSONErrorUsesStablePayload(t *testing.T) {
	w := httptest.NewRecorder()

	writeJSONError(w, http.StatusTeapot, "bad request")

	if w.Code != http.StatusTeapot || strings.TrimSpace(w.Body.String()) != `{"error":"bad request"}` {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestRequireMethodUsesNetworkMethodMessage(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/newapi/model-status/test", nil)

	if requireMethod(w, req, http.MethodPost) {
		t.Fatal("GET should be rejected when POST is required")
	}
	if w.Code != http.StatusMethodNotAllowed || strings.TrimSpace(w.Body.String()) != `{"error":"Only POST is supported"}` {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestReadJSONRejectsTrailingJSONValue(t *testing.T) {
	var body struct {
		SiteName string `json:"siteName"`
	}
	req := httptest.NewRequest(http.MethodPost, "/api/newapi/model-status/test", strings.NewReader(`{"siteName":"a"} {}`))

	if err := readJSON(req, &body); err == nil {
		t.Fatal("expected trailing JSON value to be rejected")
	}
}

func TestReadJSONAllowsTrailingWhitespace(t *testing.T) {
	var body struct {
		SiteName string `json:"siteName"`
	}
	req := httptest.NewRequest(http.MethodPost, "/api/newapi/model-status/test", strings.NewReader("{\"siteName\":\"a\"}\n\t "))

	if err := readJSON(req, &body); err != nil {
		t.Fatalf("readJSON: %v", err)
	}
}

func TestParseListCleansSortsAndDedupes(t *testing.T) {
	got := parseList(`["beta","alpha","beta",""]`)
	if strings.Join(got, ",") != "alpha,beta" {
		t.Fatalf("json parseList = %#v", got)
	}

	got = parseList(" beta,alpha beta|gamma ")
	if strings.Join(got, ",") != "alpha,beta,gamma" {
		t.Fatalf("text parseList = %#v", got)
	}
}

func TestModelStatusClassifiers(t *testing.T) {
	if got := statusFromCounts(0, 0); got != "unknown" {
		t.Fatalf("empty status = %s", got)
	}
	if got := statusFromCounts(4, 1); got != "operational" {
		t.Fatalf("mostly successful status = %s", got)
	}
	if got := statusFromCounts(1, 3); got != "degraded" {
		t.Fatalf("mixed status = %s", got)
	}
	if got := statusFromCounts(0, 3); got != "down" {
		t.Fatalf("failed status = %s", got)
	}

	rowStatus := modelRowStatus([]*ModelCell{{Configured: true, Status: "operational"}, {Configured: true, Status: "degraded"}})
	if rowStatus != "degraded" {
		t.Fatalf("row status = %s", rowStatus)
	}
}
