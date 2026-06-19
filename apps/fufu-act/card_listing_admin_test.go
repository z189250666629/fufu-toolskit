package activityapp

import (
	"encoding/json"
	"fmt"
	"fufu/activity"
	"fufu/newapi"
	"fufu/tokens"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestHandleAdminSaleCardsConfigReturnsPlansAndDefaultSchedule(t *testing.T) {
	setupSaleCardConfigTestRoot(t)
	t.Setenv("ADMIN_TOKEN", "test-admin-token")

	req := httptest.NewRequest(http.MethodGet, "/api/admin/sale-cards/config", nil)
	req.Header.Set("Authorization", "Bearer test-admin-token")
	w := httptest.NewRecorder()

	apiRoute(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	var body saleCardAdminConfigResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, w.Body.String())
	}
	if len(body.Plans) < 6 {
		t.Fatalf("plans=%#v, want default special/monthly plans", body.Plans)
	}
	if body.Schedule.Timezone != "Asia/Shanghai" || body.Schedule.Enabled {
		t.Fatalf("default schedule=%#v", body.Schedule)
	}
	if len(body.Schedule.Slots) != 2 {
		t.Fatalf("default schedule should have 2 slots (55卡/月卡), got %#v", body.Schedule.Slots)
	}
	if body.Schedule.Slots[0].Group != "special55" || body.Schedule.Slots[0].Time != "09:00" {
		t.Fatalf("first slot=%#v", body.Schedule.Slots[0])
	}
	if body.Schedule.Slots[1].Group != "month" || body.Schedule.Slots[1].Time != "09:30" {
		t.Fatalf("second slot=%#v", body.Schedule.Slots[1])
	}
	if body.Plans[0].ID == "" || body.Plans[0].Name == "" || body.Plans[0].Count != 0 || body.Plans[0].Slot == "" {
		t.Fatalf("plan should expose display metadata + slot but no run count default: %#v", body.Plans[0])
	}
}

func TestHandleAdminSaleCardsConfigPersistsValidatedSchedule(t *testing.T) {
	root := setupSaleCardConfigTestRoot(t)
	t.Setenv("ADMIN_TOKEN", "test-admin-token")
	payload := `{
		"schedule": {
			"enabled": true,
			"timezone": "Asia/Shanghai",
			"slots": [
				{"group": "special55", "time": "08:30", "enabled": true,
				 "jobs": [{"plan": "fufu-mix-special-55", "targetStock": 50, "enabled": true}]},
				{"group": "month", "time": "10:15", "enabled": false,
				 "jobs": [{"plan": "fufu-mix-month-100", "targetStock": 120, "enabled": false}]}
			]
		}
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/admin/sale-cards/config", strings.NewReader(payload))
	req.Header.Set("Authorization", "Bearer test-admin-token")
	w := httptest.NewRecorder()

	apiRoute(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	var body saleCardAdminConfigResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, w.Body.String())
	}
	if !body.Schedule.Enabled || len(body.Schedule.Slots) != 2 {
		t.Fatalf("saved schedule=%#v", body.Schedule)
	}
	special := body.Schedule.Slots[0]
	if special.Group != "special55" || special.Time != "08:30" || !special.Enabled || len(special.Jobs) != 1 {
		t.Fatalf("special slot=%#v", special)
	}
	if special.Jobs[0].Plan != "fufu-mix-special-55" || special.Jobs[0].TargetStock != 50 || !special.Jobs[0].Enabled {
		t.Fatalf("special job=%#v", special.Jobs[0])
	}
	month := body.Schedule.Slots[1]
	if month.Group != "month" || month.Time != "10:15" || month.Enabled || month.Jobs[0].TargetStock != 120 {
		t.Fatalf("month slot=%#v", month)
	}
	if _, err := os.Stat(filepath.Join(root, "data", "sale-card-schedule.json")); err != nil {
		t.Fatalf("schedule should be persisted: %v", err)
	}
}

func TestNormalizeSaleCardScheduleValidatesTimezone(t *testing.T) {
	setupSaleCardConfigTestRoot(t)
	base := SaleCardScheduleConfig{Enabled: true}

	base.Timezone = "Asia/Shanghai"
	got, err := normalizeSaleCardSchedule(base)
	if err != nil || got.Timezone != "Asia/Shanghai" || len(got.Slots) != 2 {
		t.Fatalf("valid tz should pass: schedule=%#v err=%v", got, err)
	}

	base.Timezone = "Asia/Shangha" // typo
	if _, err := normalizeSaleCardSchedule(base); err == nil || !strings.Contains(err.Error(), "时区") {
		t.Fatalf("typo tz should be rejected, err=%v", err)
	}

	base.Timezone = strings.Repeat("z", 65)
	if _, err := normalizeSaleCardSchedule(base); err == nil || !strings.Contains(err.Error(), "时区格式错误") {
		t.Fatalf("oversize tz should be rejected, err=%v", err)
	}
}

func TestNormalizeSlotJobsValidatesTargetRange(t *testing.T) {
	cases := []struct {
		name    string
		target  int
		wantErr bool
	}{
		{"zero ok", 0, false},
		{"max ok", 2000, false},
		{"over max", 2001, true},
		{"negative", -1, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := normalizeSlotJobs("special55", []SaleCardScheduleJob{
				{Plan: "fufu-mix-special-55", TargetStock: c.target, Enabled: true},
			})
			if c.wantErr && err == nil {
				t.Fatalf("target=%d should be rejected", c.target)
			}
			if !c.wantErr && err != nil {
				t.Fatalf("target=%d should be accepted, err=%v", c.target, err)
			}
		})
	}
}

func TestNormalizeSaleCardScheduleRejectsPlanInWrongSlot(t *testing.T) {
	setupSaleCardConfigTestRoot(t)
	// A month plan placed under the 55卡 slot must be rejected.
	_, err := normalizeSaleCardSchedule(SaleCardScheduleConfig{
		Enabled:  true,
		Timezone: "Asia/Shanghai",
		Slots: []SaleCardScheduleSlot{
			{Group: "special55", Time: "08:30", Enabled: true, Jobs: []SaleCardScheduleJob{
				{Plan: "fufu-mix-month-100", TargetStock: 10, Enabled: true},
			}},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "时段不匹配") {
		t.Fatalf("err=%v, want 时段不匹配", err)
	}
}

func TestHandleAdminSaleCardsConfigRejectsUnknownSchedulePlan(t *testing.T) {
	setupSaleCardConfigTestRoot(t)
	t.Setenv("ADMIN_TOKEN", "test-admin-token")
	payload := `{"schedule":{"enabled":true,"timezone":"Asia/Shanghai","slots":[{"group":"special55","time":"08:30","enabled":true,"jobs":[{"plan":"unknown","targetStock":1,"enabled":true}]}]}}`
	req := httptest.NewRequest(http.MethodPost, "/api/admin/sale-cards/config", strings.NewReader(payload))
	req.Header.Set("Authorization", "Bearer test-admin-token")
	w := httptest.NewRecorder()

	apiRoute(w, req)

	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "未知上架计划") {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestHandleAdminSaleCardsTestKeyGeneratesNewAPIOnlyAndReportsGameplay(t *testing.T) {
	t.Setenv("ADMIN_TOKEN", "test-admin-token")
	setSaleCardNowForTest(t, time.Date(2026, 6, 16, 12, 34, 56, 0, time.UTC))
	originalConfig := SnapshotRuntimeConfig()
	t.Cleanup(func() { SetRuntimeConfig(originalConfig) })
	cfg := activity.DefaultConfig()
	cfg.GameRoutes = []activity.GameRoute{{Dollars: 55, Game: activity.GameDragon, DrawCount: 6}}
	SetRuntimeConfig(cfg)

	var createHits atomic.Int32
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/token/" {
			t.Fatalf("unexpected NewAPI request %s %s", r.Method, r.URL.String())
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode NewAPI body: %v", err)
		}
		idx := createHits.Add(1)
		name := strings.TrimSpace(fmt.Sprint(body["name"]))
		if !strings.Contains(name, "55-act-test") {
			t.Fatalf("test key token name=%q, want activity-test marker", name)
		}
		if got, want := int64(body["remain_quota"].(float64)), int64(55_000); got != want {
			t.Fatalf("remain_quota=%d, want %d; body=%#v", got, want, body)
		}
		if body["group"] != "mix" || int(body["interval_unit"].(float64)) != 3 {
			t.Fatalf("unexpected token create body: %#v", body)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data":    map[string]any{"id": idx, "key": fmt.Sprintf("test-key-%d", idx), "name": name},
		})
	}))
	t.Cleanup(tokenSrv.Close)
	oldTokenSvc := tokenSvc
	oldTokenConfigErr := tokenConfigErr
	t.Cleanup(func() {
		tokenSvc = oldTokenSvc
		tokenConfigErr = oldTokenConfigErr
	})
	tokenConfigErr = nil
	tokenSvc = tokens.NewService(newapi.NewClient(newapi.Site{URL: tokenSrv.URL, Token: "test-token", UserID: "1", QuotaUnit: 1000}))

	var mcyHits atomic.Int32
	mcySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mcyHits.Add(1)
		t.Fatalf("test-key generation must not call MCY, got %s", r.URL.Path)
	}))
	t.Cleanup(mcySrv.Close)
	t.Setenv("MCY_BASE_URL", mcySrv.URL)

	req := httptest.NewRequest(http.MethodPost, "/api/admin/sale-cards/test-key", strings.NewReader(`{"plan":"fufu-mix-special-55","count":2}`))
	req.Header.Set("Authorization", "Bearer test-admin-token")
	w := httptest.NewRecorder()
	apiRoute(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	var body SaleCardTestKeyResult
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, w.Body.String())
	}
	if body.PlanID != "fufu-mix-special-55" || body.Generated != 2 || strings.Join(body.Keys, ",") != "sk-test-key-1,sk-test-key-2" {
		t.Fatalf("response keys/plan wrong: %#v", body)
	}
	if body.Game != activity.GameDragon || body.DrawCount != 6 {
		t.Fatalf("gameplay should come from activity config, got game=%q drawCount=%d", body.Game, body.DrawCount)
	}
	if createHits.Load() != 2 || mcyHits.Load() != 0 {
		t.Fatalf("createHits=%d mcyHits=%d, want 2/0", createHits.Load(), mcyHits.Load())
	}
}

func TestHandleAdminSaleCardsTestKeyRequiresAdminToken(t *testing.T) {
	t.Setenv("ADMIN_TOKEN", "test-admin-token")
	req := httptest.NewRequest(http.MethodPost, "/api/admin/sale-cards/test-key", strings.NewReader(`{"plan":"fufu-mix-special-55"}`))
	w := httptest.NewRecorder()
	apiRoute(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized test-key query should be 401, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestHandleAdminSaleCardsStockReportsCurrentPerPlan(t *testing.T) {
	t.Setenv("ADMIN_TOKEN", "test-admin-token")
	setMCYCookieForTest(t, "manage_token=test")

	var stockHits atomic.Int32
	mcySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/plugin/virtual-card-ship/card/get" {
			t.Fatalf("stock query should only hit MCY card/get, got %s", r.URL.Path)
		}
		stockHits.Add(1)
		// Each plan is a precise equal-* query; the shop narrows data.total per sku.
		// 55卡 (sku66) reports 7; month-100 (sku65) reports 3; every other sku 0.
		payload := testDecodeMCYRequest(t, r.Body, r.Header.Get("Secret"))
		if int(payload["equal-status"].(float64)) != 0 {
			t.Fatalf("stock query must filter equal-status=0, got %v", payload["equal-status"])
		}
		total := 0
		switch int(payload["equal-sku_id"].(float64)) {
		case 66:
			total = 7
		case 65:
			total = 3
		}
		testWriteEncryptedMCYResponse(t, w, map[string]any{"code": 200, "data": map[string]any{"total": total, "list": []any{}}})
	}))
	t.Cleanup(mcySrv.Close)
	t.Setenv("MCY_BASE_URL", mcySrv.URL)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/sale-cards/stock", nil)
	req.Header.Set("Authorization", "Bearer test-admin-token")
	w := httptest.NewRecorder()
	apiRoute(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Stock []struct {
			PlanID       string `json:"planId"`
			Slot         string `json:"slot"`
			CurrentStock int    `json:"currentStock"`
		} `json:"stock"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v; body=%s", err, w.Body.String())
	}
	if len(body.Stock) < 6 {
		t.Fatalf("expected stock for all plans, got %#v", body.Stock)
	}
	var special, month100, month500 int = -1, -1, -1
	for _, s := range body.Stock {
		switch s.PlanID {
		case "fufu-mix-special-55":
			special = s.CurrentStock
			if s.Slot != "special55" {
				t.Fatalf("special slot=%q", s.Slot)
			}
		case "fufu-mix-month-100":
			month100 = s.CurrentStock
		case "fufu-mix-month-500":
			month500 = s.CurrentStock
		}
	}
	if special != 7 || month100 != 3 {
		t.Fatalf("per-plan stock wrong: special=%d month100=%d", special, month100)
	}
	// A SKU with no unsold cards must read 0, not the global total.
	if month500 != 0 {
		t.Fatalf("month500 (no unsold cards) stock=%d, want 0", month500)
	}
	if got, want := int(stockHits.Load()), len(saleCardPlanList()); got != want {
		t.Fatalf("expected one precise query per plan: %d hits for %d plans", got, want)
	}
}

// TestHandleAdminSaleCardsStockQueriesSequentially proves the per-plan stock
// queries run one at a time — the shop rejects concurrent requests on a single
// session with 登录已过期.
func TestHandleAdminSaleCardsStockQueriesSequentially(t *testing.T) {
	t.Setenv("ADMIN_TOKEN", "test-admin-token")
	setMCYCookieForTest(t, "manage_token=test")

	var inFlight, maxInFlight atomic.Int32
	mcySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cur := inFlight.Add(1)
		for {
			m := maxInFlight.Load()
			if cur <= m || maxInFlight.CompareAndSwap(m, cur) {
				break
			}
		}
		time.Sleep(15 * time.Millisecond) // hold the request so any overlap is observable
		inFlight.Add(-1)
		testWriteEncryptedMCYResponse(t, w, map[string]any{"code": 200, "data": map[string]any{"total": 1, "list": []any{}}})
	}))
	t.Cleanup(mcySrv.Close)
	t.Setenv("MCY_BASE_URL", mcySrv.URL)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/sale-cards/stock", nil)
	req.Header.Set("Authorization", "Bearer test-admin-token")
	w := httptest.NewRecorder()
	apiRoute(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	if got := maxInFlight.Load(); got != 1 {
		t.Fatalf("stock queries must be sequential (shop rejects concurrency), max concurrent=%d", got)
	}
}

func TestHandleAdminSaleCardsStockRequiresAdminToken(t *testing.T) {
	t.Setenv("ADMIN_TOKEN", "test-admin-token")
	req := httptest.NewRequest(http.MethodGet, "/api/admin/sale-cards/stock", nil)
	w := httptest.NewRecorder()
	apiRoute(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized stock query should be 401, got %d", w.Code)
	}
}

func TestHandleAdminSaleCardsStockReportsMCYCredentialHint(t *testing.T) {
	t.Setenv("ADMIN_TOKEN", "test-admin-token")
	setMCYCookieForTest(t, "manage_token=stale")

	var loginHits atomic.Int32
	mcySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/plugin/virtual-card-ship/card/get":
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`unauthorized`))
		case "/admin/login", "/admin":
			loginHits.Add(1)
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`bad password`))
		default:
			t.Fatalf("unexpected MCY request %s", r.URL.Path)
		}
	}))
	t.Cleanup(mcySrv.Close)
	t.Setenv("MCY_BASE_URL", mcySrv.URL)
	t.Setenv("MCY_USERNAME", "u")
	t.Setenv("MCY_PASSWORD", "wrong")

	req := httptest.NewRequest(http.MethodGet, "/api/admin/sale-cards/stock", nil)
	req.Header.Set("Authorization", "Bearer test-admin-token")
	w := httptest.NewRecorder()
	apiRoute(w, req)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "请检查商城账号或密码") {
		t.Fatalf("credential hint missing: %s", w.Body.String())
	}
	if loginHits.Load() != 0 {
		t.Fatalf("HTTP 401 from MCY card/get should not trigger relogin, got %d login hits", loginHits.Load())
	}
}

func setupSaleCardConfigTestRoot(t *testing.T) string {
	t.Helper()
	oldRoot := rootDir
	root := t.TempDir()
	rootDir = root
	t.Cleanup(func() { rootDir = oldRoot })
	return root
}
