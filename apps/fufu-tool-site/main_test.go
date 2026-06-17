package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeToolSiteFixture(t *testing.T, root string) {
	t.Helper()
	for _, dir := range []string{"frontend", "combine", "nav", "admin", filepath.Join("ui-dist", "assets"), filepath.Join("activity", "public")} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0755); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		filepath.Join("nav", "index.html"):                     `<html><title>导航</title><head><link rel="stylesheet" href="./nav-ui-tokens.css"></head><body>FuFu 工具站导航<script type="module" src="./theme.mjs"></script><script type="module" src="./latency.mjs"></script></body></html>`,
		filepath.Join("nav", "nav-ui-tokens.css"):              `/* fufu-navigation-ui-tokens */ :root { --fufu-nav-bg: var(--background); }`,
		filepath.Join("nav", "theme.mjs"):                      `export function initTheme() {}`,
		filepath.Join("nav", "latency.mjs"):                    `export function initLatencyProbes() {}`,
		filepath.Join("ui-dist", "index.html"):                 `<html><title>fufu 工具站</title><body><div id="root">fufu HeroUI shell</div><script type="module" src="/assets/app.js"></script></body></html>`,
		filepath.Join("ui-dist", "assets", "app.js"):           `import{HeroUIProvider}from"@heroui/react";console.log("fufu heroui app");`,
		filepath.Join("frontend", "index.html"):                `<html><title>fufu API 状态面板</title><body>状态面板<script type="module" src="/app.js"></script><link rel="stylesheet" href="/styles.css"></body></html>`,
		filepath.Join("frontend", "app.js"):                    `export const boot = true;`,
		filepath.Join("frontend", "styles.css"):                `.app{}`,
		filepath.Join("combine", "index.html"):                 `<html><body>合卡工具</body></html>`,
		filepath.Join("admin", "index.html"):                   `<html><title>fufu 管理面板</title><body><h1>fufu 管理面板</h1><a href="/status">API/模型状态</a><a href="/combine">合卡工具</a><a href="/activity">活动前台</a><section data-section="newapi-sites">NewAPI 站点配置</section><section data-section="activity-odds">活动日期 整体数学期望值 中奖率</section></body></html>`,
		filepath.Join("activity", "public", "index.html"):      `<html><body>活动中心<script src="activity-api.js"></script></body></html>`,
		filepath.Join("activity", "public", "admin.html"):      `<html><body>ADMIN PANEL<script src="admin-render.js"></script></body></html>`,
		filepath.Join("activity", "public", "activity-api.js"): `window.activityApi = {};`,
		filepath.Join("activity", "public", "admin-render.js"): `window.adminRender = {};`,
		filepath.Join("activity", "public", "scratch-card.js"): `window.scratchCard = {};`,
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestToolSiteMergedPageRoutes(t *testing.T) {
	root := t.TempDir()
	writeToolSiteFixture(t, root)
	if err := initRuntime(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(shutdownRuntime)

	for _, tc := range []struct {
		path string
		want string
	}{
		{path: "/", want: "fufu HeroUI shell"},
		{path: "/status", want: "状态面板"},
		{path: "/combine", want: "合卡工具"},
		{path: "/activity", want: "活动中心"},
		{path: "/admin", want: "fufu HeroUI shell"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			w := httptest.NewRecorder()
			route(w, req)
			if w.Code != http.StatusOK {
				t.Fatalf("%s code=%d body=%s", tc.path, w.Code, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), tc.want) {
				t.Fatalf("%s body %q does not contain %q", tc.path, w.Body.String(), tc.want)
			}
		})
	}
}

func TestToolSiteAdminShellIsIntegrated(t *testing.T) {
	root := t.TempDir()
	writeToolSiteFixture(t, root)
	if err := initRuntime(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(shutdownRuntime)

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	w := httptest.NewRecorder()
	route(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("admin code=%d body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, want := range []string{"fufu HeroUI shell", `src="/assets/app.js"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("admin body %q does not contain %q", body, want)
		}
	}
	if strings.Contains(body, "ADMIN PANEL") || strings.Contains(body, "<iframe") {
		t.Fatalf("/admin should serve one integrated configuration console, not embed the raw activity admin panel: %q", body)
	}
}

func TestToolSiteActivityAdminRoutesRedirectToUnifiedAdmin(t *testing.T) {
	root := t.TempDir()
	writeToolSiteFixture(t, root)
	if err := initRuntime(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(shutdownRuntime)

	for _, path := range []string{"/activity-admin", "/activity-admin/", "/activity-admin.html"} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			w := httptest.NewRecorder()
			route(w, req)
			if w.Code != http.StatusFound {
				t.Fatalf("%s code=%d body=%s", path, w.Code, w.Body.String())
			}
			if got := w.Header().Get("Location"); got != "/admin" {
				t.Fatalf("%s Location=%q, want /admin", path, got)
			}
			if strings.Contains(w.Body.String(), "ADMIN PANEL") {
				t.Fatalf("%s leaked raw activity admin body: %q", path, w.Body.String())
			}
		})
	}
}

func TestToolSiteServesActivityAssetsFromRootForMergedRoutes(t *testing.T) {
	root := t.TempDir()
	writeToolSiteFixture(t, root)
	if err := initRuntime(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(shutdownRuntime)

	for _, tc := range []struct {
		path string
		want string
	}{
		{path: "/activity-api.js", want: "window.activityApi"},
		{path: "/activity/activity-api.js", want: "window.activityApi"},
		{path: "/nav-ui-tokens.css", want: "fufu-navigation-ui-tokens"},
		{path: "/assets/app.js", want: "fufu heroui app"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			w := httptest.NewRecorder()
			route(w, req)
			if w.Code != http.StatusOK {
				t.Fatalf("%s code=%d body=%s", tc.path, w.Code, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), tc.want) {
				t.Fatalf("%s body %q does not contain %q", tc.path, w.Body.String(), tc.want)
			}
		})
	}
}

func TestToolSiteDoesNotServeLegacyActivityAdminAssets(t *testing.T) {
	root := t.TempDir()
	writeToolSiteFixture(t, root)
	if err := initRuntime(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(shutdownRuntime)

	for _, path := range []string{"/admin-render.js", "/activity/admin-render.js"} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			w := httptest.NewRecorder()
			route(w, req)
			if w.Code == http.StatusOK {
				t.Fatalf("%s should not serve legacy activity admin asset: body=%s", path, w.Body.String())
			}
			if strings.Contains(w.Body.String(), "window.adminRender") {
				t.Fatalf("%s leaked legacy activity admin asset body: %s", path, w.Body.String())
			}
		})
	}
}

func TestToolSiteNavigationUsesLocalMergedRoutes(t *testing.T) {
	root := t.TempDir()
	writeToolSiteFixture(t, root)
	if err := os.WriteFile(filepath.Join(root, "nav", "index.html"), []byte(`<html><title>fufu 工具站</title><body><a href="/status">API/模型状态</a><a href="/combine">合卡</a><a href="/activity">活动前台</a><a href="/admin">活动后台</a></body></html>`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := initRuntime(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(shutdownRuntime)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	route(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("home code=%d body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, want := range []string{"fufu HeroUI shell", `src="/assets/app.js"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("home body %q does not contain %q", body, want)
		}
	}
	if strings.Contains(body, "活动后台") {
		t.Fatalf("home should serve unified HeroUI app shell, not stale nav HTML: %q", body)
	}
}

func TestToolSiteMergedAPIRoutes(t *testing.T) {
	root := t.TempDir()
	writeToolSiteFixture(t, root)
	if err := initRuntime(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(shutdownRuntime)

	healthReq := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	healthW := httptest.NewRecorder()
	route(healthW, healthReq)
	if healthW.Code != http.StatusOK || !strings.Contains(healthW.Body.String(), `"ok":true`) {
		t.Fatalf("health code=%d body=%s", healthW.Code, healthW.Body.String())
	}

	adminReq := httptest.NewRequest(http.MethodGet, "/api/admin/stats", nil)
	adminW := httptest.NewRecorder()
	route(adminW, adminReq)
	if adminW.Code != http.StatusUnauthorized {
		t.Fatalf("admin stats without token code=%d body=%s", adminW.Code, adminW.Body.String())
	}

	dragonReq := httptest.NewRequest(http.MethodPost, "/api/dragonboat/start", strings.NewReader(`{}`))
	dragonReq.Header.Set("Content-Type", "application/json")
	dragonW := httptest.NewRecorder()
	route(dragonW, dragonReq)
	if dragonW.Code != http.StatusBadRequest || !strings.Contains(dragonW.Body.String(), "请输入卡密") {
		t.Fatalf("dragonboat API should be forwarded to activity app, code=%d body=%s", dragonW.Code, dragonW.Body.String())
	}
	if strings.Contains(dragonW.Body.String(), "API not found") {
		t.Fatalf("dragonboat API must not be handled by tool-site fallback: %s", dragonW.Body.String())
	}
}
