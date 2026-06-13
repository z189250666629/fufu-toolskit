package activityapp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"fufu/newapi"
	"fufu/tokens"
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

func TestHandleAdminSaleCardsStockReportsCurrentPerPlan(t *testing.T) {
	t.Setenv("ADMIN_TOKEN", "test-admin-token")

	var searchHits atomic.Int32
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/token/search" {
			t.Fatalf("stock query should only hit token search, got %s", r.URL.Path)
		}
		searchHits.Add(1)
		// special55 keyword reports 7, every other plan reports 3.
		total := "3"
		if strings.Contains(r.URL.Query().Get("keyword"), "fufu-mix-special-55-") {
			total = "7"
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]any{"items": []any{}, "total": json.Number(total)}})
	}))
	t.Cleanup(tokenSrv.Close)

	oldSvc, oldErr := tokenSvc, tokenConfigErr
	t.Cleanup(func() { tokenSvc, tokenConfigErr = oldSvc, oldErr })
	tokenConfigErr = nil
	tokenSvc = tokens.NewService(newapi.NewClient(newapi.Site{URL: tokenSrv.URL, Token: "test-token", UserID: "1", QuotaUnit: 1000}))

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
	var special, month100 int = -1, -1
	for _, s := range body.Stock {
		if s.PlanID == "fufu-mix-special-55" {
			special = s.CurrentStock
			if s.Slot != "special55" {
				t.Fatalf("special slot=%q", s.Slot)
			}
		}
		if s.PlanID == "fufu-mix-month-100" {
			month100 = s.CurrentStock
		}
	}
	if special != 7 || month100 != 3 {
		t.Fatalf("per-plan stock wrong: special=%d month100=%d", special, month100)
	}
	if searchHits.Load() < 6 {
		t.Fatalf("each plan should be queried, hits=%d", searchHits.Load())
	}
}

func TestHandleAdminSaleCardsStockQueriesPlansConcurrently(t *testing.T) {
	t.Setenv("ADMIN_TOKEN", "test-admin-token")

	const perCall = 80 * time.Millisecond
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(perCall)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]any{"items": []any{}, "total": json.Number("1")}})
	}))
	t.Cleanup(tokenSrv.Close)

	oldSvc, oldErr := tokenSvc, tokenConfigErr
	t.Cleanup(func() { tokenSvc, tokenConfigErr = oldSvc, oldErr })
	tokenConfigErr = nil
	tokenSvc = tokens.NewService(newapi.NewClient(newapi.Site{URL: tokenSrv.URL, Token: "t", UserID: "1", QuotaUnit: 1000}))

	planCount := len(saleCardPlanList())
	req := httptest.NewRequest(http.MethodGet, "/api/admin/sale-cards/stock", nil)
	req.Header.Set("Authorization", "Bearer test-admin-token")
	w := httptest.NewRecorder()

	start := time.Now()
	apiRoute(w, req)
	elapsed := time.Since(start)

	if w.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	// Serial would be planCount*perCall; concurrent should be ~perCall. Use a
	// generous ceiling (half the serial time) to stay non-flaky.
	if serial := time.Duration(planCount) * perCall; elapsed >= serial/2 {
		t.Fatalf("stock queries look serial: %d plans took %s (serial≈%s)", planCount, elapsed, serial)
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

func setupSaleCardConfigTestRoot(t *testing.T) string {
	t.Helper()
	oldRoot := rootDir
	root := t.TempDir()
	rootDir = root
	t.Cleanup(func() { rootDir = oldRoot })
	return root
}
