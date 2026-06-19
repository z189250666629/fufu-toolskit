package main

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	activityapp "fufu-act"
	"fufu/newapi"
)

func TestAdminConfigAPIRequiresAdminToken(t *testing.T) {
	root := t.TempDir()
	writeToolSiteFixture(t, root)
	t.Setenv("ADMIN_TOKEN", "secret-admin-token")
	if err := initRuntime(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(shutdownRuntime)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/config", nil)
	w := httptest.NewRecorder()
	route(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("config without token code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestAdminSessionLoginSetsCookieAndAuthorizesConfigAPI(t *testing.T) {
	root := t.TempDir()
	writeToolSiteFixture(t, root)
	t.Setenv("ADMIN_TOKEN", "secret-admin-token")
	if err := initRuntime(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(shutdownRuntime)

	badReq := jsonRequest(t, http.MethodPost, "/api/admin/session", map[string]any{"token": "wrong-password"})
	badW := httptest.NewRecorder()
	route(badW, badReq)
	if badW.Code != http.StatusUnauthorized {
		t.Fatalf("bad login code=%d body=%s", badW.Code, badW.Body.String())
	}

	loginReq := jsonRequest(t, http.MethodPost, "/api/admin/session", map[string]any{"token": "secret-admin-token"})
	loginW := httptest.NewRecorder()
	route(loginW, loginReq)
	if loginW.Code != http.StatusOK {
		t.Fatalf("login code=%d body=%s", loginW.Code, loginW.Body.String())
	}
	cookie := adminSessionCookieFromRecorder(t, loginW)
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode || cookie.Path != "/" || cookie.MaxAge <= 0 {
		t.Fatalf("admin session cookie should be HttpOnly/Lax/path=/ with max-age, got %#v", cookie)
	}

	sessionReq := httptest.NewRequest(http.MethodGet, "/api/admin/session", nil)
	sessionReq.AddCookie(cookie)
	sessionW := httptest.NewRecorder()
	route(sessionW, sessionReq)
	if sessionW.Code != http.StatusOK || !strings.Contains(sessionW.Body.String(), `"authenticated":true`) {
		t.Fatalf("session check code=%d body=%s", sessionW.Code, sessionW.Body.String())
	}

	configReq := httptest.NewRequest(http.MethodGet, "/api/admin/config", nil)
	configReq.AddCookie(cookie)
	configW := httptest.NewRecorder()
	route(configW, configReq)
	if configW.Code != http.StatusOK {
		t.Fatalf("config with session cookie code=%d body=%s", configW.Code, configW.Body.String())
	}
}

func TestAdminSessionCookieAuthorizesForwardedActivityAdminAPIs(t *testing.T) {
	root := t.TempDir()
	writeToolSiteFixture(t, root)
	t.Setenv("ADMIN_TOKEN", "secret-admin-token")
	if err := initRuntime(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(shutdownRuntime)

	loginReq := jsonRequest(t, http.MethodPost, "/api/admin/session", map[string]any{"token": "secret-admin-token"})
	loginW := httptest.NewRecorder()
	route(loginW, loginReq)
	if loginW.Code != http.StatusOK {
		t.Fatalf("login code=%d body=%s", loginW.Code, loginW.Body.String())
	}
	cookie := adminSessionCookieFromRecorder(t, loginW)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/stats", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	route(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("forwarded stats with session cookie code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestAdminLoginRejectedWhenTokenUnset(t *testing.T) {
	root := t.TempDir()
	writeToolSiteFixture(t, root)
	t.Setenv("ADMIN_TOKEN", "")
	if err := initRuntime(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(shutdownRuntime)

	// 未配置 ADMIN_TOKEN 时，任何口令都不能登录后台（不存在硬编码后门）。
	for _, token := range []string{"", "Cky98", "anything"} {
		loginReq := jsonRequest(t, http.MethodPost, "/api/admin/session", map[string]any{"token": token})
		loginW := httptest.NewRecorder()
		route(loginW, loginReq)
		if loginW.Code != http.StatusUnauthorized {
			t.Fatalf("login with %q while ADMIN_TOKEN unset should be 401, got %d body=%s", token, loginW.Code, loginW.Body.String())
		}
	}
}

func TestAdminSessionLoginRateLimitsRepeatedFailures(t *testing.T) {
	root := t.TempDir()
	writeToolSiteFixture(t, root)
	t.Setenv("ADMIN_TOKEN", "secret-admin-token")
	if err := initRuntime(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(shutdownRuntime)

	for i := 0; i < adminLoginFailureLimit; i++ {
		req := jsonRequest(t, http.MethodPost, "/api/admin/session", map[string]any{"token": "wrong-password"})
		req.RemoteAddr = "203.0.113.88:5000"
		w := httptest.NewRecorder()
		route(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d code=%d body=%s", i+1, w.Code, w.Body.String())
		}
	}
	blockedReq := jsonRequest(t, http.MethodPost, "/api/admin/session", map[string]any{"token": "secret-admin-token"})
	blockedReq.RemoteAddr = "203.0.113.88:5000"
	blockedW := httptest.NewRecorder()
	route(blockedW, blockedReq)
	if blockedW.Code != http.StatusTooManyRequests {
		t.Fatalf("blocked login code=%d body=%s", blockedW.Code, blockedW.Body.String())
	}
	if blockedW.Header().Get("Retry-After") == "" {
		t.Fatal("rate-limited admin login should include Retry-After")
	}
	if !strings.Contains(blockedW.Body.String(), "管理员登录尝试过多") {
		t.Fatalf("unexpected rate-limit body: %s", blockedW.Body.String())
	}

	otherReq := jsonRequest(t, http.MethodPost, "/api/admin/session", map[string]any{"token": "secret-admin-token"})
	otherReq.RemoteAddr = "198.51.100.88:5000"
	otherW := httptest.NewRecorder()
	route(otherW, otherReq)
	if otherW.Code != http.StatusOK {
		t.Fatalf("another client should not be rate limited, code=%d body=%s", otherW.Code, otherW.Body.String())
	}
}

func TestProductionAdminShellReferencesActualBusinessAPIs(t *testing.T) {
	html := readToolSiteUISource(t)
	for _, want := range []string{
		"活动统计",
		"补卡 / MCY 库存检测（暂时下线）",
		"当前奖池中奖率",
		"/api/admin/stats",
		"/api/admin/sale-cards/config",
		"/api/admin/sale-cards/test-key",
		"/api/prizes",
		"/api/newapi/sites",
		"/api/nav/lines",
		"/api/nav/tools",
		"首页导航线路",
		"首页卡片",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("production admin shell should reference %q", want)
		}
	}
	if strings.Contains(html, "<iframe") {
		t.Fatalf("production admin shell should be natively integrated, not iframe-based")
	}
	for _, want := range []string{
		"管理员登录",
		"/api/admin/session",
		"authenticated",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("production admin shell should contain login-gated UX marker %q", want)
		}
	}
	for _, notWant := range []string{
		"/api/admin/sale-cards/run",
		`id="admin-token"`,
		"输入 ADMIN_TOKEN",
		"Authorization: 'Bearer '",
	} {
		if strings.Contains(html, notWant) {
			t.Fatalf("production admin shell should not expose inline token auth marker %q", notWant)
		}
	}
}

func TestProductionAdminShellDoesNotSwallowBusinessDataLoadFailures(t *testing.T) {
	html := readToolSiteUISource(t)
	for _, notWant := range []string{
		"catch(() => undefined)",
		"配置已加载', tone: 'ok' });",
	} {
		if strings.Contains(html, notWant) {
			t.Fatalf("production admin shell should not silently swallow business-data load failures with marker %q", notWant)
		}
	}
	for _, want := range []string{
		"部分业务数据加载失败",
		"loadBusinessData",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("production admin shell should expose partial-load failure handling marker %q", want)
		}
	}
}

func TestProductionAdminShellGroupsOverlappingConfigIntoTwoTabs(t *testing.T) {
	html := readToolSiteUISource(t)
	for _, want := range []string{
		`site-replenish`,
		"状态页 / 合卡 / 活动卡档",
		"状态页实际站点",
		"NewAPI 站点配置",
		"补卡 / MCY 库存检测（暂时下线）",
		`activity`,
		"活动统计",
		"当前奖池中奖率",
		"活动配置",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("production admin shell should group config by tabs and include %q", want)
		}
	}
	for _, notWant := range []string{
		`data-section="business-dashboard"`,
		`data-section="sale-cards"`,
		`data-section="current-prizes"`,
		`data-section="status-runtime"`,
		`data-section="newapi-sites"`,
		`data-section="activity-odds"`,
	} {
		if strings.Contains(html, notWant) {
			t.Fatalf("production admin shell should not expose scattered section marker %q", notWant)
		}
	}
}

func TestProductionAdminShellUsesBlueprintNavigationVisualLanguage(t *testing.T) {
	html := readToolSiteUISource(t)
	for _, want := range []string{
		"blueprint-page",
		"BlueprintHeader",
		"BlueprintGuideLine",
		"blueprint-top-actions",
		"ThemeToggle",
		"blueprint-stamp",
		"admin-tab-card",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("production admin shell should reuse blueprint navigation marker %q", want)
		}
	}
	for _, notWant := range []string{
		"class=\"shell\"",
		"class=\"topbar\"",
		"class=\"nav-actions\"",
		"Wabi",
		"wabi-",
	} {
		if strings.Contains(html, notWant) {
			t.Fatalf("production admin shell should not use old admin/wabi shell marker %q", notWant)
		}
	}
}

func TestProductionAdminShellUsesRealHeroUIWithBlueprintTokens(t *testing.T) {
	html := readToolSiteUISource(t)
	for _, want := range []string{
		"@heroui/react",
		"useTheme",
		"@heroui/styles",
		"--blueprint-canvas",
		"--blueprint-panel",
		"--blueprint-accent",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("production admin shell should contain real HeroUI/blueprint token marker %q", want)
		}
	}
	if strings.Contains(strings.ToLower(html), "--sage-") || strings.Contains(strings.ToLower(html), "sage-design") {
		t.Fatalf("production admin shell should not use Sage Design tokens")
	}
	if strings.Contains(strings.ToLower(html), "--fufu-") || strings.Contains(strings.ToLower(html), "wabi") {
		t.Fatalf("production admin shell should not keep old fufu wabi design tokens")
	}
}

func TestProductionAdminShellUsesHeroUIComponentsForControls(t *testing.T) {
	html := readToolSiteUISource(t)
	for _, want := range []string{
		"<Button",
		"<Card",
		"<Tabs",
		"<Input",
		"<Table",
		"<Chip",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("production admin shell should use HeroUI component marker %q", want)
		}
	}
	if regexp.MustCompile(`<[A-Za-z][^>]*\sdata-slot=`).MatchString(html) {
		t.Fatalf("production admin shell should use real HeroUI components, not static data-slot shims")
	}
}

func TestUnifiedAdminRoutesExposeActivityBusinessEndpoints(t *testing.T) {
	root := t.TempDir()
	writeToolSiteFixture(t, root)
	t.Setenv("ADMIN_TOKEN", "secret-admin-token")
	if err := initRuntime(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(shutdownRuntime)

	for _, tc := range []struct {
		path string
		want string
	}{
		{path: "/api/admin/stats", want: "totalSpins"},
		{path: "/api/admin/sale-cards/config", want: "plans"},
		{path: "/api/prizes", want: "spinMap"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			req := authorizedJSONRequest(t, http.MethodGet, tc.path, nil)
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

func TestAdminConfigSavesNewAPISitesForStatusAndCombine(t *testing.T) {
	root := t.TempDir()
	writeToolSiteFixture(t, root)
	t.Setenv("ADMIN_TOKEN", "secret-admin-token")
	if err := initRuntime(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(shutdownRuntime)

	saveAdminConfig(t, map[string]any{
		"newapi": map[string]any{
			"sites": []map[string]any{
				{
					"name":          "主次数站",
					"url":           "https://api-primary.example.test/",
					"token":         "primary-token",
					"userId":        "9",
					"quotaUnit":     600000,
					"rechargeRatio": 0.25,
				},
				{
					"name":      "备用次数站",
					"url":       "https://api-backup.example.test",
					"token":     "backup-token",
					"userId":    "1",
					"quotaUnit": 500000,
				},
			},
		},
	})

	configPath := filepath.Join(root, "data", toolConfigDBName)
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("expected config database at %s: %v", configPath, err)
	}

	statusReq := httptest.NewRequest(http.MethodGet, "/api/newapi/sites", nil)
	statusW := httptest.NewRecorder()
	route(statusW, statusReq)
	if statusW.Code != http.StatusOK {
		t.Fatalf("sites code=%d body=%s", statusW.Code, statusW.Body.String())
	}
	if body := statusW.Body.String(); !strings.Contains(body, "主次数站") || !strings.Contains(body, "备用次数站") {
		t.Fatalf("status page sites should come from admin config, got %s", body)
	}

	site, err := primarySiteForCombine()
	if err != nil {
		t.Fatalf("primarySiteForCombine: %v", err)
	}
	if site.Name != "主次数站" || site.URL != "https://api-primary.example.test" || site.Token != "primary-token" {
		t.Fatalf("combine should reuse first status-page site, got %#v", site)
	}

	connectivityReq := httptest.NewRequest(http.MethodGet, "/api/connectivity/targets", nil)
	connectivityW := httptest.NewRecorder()
	route(connectivityW, connectivityReq)
	if connectivityW.Code != http.StatusOK {
		t.Fatalf("connectivity targets code=%d body=%s", connectivityW.Code, connectivityW.Body.String())
	}
	if body := connectivityW.Body.String(); !strings.Contains(body, "https://api-primary.example.test") || !strings.Contains(body, "https://api-backup.example.test") {
		t.Fatalf("connectivity targets should reuse status-page base URLs, got %s", body)
	}

	navReq := httptest.NewRequest(http.MethodGet, "/api/nav/lines", nil)
	navW := httptest.NewRecorder()
	route(navW, navReq)
	if navW.Code != http.StatusOK {
		t.Fatalf("nav lines code=%d body=%s", navW.Code, navW.Body.String())
	}
	if body := navW.Body.String(); !strings.Contains(body, "https://api-primary.example.test") || !strings.Contains(body, "https://api-backup.example.test") {
		t.Fatalf("homepage nav lines should reuse configured base URLs, got %s", body)
	}

	runReq := authorizedJSONRequest(t, http.MethodPost, "/api/admin/sale-cards/run", map[string]any{
		"plan":        "__unknown_plan__",
		"targetStock": 1,
	})
	runW := httptest.NewRecorder()
	route(runW, runReq)
	if runW.Code != http.StatusServiceUnavailable || !strings.Contains(runW.Body.String(), "暂时下线") {
		t.Fatalf("sale-card run should be paused before touching integrations, code=%d body=%s", runW.Code, runW.Body.String())
	}
}

func TestNavLinesExposeHomepageDefaultsWithoutManagedSites(t *testing.T) {
	root := t.TempDir()
	writeToolSiteFixture(t, root)
	t.Setenv("ADMIN_TOKEN", "secret-admin-token")
	t.Setenv("NEWAPI_MANAGED_API_CONFIG", filepath.Join(root, "missing-managed-sites.json"))
	if err := initRuntime(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(shutdownRuntime)

	sitesReq := httptest.NewRequest(http.MethodGet, "/api/newapi/sites", nil)
	sitesW := httptest.NewRecorder()
	route(sitesW, sitesReq)
	if sitesW.Code != http.StatusOK {
		t.Fatalf("newapi sites code=%d body=%s", sitesW.Code, sitesW.Body.String())
	}
	if body := sitesW.Body.String(); !strings.Contains(body, `"configured":false`) || strings.Contains(body, `api.fufuapi.top`) {
		t.Fatalf("runtime NewAPI sites must stay empty without token config, got %s", body)
	}

	navReq := httptest.NewRequest(http.MethodGet, "/api/nav/lines", nil)
	navW := httptest.NewRecorder()
	route(navW, navReq)
	if navW.Code != http.StatusOK {
		t.Fatalf("nav lines code=%d body=%s", navW.Code, navW.Body.String())
	}
	body := navW.Body.String()
	for _, want := range []string{
		"https://api.fufuapi.top",
		"https://api.fufuapi.online",
		"https://api.fufuflower.top",
		"https://token.fufuapi.top",
		"https://token.fufuapi.online",
		"https://token.fufuflower.top",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("nav fallback should include %q, got %s", want, body)
		}
	}
}

func TestConnectivityTargetsSplitManagedSitesByAPIAndTokenCategory(t *testing.T) {
	root := t.TempDir()
	writeToolSiteFixture(t, root)
	t.Setenv("ADMIN_TOKEN", "secret-admin-token")
	if err := initRuntime(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(shutdownRuntime)

	saveAdminConfig(t, map[string]any{
		"newapi": map[string]any{
			"sites": []map[string]any{
				{"name": "次数站", "category": "api", "token": "api-token", "urls": []map[string]any{
					{"name": "国内加速", "url": "https://api-a.example.test"},
					{"name": "海外线路", "url": "https://api-b.example.test"},
				}},
				{"name": "Token 站", "category": "token", "token": "token-site-token", "urls": []map[string]any{
					{"name": "主线路", "url": "https://token-a.example.test"},
					{"name": "备用线路", "url": "https://token-b.example.test"},
				}},
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/connectivity/targets", nil)
	w := httptest.NewRecorder()
	route(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("connectivity targets code=%d body=%s", w.Code, w.Body.String())
	}

	var payload struct {
		Groups []struct {
			ID   string   `json:"id"`
			Name string   `json:"name"`
			URLs []string `json:"urls"`
		} `json:"groups"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode connectivity targets: %v body=%s", err, w.Body.String())
	}
	if len(payload.Groups) != 2 {
		t.Fatalf("expected api/token groups, got %#v body=%s", payload.Groups, w.Body.String())
	}
	if payload.Groups[0].ID != "api" || payload.Groups[0].Name != "API 次数站" || len(payload.Groups[0].URLs) != 2 {
		t.Fatalf("api group = %#v", payload.Groups[0])
	}
	if payload.Groups[1].ID != "token" || payload.Groups[1].Name != "Token 站" || len(payload.Groups[1].URLs) != 2 {
		t.Fatalf("token group = %#v", payload.Groups[1])
	}
	for _, notWant := range []string{"NewAPI 站点", `"id":"newapi"`} {
		if strings.Contains(w.Body.String(), notWant) {
			t.Fatalf("connectivity targets should not merge categories into %q: %s", notWant, w.Body.String())
		}
	}
}

func TestAdminConfigSavesNavigationToolsForHomepage(t *testing.T) {
	root := t.TempDir()
	writeToolSiteFixture(t, root)
	t.Setenv("ADMIN_TOKEN", "secret-admin-token")
	if err := initRuntime(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(shutdownRuntime)

	saveAdminConfig(t, map[string]any{
		"navigation": map[string]any{
			"cards": []map[string]any{
				{
					"id":          "terminal",
					"stamp":       "终端",
					"title":       "Web Terminal",
					"description": "服务器网页管理终端",
					"accent":      "moss",
					"links": []map[string]any{
						{"label": "主线路", "href": "https://terminal.example.test", "ping": "https://terminal.example.test/health"},
					},
				},
				{
					"id":          "build",
					"stamp":       "造物",
					"title":       "Build",
					"description": "AI 画图生成",
					"accent":      "stone",
					"href":        "https://build.example.test",
				},
			},
		},
	})

	toolsReq := httptest.NewRequest(http.MethodGet, "/api/nav/tools", nil)
	toolsW := httptest.NewRecorder()
	route(toolsW, toolsReq)
	if toolsW.Code != http.StatusOK {
		t.Fatalf("nav tools code=%d body=%s", toolsW.Code, toolsW.Body.String())
	}
	body := toolsW.Body.String()
	for _, want := range []string{"terminal", "https://terminal.example.test", "https://build.example.test"} {
		if !strings.Contains(body, want) {
			t.Fatalf("nav tools should include configured %q, got %s", want, body)
		}
	}
	if strings.Contains(body, "https://terminal.fufuapi.top") || strings.Contains(body, "https://build.fufuapi.online") {
		t.Fatalf("nav tools should use saved config instead of hardcoded defaults, got %s", body)
	}

	configReq := authorizedJSONRequest(t, http.MethodGet, "/api/admin/config", nil)
	configW := httptest.NewRecorder()
	route(configW, configReq)
	if configW.Code != http.StatusOK {
		t.Fatalf("admin config code=%d body=%s", configW.Code, configW.Body.String())
	}
	if body := configW.Body.String(); !strings.Contains(body, `"navigation"`) || !strings.Contains(body, "https://terminal.example.test") {
		t.Fatalf("admin config should expose navigation config for editing, got %s", body)
	}
}

func TestNavToolsResolveConfiguredLineCardsFromManagedSites(t *testing.T) {
	root := t.TempDir()
	writeToolSiteFixture(t, root)
	t.Setenv("ADMIN_TOKEN", "secret-admin-token")
	if err := initRuntime(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(shutdownRuntime)

	saveAdminConfig(t, map[string]any{
		"newapi": map[string]any{
			"sites": []map[string]any{
				{
					"name":      "主次数站",
					"category":  "api",
					"url":       "https://api-primary.example.test/",
					"token":     "primary-token",
					"userId":    "1",
					"quotaUnit": 500000,
				},
				{
					"name":      "主 Token 站",
					"category":  "token",
					"url":       "https://token-primary.example.test/",
					"token":     "token-site-token",
					"userId":    "1",
					"quotaUnit": 500000,
				},
			},
		},
		"navigation": map[string]any{
			"cards": []map[string]any{
				{"id": "api", "stamp": "次数", "title": "API 次数站", "accent": "clay", "lineKind": "api"},
				{"id": "token", "stamp": "额度", "title": "Token 站", "accent": "moss", "lineKind": "token"},
				{"id": "status", "stamp": "状态", "title": "状态页", "accent": "moss", "href": "/status"},
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/nav/tools", nil)
	w := httptest.NewRecorder()
	route(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("nav tools code=%d body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, want := range []string{
		`"id":"api"`,
		`"lineKind":"api"`,
		"https://api-primary.example.test",
		`"id":"token"`,
		`"lineKind":"token"`,
		"https://token-primary.example.test",
		"/status",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("nav tools should include configured line card %q, got %s", want, body)
		}
	}
	for _, notWant := range []string{"https://api.fufuapi.top", "https://token.fufuapi.top"} {
		if strings.Contains(body, notWant) {
			t.Fatalf("nav tools should resolve from managed sites instead of fallback %q, got %s", notWant, body)
		}
	}
}

func TestNavToolsSortsRuntimeMultiLineCardsBeforeSingleLinkCards(t *testing.T) {
	root := t.TempDir()
	writeToolSiteFixture(t, root)
	t.Setenv("ADMIN_TOKEN", "secret-admin-token")
	if err := initRuntime(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(shutdownRuntime)

	saveAdminConfig(t, map[string]any{
		"navigation": map[string]any{
			"cards": []map[string]any{
				{"id": "status", "stamp": "状态", "title": "状态页", "accent": "moss", "href": "/status"},
				{"id": "api", "stamp": "次数", "title": "API 次数站", "accent": "clay", "lineKind": "api"},
				{"id": "build", "stamp": "造物", "title": "Build", "accent": "stone", "href": "https://build.example.test"},
				{"id": "token", "stamp": "额度", "title": "Token 站", "accent": "moss", "lineKind": "token"},
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/nav/tools", nil)
	w := httptest.NewRecorder()
	route(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("nav tools code=%d body=%s", w.Code, w.Body.String())
	}

	var payload navToolsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode nav tools: %v body=%s", err, w.Body.String())
	}
	gotIDs := []string{}
	for _, card := range payload.Cards {
		gotIDs = append(gotIDs, card.ID)
	}
	wantIDs := []string{"api", "token", "status", "build"}
	for i, want := range wantIDs {
		if i >= len(gotIDs) || gotIDs[i] != want {
			t.Fatalf("nav tools order = %#v, want prefix %#v", gotIDs, wantIDs)
		}
	}
}

func TestNavToolsExposeHomepageDefaults(t *testing.T) {
	root := t.TempDir()
	writeToolSiteFixture(t, root)
	t.Setenv("ADMIN_TOKEN", "secret-admin-token")
	t.Setenv("NEWAPI_MANAGED_API_CONFIG", filepath.Join(root, "missing-managed-sites.json"))
	if err := initRuntime(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(shutdownRuntime)

	req := httptest.NewRequest(http.MethodGet, "/api/nav/tools", nil)
	w := httptest.NewRecorder()
	route(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("nav tools code=%d body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, want := range []string{
		"API 次数站",
		`"lineKind":"api"`,
		"https://api.fufuapi.top",
		"Token 站",
		`"lineKind":"token"`,
		"https://token.fufuapi.top",
		"Web Terminal",
		"https://terminal.fufuapi.top",
		"Build",
		"https://build.fufuapi.online",
		"/status",
		"/combine",
		"/activity",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("nav tools default should include %q, got %s", want, body)
		}
	}
}

func TestAdminConfigSiteHoldsOneTokenAndManyURLs(t *testing.T) {
	root := t.TempDir()
	writeToolSiteFixture(t, root)
	t.Setenv("ADMIN_TOKEN", "secret-admin-token")
	if err := initRuntime(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(shutdownRuntime)

	// A token 站 configured ONCE with one token + two base_urls. This previously
	// failed with "第 2 个 NewAPI 站点缺少 token" because every url demanded its own
	// token; now a site carries one token shared across its urls.
	saveAdminConfig(t, map[string]any{
		"newapi": map[string]any{
			"sites": []map[string]any{
				{"name": "token 站", "category": "token", "token": "tok-shared", "urls": []map[string]any{
					{"name": "线路 一", "url": "https://t1.example.test"},
					{"name": "线路 二", "url": "https://t2.example.test"},
				}},
			},
		},
	})

	snap := unifiedConfig.Snapshot()
	if len(snap.NewAPI.Sites) != 1 {
		t.Fatalf("token 站 should persist as ONE site, got %#v", snap.NewAPI.Sites)
	}
	site := snap.NewAPI.Sites[0]
	if site.Token != "tok-shared" || len(site.URLs) != 2 {
		t.Fatalf("site should hold one token + two urls, got %#v", site)
	}

	// Runtime expansion: one grouped site -> two flat newapi.Site, both sharing
	// the single token, both category token, each carrying its line name.
	runtime := unifiedConfig.ManagedSites()
	if len(runtime) != 2 {
		t.Fatalf("expected 2 expanded runtime sites, got %#v", runtime)
	}
	for _, s := range runtime {
		if s.Token != "tok-shared" || s.Category != "token" {
			t.Fatalf("expanded site %q lost token/category: %#v", s.LineName, s)
		}
	}
	if runtime[0].URL != "https://t1.example.test" || runtime[0].LineName != "线路 一" {
		t.Fatalf("first expanded url/line wrong: %#v", runtime[0])
	}

	// Re-save with a BLANK token (UI never re-sends the masked token) and a third
	// url. The site must keep its one token — no "缺少 token" error.
	saveAdminConfig(t, map[string]any{
		"newapi": map[string]any{
			"sites": []map[string]any{
				{"name": "token 站", "category": "token", "urls": []map[string]any{
					{"name": "线路 一", "url": "https://t1.example.test"},
					{"name": "线路 二", "url": "https://t2.example.test"},
					{"name": "线路 三", "url": "https://t3.example.test"},
				}},
			},
		},
	})
	resaved := unifiedConfig.Snapshot().NewAPI.Sites
	if len(resaved) != 1 || resaved[0].Token != "tok-shared" || len(resaved[0].URLs) != 3 {
		t.Fatalf("blank-token re-save must keep the one token + grow urls, got %#v", resaved)
	}
}

func TestAdminConfigBlankTokenInheritsOnlyBySameSiteName(t *testing.T) {
	root := t.TempDir()
	writeToolSiteFixture(t, root)
	t.Setenv("ADMIN_TOKEN", "secret-admin-token")
	if err := initRuntime(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(shutdownRuntime)

	saveAdminConfig(t, map[string]any{
		"newapi": map[string]any{
			"sites": []map[string]any{
				{"name": "次数fufu", "category": "api", "token": "secret-token", "urls": []map[string]any{{"url": "https://api-1.example.test"}}},
			},
		},
	})

	// A DIFFERENTLY-named site in the same category with a blank token must NOT
	// silently inherit the existing site's token — it is rejected.
	req := authorizedJSONRequest(t, http.MethodPut, "/api/admin/config", map[string]any{
		"newapi": map[string]any{
			"sites": []map[string]any{
				{"name": "rogue", "category": "api", "urls": []map[string]any{{"url": "https://rogue.example.test"}}},
			},
		},
	})
	w := httptest.NewRecorder()
	route(w, req)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "缺少 token") {
		t.Fatalf("blank-token rogue site should be rejected, got code=%d body=%s", w.Code, w.Body.String())
	}

	// The same-named site re-saving with a blank token DOES inherit its token.
	saveAdminConfig(t, map[string]any{
		"newapi": map[string]any{
			"sites": []map[string]any{
				{"name": "次数fufu", "category": "api", "urls": []map[string]any{
					{"url": "https://api-1.example.test"},
					{"url": "https://api-2.example.test"},
				}},
			},
		},
	})
	sites := unifiedConfig.Snapshot().NewAPI.Sites
	if len(sites) != 1 || sites[0].Token != "secret-token" || len(sites[0].URLs) != 2 {
		t.Fatalf("same-name blank re-save should inherit token + grow urls, got %#v", sites)
	}
}

func TestAdminConfigMergesLegacyPerLineSitesIntoOneSitePerToken(t *testing.T) {
	root := t.TempDir()
	writeToolSiteFixture(t, root)
	t.Setenv("ADMIN_TOKEN", "secret-admin-token")
	if err := initRuntime(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(shutdownRuntime)

	// Legacy per-line shape (singular "url"), two api lines sharing one token.
	// These must collapse into ONE grouped site with two urls.
	saveAdminConfig(t, map[string]any{
		"newapi": map[string]any{
			"sites": []map[string]any{
				{"name": "次数fufu", "category": "api", "url": "https://api-1.example.test", "token": "api-token"},
				{"name": "线路 2", "category": "api", "url": "https://api-2.example.test", "token": "api-token"},
			},
		},
	})

	sites := unifiedConfig.Snapshot().NewAPI.Sites
	if len(sites) != 1 {
		t.Fatalf("same-token api lines should merge into one site, got %#v", sites)
	}
	if sites[0].Token != "api-token" || len(sites[0].URLs) != 2 || sites[0].URL != "https://api-1.example.test" {
		t.Fatalf("merged site should keep token + both urls + mirror primary url: %#v", sites[0])
	}
}

func TestNormalizeMCYBaseURL(t *testing.T) {
	cases := []struct{ in, want string }{
		// The admin URL users paste carries http + an /admin path; both break the
		// encrypted POST (http 301 drops the body; /admin would double to
		// /admin/admin/login). Reduce to https://host.
		{"http://shop.fufuapi.top/admin", "https://shop.fufuapi.top"},
		{"https://shop.fufuapi.top/admin/", "https://shop.fufuapi.top"},
		{"https://shop.fufuapi.top", "https://shop.fufuapi.top"},
		{"shop.fufuapi.top", "https://shop.fufuapi.top"},
		{"http://shop.fufuapi.top", "https://shop.fufuapi.top"},
		{"  https://shop.fufuapi.top/x/y?z=1  ", "https://shop.fufuapi.top"},
		{"http://127.0.0.1:8099/admin", "https://127.0.0.1:8099"},
		{"", ""},
	}
	for _, c := range cases {
		if got := normalizeMCYBaseURL(c.in); got != c.want {
			t.Fatalf("normalizeMCYBaseURL(%q)=%q, want %q", c.in, got, c.want)
		}
	}
}

func TestNormalizeMCYConfigCleansBaseURL(t *testing.T) {
	got := normalizeMCYConfig(MCYAdminConfig{BaseURL: "http://shop.fufuapi.top/admin"}, MCYAdminConfig{})
	if got.BaseURL != "https://shop.fufuapi.top" {
		t.Fatalf("normalizeMCYConfig base=%q, want https://shop.fufuapi.top", got.BaseURL)
	}
}

func TestAdminConfigSavesMCYCredentialsMaskedAndInherited(t *testing.T) {
	root := t.TempDir()
	writeToolSiteFixture(t, root)
	t.Setenv("ADMIN_TOKEN", "secret-admin-token")
	if err := initRuntime(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(shutdownRuntime)

	saveAdminConfig(t, map[string]any{
		"mcy": map[string]any{
			"baseUrl":  "https://shop.example.test/",
			"username": "shopuser@example.com",
			"password": "super-secret-shop-pw",
		},
	})

	snap := unifiedConfig.Snapshot()
	if snap.MCY.BaseURL != "https://shop.example.test" || snap.MCY.Username != "shopuser@example.com" || snap.MCY.Password != "super-secret-shop-pw" {
		t.Fatalf("MCY config not stored (trailing slash trimmed, password kept): %#v", snap.MCY)
	}

	// The admin response masks the password and never leaks it raw.
	raw, err := json.Marshal(adminConfigResponse(snap))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "super-secret-shop-pw") {
		t.Fatalf("raw MCY password must never be in the response: %s", raw)
	}
	if !strings.Contains(string(raw), "shopuser@example.com") || !strings.Contains(string(raw), "passwordSet") {
		t.Fatalf("MCY response should carry username + passwordSet: %s", raw)
	}

	// Re-save with a blank password (UI holds only the mask) → keep the stored one.
	saveAdminConfig(t, map[string]any{
		"mcy": map[string]any{"baseUrl": "https://shop.example.test", "username": "shopuser@example.com"},
	})
	if got := unifiedConfig.Snapshot().MCY.Password; got != "super-secret-shop-pw" {
		t.Fatalf("blank-password re-save must keep the stored password, got %q", got)
	}
}

func TestAdminConfigSavesActivityOddsAndDates(t *testing.T) {
	root := t.TempDir()
	writeToolSiteFixture(t, root)
	t.Setenv("ADMIN_TOKEN", "secret-admin-token")
	if err := initRuntime(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(shutdownRuntime)

	saveAdminConfig(t, map[string]any{
		"activity": map[string]any{
			"startText":           "2026-06-01 00:00:00",
			"endText":             "2026-06-30 23:59:59",
			"startTS":             1780243200,
			"endTS":               1782835199,
			"targetExpectedValue": 4.5,
			"gameConfigs": []map[string]any{
				{"game": "slot", "targetExpectedValue": 4.5, "actualExpectedValue": 4.5},
				{"game": "scratch", "targetExpectedValue": 2.5, "actualExpectedValue": 2.5},
			},
			"spinMap": map[string]int{
				"42": 3,
			},
			"prizePool": []map[string]any{
				{"type": "miss", "weight": 100},
				{"type": "win", "dollars": 9, "weight": 1},
			},
			"tierPools": map[string][]map[string]any{
				"42": {
					{"type": "miss", "weight": 1},
					{"type": "win", "dollars": 7, "weight": 3},
				},
			},
			"scratchRewards": []int{2, 4, 6, 8, 10, 12},
		},
	})

	cfg := activityapp.SnapshotRuntimeConfig()
	if cfg.StartText != "2026-06-01 00:00:00" || cfg.EndText != "2026-06-30 23:59:59" || cfg.StartTS != 1780243200 || cfg.EndTS != 1782835199 {
		t.Fatalf("activity dates not applied: %#v", cfg)
	}
	if cfg.DrawCountForTier(42) != 3 || len(cfg.PrizePool) != 2 || cfg.PrizePool[0].Weight != 100 || cfg.PrizePool[1].Dollars != 9 || cfg.PrizePool[1].Weight != 1 {
		t.Fatalf("activity odds not applied: %#v", cfg)
	}

	req := authorizedJSONRequest(t, http.MethodGet, "/api/admin/config", nil)
	w := httptest.NewRecorder()
	route(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("get config code=%d body=%s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	activity, _ := body["activity"].(map[string]any)
	if activity["targetExpectedValue"] != float64(4.5) || activity["actualExpectedValue"] != float64(4.5) {
		t.Fatalf("activity expected values = %#v", activity)
	}
	gameConfigs, _ := activity["gameConfigs"].([]any)
	if len(gameConfigs) < 2 {
		t.Fatalf("activity should expose per-game configs, got %#v", activity)
	}

	prizesReq := httptest.NewRequest(http.MethodGet, "/api/prizes", nil)
	prizesW := httptest.NewRecorder()
	route(prizesW, prizesReq)
	if prizesW.Code != http.StatusOK {
		t.Fatalf("prizes code=%d body=%s", prizesW.Code, prizesW.Body.String())
	}
	if body := prizesW.Body.String(); !strings.Contains(body, `"gameConfigs"`) || !strings.Contains(body, `"dollars":9`) || !strings.Contains(body, `"weight":1`) || !strings.Contains(body, `"totalWeight":2`) || strings.Contains(body, `"tierPools"`) || strings.Contains(body, `"postJackpotPrizes"`) || strings.Contains(body, `"dollars":7`) {
		t.Fatalf("activity prizes should reflect unified admin config, got %s", body)
	}
}

func TestAdminConfigMigratesLegacyFileAndBecomesSourceOfTruth(t *testing.T) {
	root := t.TempDir()
	writeToolSiteFixture(t, root)
	t.Setenv("ADMIN_TOKEN", "secret-admin-token")

	// Simulate a pre-database deployment that already saved a tool-config.json.
	dataDir := filepath.Join(root, "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(dataDir, "tool-config.json")
	legacyJSON := `{"newapi":{"sites":[{"name":"迁移站点","url":"https://migrated.example.test","token":"legacy-token","userId":"1","kind":"api","quotaUnit":500000,"currency":"$","rechargeRatio":1}]},"activity":{"startText":"2026-07-01 00:00:00","spinMap":{"7":7}}}`
	if err := os.WriteFile(legacy, []byte(legacyJSON), 0600); err != nil {
		t.Fatal(err)
	}

	if err := initRuntime(root); err != nil {
		t.Fatal(err)
	}

	snap := unifiedConfig.Snapshot()
	if len(snap.NewAPI.Sites) != 1 || snap.NewAPI.Sites[0].Name != "迁移站点" || snap.NewAPI.Sites[0].Token != "legacy-token" {
		t.Fatalf("legacy sites not migrated into database: %#v", snap.NewAPI.Sites)
	}
	if snap.Activity.StartText != "2026-07-01 00:00:00" || snap.Activity.SpinMap[7] != 7 {
		t.Fatalf("legacy activity not migrated: %#v", snap.Activity)
	}
	if _, err := os.Stat(filepath.Join(dataDir, toolConfigDBName)); err != nil {
		t.Fatalf("expected config database after migration: %v", err)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatalf("legacy tool-config.json should be renamed after migration, stat err=%v", err)
	}
	if _, err := os.Stat(legacy + ".migrated"); err != nil {
		t.Fatalf("expected migrated legacy backup: %v", err)
	}

	// Restart with different env vars. The database must remain the source of
	// truth, so env changes after the first boot are ignored.
	shutdownRuntime()
	t.Setenv("NEWAPI_API_SITE_URL", "https://env-should-be-ignored.example.test")
	t.Setenv("NEWAPI_API_SITE_TOKEN", "env-token")
	if err := initRuntime(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(shutdownRuntime)

	snap = unifiedConfig.Snapshot()
	if len(snap.NewAPI.Sites) != 1 || snap.NewAPI.Sites[0].Name != "迁移站点" || snap.NewAPI.Sites[0].URL != "https://migrated.example.test" {
		t.Fatalf("database should stay source of truth over env, got %#v", snap.NewAPI.Sites)
	}
}

func validBaseSiteConfig() ManagedAPISiteConfig {
	return ManagedAPISiteConfig{
		Name:          "次数fufu",
		Category:      "api",
		URLs:          []ManagedSiteURL{{Name: "线路 1", URL: "https://api.example.test"}},
		Token:         "tok",
		UserID:        "1",
		Kind:          "api",
		QuotaUnit:     500000,
		Currency:      "$",
		RechargeRatio: 1,
	}
}

func TestNormalizeManagedAPISiteConfigsRejectsInvalidSites(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(s *ManagedAPISiteConfig)
		wantMsg string
	}{
		{"unsupported category", func(s *ManagedAPISiteConfig) { s.Category = "webhook" }, "只能 api 或 token"},
		{"unsupported kind", func(s *ManagedAPISiteConfig) { s.Kind = "webhook" }, "kind 不支持"},
		{"blank name", func(s *ManagedAPISiteConfig) { s.Name = "  " }, "缺少名称"},
		{"zero urls", func(s *ManagedAPISiteConfig) { s.URLs = nil; s.URL = "" }, "至少需要一个 base_url"},
		{"blank token new site", func(s *ManagedAPISiteConfig) { s.Token = "" }, "缺少 token"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			site := validBaseSiteConfig()
			c.mutate(&site)
			_, err := normalizeManagedAPISiteConfigs([]ManagedAPISiteConfig{site}, nil)
			if err == nil || !strings.Contains(err.Error(), c.wantMsg) {
				t.Fatalf("err=%v, want substring %q", err, c.wantMsg)
			}
		})
	}
}

func TestNormalizeManagedAPISiteConfigsRejectsDuplicateNameDifferentToken(t *testing.T) {
	a := validBaseSiteConfig()
	a.Token = "tok-a"
	b := validBaseSiteConfig() // same (category, name) but a different token + url
	b.Token = "tok-b"
	b.URLs = []ManagedSiteURL{{Name: "线路 2", URL: "https://api-2.example.test"}}

	_, err := normalizeManagedAPISiteConfigs([]ManagedAPISiteConfig{a, b}, nil)
	if err == nil || !strings.Contains(err.Error(), "重复") {
		t.Fatalf("same name + different token should be a duplicate, err=%v", err)
	}
}

func TestNormalizeManagedAPISiteConfigsMergesSameToken(t *testing.T) {
	a := validBaseSiteConfig()
	a.Name = "次数fufu"
	a.URLs = []ManagedSiteURL{{Name: "线路 1", URL: "https://api-1.example.test"}}
	b := validBaseSiteConfig()
	b.Name = "线路 2"
	b.URLs = []ManagedSiteURL{{Name: "线路 2", URL: "https://api-2.example.test"}}

	out, err := normalizeManagedAPISiteConfigs([]ManagedAPISiteConfig{a, b}, nil)
	if err != nil {
		t.Fatalf("merge error: %v", err)
	}
	if len(out) != 1 || len(out[0].URLs) != 2 || out[0].URL != "https://api-1.example.test" {
		t.Fatalf("same-token sites should merge into one with both urls: %#v", out)
	}
}

func TestManagedSiteConfigsFromSitesGroupsByCategoryAndToken(t *testing.T) {
	got := managedSiteConfigsFromSites([]newapi.Site{
		{Name: "主站", Category: "api", Token: "tok1", URL: "https://a.example.test", LineName: "线路 一"},
		{Name: "备站", Category: "api", Token: "tok1", URL: "https://b.example.test", LineName: "线路 二"},
		{Name: "token-fufu", Category: "token", Token: "tok2", URL: "https://t.example.test"},
	})
	if len(got) != 2 {
		t.Fatalf("expected 2 grouped sites (api+token), got %#v", got)
	}
	api := got[0]
	if api.Category != "api" || api.Token != "tok1" || len(api.URLs) != 2 {
		t.Fatalf("api group wrong: %#v", api)
	}
	if api.URLs[0].URL != "https://a.example.test" || api.URLs[1].Name != "线路 二" {
		t.Fatalf("api urls wrong: %#v", api.URLs)
	}
}

func TestAdminSiteResponsesNeverExposesRawToken(t *testing.T) {
	secret := "super-secret-token-value-1234"
	resp := adminSiteResponses([]ManagedAPISiteConfig{{
		Name: "s", Category: "api", Token: secret,
		URLs: []ManagedSiteURL{{Name: "线路 1", URL: "https://a.example.test"}},
	}})
	raw, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), secret) {
		t.Fatalf("response must never contain the raw token: %s", raw)
	}
	if resp[0]["tokenSet"] != true || resp[0]["tokenMasked"] == "" {
		t.Fatalf("response should expose tokenSet + tokenMasked, got %#v", resp[0])
	}
	if _, hasToken := resp[0]["token"]; hasToken {
		t.Fatalf("response must not carry a raw token field")
	}
}

func TestMergeManagedSiteURLsDedupesPreservingOrder(t *testing.T) {
	got := mergeManagedSiteURLs(
		[]ManagedSiteURL{{URL: "https://a"}, {URL: "https://b"}},
		[]ManagedSiteURL{{URL: "https://b"}, {URL: "https://c"}},
	)
	if len(got) != 3 || got[0].URL != "https://a" || got[1].URL != "https://b" || got[2].URL != "https://c" {
		t.Fatalf("merge should dedupe b and keep order a,b,c: %#v", got)
	}
}

func TestNormalizeManagedSiteURLsFoldsLegacyAndDedupes(t *testing.T) {
	got := normalizeManagedSiteURLs(
		[]ManagedSiteURL{{Name: "线路 一", URL: "https://a"}, {URL: "https://b"}},
		"https://b", // legacy singular url duplicates an entry
	)
	if len(got) != 2 || got[0].Name != "线路 一" || got[1].Name != "线路 2" {
		t.Fatalf("legacy url should fold + dedupe, blanks get default names: %#v", got)
	}
}

func saveAdminConfig(t *testing.T, payload map[string]any) {
	t.Helper()
	req := authorizedJSONRequest(t, http.MethodPut, "/api/admin/config", payload)
	w := httptest.NewRecorder()
	route(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("save config code=%d body=%s", w.Code, w.Body.String())
	}
}

func readToolSiteUISource(t *testing.T) string {
	t.Helper()
	var builder strings.Builder
	err := filepath.WalkDir(filepath.Join("ui", "src"), func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		switch filepath.Ext(path) {
		case ".ts", ".tsx", ".css":
		default:
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		builder.WriteString("\n/* ")
		builder.WriteString(filepath.ToSlash(path))
		builder.WriteString(" */\n")
		builder.Write(body)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return builder.String()
}

func jsonRequest(t *testing.T, method, path string, payload any) *http.Request {
	t.Helper()
	var body *bytes.Reader
	if payload == nil {
		body = bytes.NewReader(nil)
	} else {
		data, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		body = bytes.NewReader(data)
	}
	req := httptest.NewRequest(method, path, body)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

func adminSessionCookieFromRecorder(t *testing.T, w *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == adminSessionCookieName {
			return cookie
		}
	}
	t.Fatalf("missing %s cookie; cookies=%#v", adminSessionCookieName, w.Result().Cookies())
	return nil
}

func authorizedJSONRequest(t *testing.T, method, path string, payload any) *http.Request {
	t.Helper()
	var body *bytes.Reader
	if payload == nil {
		body = bytes.NewReader(nil)
	} else {
		data, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		body = bytes.NewReader(data)
	}
	req := httptest.NewRequest(method, path, body)
	req.Header.Set("Authorization", "Bearer secret-admin-token")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}
