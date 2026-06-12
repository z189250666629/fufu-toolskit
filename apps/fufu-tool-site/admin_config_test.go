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

	badReq := jsonRequest(t, http.MethodPost, "/api/admin/session", map[string]any{"token": "wrong-token"})
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

func TestProductionAdminShellReferencesActualBusinessAPIs(t *testing.T) {
	html := readToolSiteUISource(t)
	for _, want := range []string{
		"活动统计",
		"自动补卡",
		"当前奖池中奖率",
		"/api/admin/stats",
		"/api/admin/sale-cards/config",
		"/api/admin/sale-cards/run",
		"/api/prizes",
		"/api/newapi/sites",
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
		`id="admin-token"`,
		"输入 ADMIN_TOKEN",
		"Authorization: 'Bearer '",
	} {
		if strings.Contains(html, notWant) {
			t.Fatalf("production admin shell should not expose inline token auth marker %q", notWant)
		}
	}
}

func TestProductionAdminShellGroupsOverlappingConfigIntoTwoTabs(t *testing.T) {
	html := readToolSiteUISource(t)
	for _, want := range []string{
		`site-replenish`,
		"状态页 / 合卡 / 自动补货",
		"状态页实际站点",
		"NewAPI 站点配置",
		"自动补卡",
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
		"<Textarea",
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
}

func TestAdminConfigSharesOneTokenAcrossSiteURLs(t *testing.T) {
	root := t.TempDir()
	writeToolSiteFixture(t, root)
	t.Setenv("ADMIN_TOKEN", "secret-admin-token")
	if err := initRuntime(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(shutdownRuntime)

	// Configure the 次数站 token once on its first base_url line.
	saveAdminConfig(t, map[string]any{
		"newapi": map[string]any{
			"sites": []map[string]any{
				{"name": "线路 1", "category": "api", "url": "https://api-1.example.test", "token": "shared-token-1"},
			},
		},
	})

	// Add a second base_url line with a blank token (the UI never re-sends the
	// masked token). Both lines must end up on the one shared site token.
	saveAdminConfig(t, map[string]any{
		"newapi": map[string]any{
			"sites": []map[string]any{
				{"name": "线路 1", "category": "api", "url": "https://api-1.example.test"},
				{"name": "线路 2", "category": "api", "url": "https://api-2.example.test"},
			},
		},
	})

	snap := unifiedConfig.Snapshot()
	if len(snap.NewAPI.Sites) != 2 {
		t.Fatalf("expected 2 base_url lines, got %#v", snap.NewAPI.Sites)
	}
	for _, site := range snap.NewAPI.Sites {
		if site.Token != "shared-token-1" {
			t.Fatalf("line %q should inherit the one site token, got token=%q", site.Name, site.Token)
		}
	}

	// "线路 1" can coexist under both categories without a duplicate-name error.
	saveAdminConfig(t, map[string]any{
		"newapi": map[string]any{
			"sites": []map[string]any{
				{"name": "线路 1", "category": "api", "url": "https://api-1.example.test", "token": "shared-token-1"},
				{"name": "线路 1", "category": "token", "url": "https://token-1.example.test", "token": "shared-token-2"},
			},
		},
	})
	if got := unifiedConfig.Snapshot().NewAPI.Sites; len(got) != 2 {
		t.Fatalf("same line name across categories should be allowed, got %#v", got)
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
			"spinMap": map[string]int{
				"42": 3,
			},
			"prizePool": []map[string]any{
				{"type": "miss", "weight": 1},
				{"type": "win", "dollars": 9, "weight": 1},
			},
			"tierPools": map[string][]map[string]any{
				"42": {
					{"type": "miss", "weight": 1},
					{"type": "win", "dollars": 7, "weight": 3},
				},
			},
			"postJackpotPrizes": []map[string]any{
				{"type": "win", "dollars": 1, "weight": 1},
			},
			"scratchRewards": []int{2, 4, 6, 8, 10, 12},
		},
	})

	cfg := activityapp.SnapshotRuntimeConfig()
	if cfg.StartText != "2026-06-01 00:00:00" || cfg.EndText != "2026-06-30 23:59:59" || cfg.StartTS != 1780243200 || cfg.EndTS != 1782835199 {
		t.Fatalf("activity dates not applied: %#v", cfg)
	}
	if cfg.SpinMap[42] != 3 || len(cfg.TierPools[42]) != 2 || cfg.TierPools[42][1].Dollars != 7 || cfg.TierPools[42][1].Weight != 3 {
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

	prizesReq := httptest.NewRequest(http.MethodGet, "/api/prizes", nil)
	prizesW := httptest.NewRecorder()
	route(prizesW, prizesReq)
	if prizesW.Code != http.StatusOK {
		t.Fatalf("prizes code=%d body=%s", prizesW.Code, prizesW.Body.String())
	}
	if body := prizesW.Body.String(); !strings.Contains(body, `"42"`) || !strings.Contains(body, `"dollars":7`) || !strings.Contains(body, `"weight":3`) {
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
