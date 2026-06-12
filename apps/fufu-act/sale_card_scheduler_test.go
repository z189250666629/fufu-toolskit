package activityapp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"fufu/newapi"
	"fufu/tokens"
)

func resetSaleCardFiredGuard(t *testing.T) {
	t.Helper()
	saleCardFiredGuard.Lock()
	saleCardFiredGuard.day = map[string]string{}
	saleCardFiredGuard.Unlock()
}

// restockBackend wires a NewAPI stub (search returns total=0 so any target
// uploads, create returns N keys) plus an MCY upload stub, and points tokenSvc
// at it. The returned counters track how often each leg is hit.
func restockBackend(t *testing.T) (*atomic.Int32, *atomic.Int32, *atomic.Int32) {
	t.Helper()
	setMCYCookieForTest(t, "manage_token=test")
	var search, create, upload atomic.Int32
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/token/search":
			search.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]any{"items": []any{}, "total": json.Number("0")}})
		case "/api/token/tokens":
			create.Add(1)
			n := 1
			if c := r.URL.Query().Get("tokenCount"); c != "" {
				fmt.Sscanf(c, "%d", &n)
			}
			items := make([]any, 0, n)
			for i := range n {
				items = append(items, map[string]any{"id": i + 1, "key": fmt.Sprintf("key-%d", i)})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": items})
		default:
			t.Fatalf("unexpected NewAPI request %s", r.URL.Path)
		}
	}))
	t.Cleanup(tokenSrv.Close)
	mcySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upload.Add(1)
		testWriteEncryptedMCYResponse(t, w, map[string]any{"code": 200, "msg": "ok"})
	}))
	t.Cleanup(mcySrv.Close)
	t.Setenv("MCY_BASE_URL", mcySrv.URL)
	oldSvc := tokenSvc
	tokenSvc = tokens.NewService(newapi.NewClient(newapi.Site{URL: tokenSrv.URL, Token: "test-token", UserID: "1", QuotaUnit: 1000}))
	t.Cleanup(func() { tokenSvc = oldSvc })
	return &search, &create, &upload
}

func TestRunSaleCardSlotSkipsNonRunnableJobs(t *testing.T) {
	cases := []struct {
		name string
		jobs []SaleCardScheduleJob
		runs int32
	}{
		{"disabled", []SaleCardScheduleJob{{Plan: "fufu-mix-special-55", TargetStock: 20, Enabled: false}}, 0},
		{"zero target", []SaleCardScheduleJob{{Plan: "fufu-mix-special-55", TargetStock: 0, Enabled: true}}, 0},
		{"negative target", []SaleCardScheduleJob{{Plan: "fufu-mix-special-55", TargetStock: -5, Enabled: true}}, 0},
		{"unknown plan", []SaleCardScheduleJob{{Plan: "ghost-plan", TargetStock: 20, Enabled: true}}, 0},
		{"runnable", []SaleCardScheduleJob{{Plan: "fufu-mix-special-55", TargetStock: 20, Enabled: true}}, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			search, _, _ := restockBackend(t)
			runSaleCardSlot(SaleCardScheduleSlot{Group: "special55", Jobs: c.jobs})
			if got := search.Load(); got != c.runs {
				t.Fatalf("restock runs=%d, want %d", got, c.runs)
			}
		})
	}
}

func TestRunSaleCardSlotSkipsWhenTokenSvcNil(t *testing.T) {
	oldSvc := tokenSvc
	tokenSvc = nil
	t.Cleanup(func() { tokenSvc = oldSvc })
	// Must not panic; just returns after logging the missing service.
	runSaleCardSlot(SaleCardScheduleSlot{Group: "special55", Jobs: []SaleCardScheduleJob{
		{Plan: "fufu-mix-special-55", TargetStock: 20, Enabled: true},
	}})
}

func TestSaleCardScheduleLocationLoadsOrFallsBack(t *testing.T) {
	if loc := saleCardScheduleLocation("Asia/Shanghai"); loc == time.UTC || loc.String() != "Asia/Shanghai" {
		t.Fatalf("valid tz should load, got %v", loc)
	}
	if loc := saleCardScheduleLocation("Invalid/Nowhere"); loc != time.UTC {
		t.Fatalf("invalid tz should fall back to UTC, got %v", loc)
	}
}

func TestRunDueSaleCardSlotsFiresSlotsIndependently(t *testing.T) {
	setupSaleCardConfigTestRoot(t)
	resetSaleCardFiredGuard(t)
	search, _, _ := restockBackend(t)

	schedule := SaleCardScheduleConfig{
		Enabled:  true,
		Timezone: "UTC",
		Slots: []SaleCardScheduleSlot{
			{Group: "special55", Time: "08:00", Enabled: true, Jobs: []SaleCardScheduleJob{{Plan: "fufu-mix-special-55", TargetStock: 10, Enabled: true}}},
			{Group: "month", Time: "09:00", Enabled: true, Jobs: []SaleCardScheduleJob{{Plan: "fufu-mix-month-100", TargetStock: 10, Enabled: true}}},
		},
	}
	if err := saveSaleCardSchedule(schedule); err != nil {
		t.Fatalf("saveSaleCardSchedule: %v", err)
	}

	runDueSaleCardSlots(time.Date(2026, 6, 13, 8, 0, 0, 0, time.UTC)) // only special55
	if search.Load() != 1 {
		t.Fatalf("after 08:00 only special55 should fire, search=%d", search.Load())
	}
	runDueSaleCardSlots(time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC)) // only month
	if search.Load() != 2 {
		t.Fatalf("after 09:00 month should also fire, search=%d", search.Load())
	}
	runDueSaleCardSlots(time.Date(2026, 6, 13, 8, 0, 0, 0, time.UTC)) // same-day dedup
	if search.Load() != 2 {
		t.Fatalf("same-day re-run must not refire, search=%d", search.Load())
	}
	runDueSaleCardSlots(time.Date(2026, 6, 14, 8, 0, 0, 0, time.UTC)) // next day special55 again
	if search.Load() != 3 {
		t.Fatalf("next-day special55 should fire again, search=%d", search.Load())
	}
}

// TestRunDueSaleCardSlotsFiresEnabledSlotAtItsTime exercises the full scheduler
// path: the 55卡 slot fires only at its configured minute, restocks to target
// via the real NewAPI name query, and refuses to re-run the same day.
func TestRunDueSaleCardSlotsFiresEnabledSlotAtItsTime(t *testing.T) {
	setupSaleCardConfigTestRoot(t)
	resetSaleCardFiredGuard(t)
	setMCYCookieForTest(t, "manage_token=test")

	var searchHits, createHits, uploadHits atomic.Int32
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/token/search":
			searchHits.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]any{"items": []any{}, "total": json.Number("8")}})
		case "/api/token/tokens":
			createHits.Add(1)
			if got := r.URL.Query().Get("tokenCount"); got != "12" {
				t.Fatalf("tokenCount=%q, want 12 (target 20 - current 8)", got)
			}
			items := make([]any, 0, 12)
			for i := range 12 {
				items = append(items, map[string]any{"id": i + 1, "key": "scheduled-key"})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": items})
		default:
			t.Fatalf("unexpected NewAPI request %s", r.URL.Path)
		}
	}))
	t.Cleanup(tokenSrv.Close)

	mcySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uploadHits.Add(1)
		testWriteEncryptedMCYResponse(t, w, map[string]any{"code": 200, "msg": "ok"})
	}))
	t.Cleanup(mcySrv.Close)
	t.Setenv("MCY_BASE_URL", mcySrv.URL)

	oldSvc := tokenSvc
	tokenSvc = tokens.NewService(newapi.NewClient(newapi.Site{URL: tokenSrv.URL, Token: "test-token", UserID: "1", QuotaUnit: 1000}))
	t.Cleanup(func() { tokenSvc = oldSvc })

	schedule := SaleCardScheduleConfig{
		Enabled:  true,
		Timezone: "UTC",
		Slots: []SaleCardScheduleSlot{
			{Group: "special55", Time: "08:30", Enabled: true, Jobs: []SaleCardScheduleJob{
				{Plan: "fufu-mix-special-55", TargetStock: 20, Enabled: true},
			}},
			{Group: "month", Time: "09:30", Enabled: false, Jobs: []SaleCardScheduleJob{
				{Plan: "fufu-mix-month-100", TargetStock: 30, Enabled: true},
			}},
		},
	}
	if err := saveSaleCardSchedule(schedule); err != nil {
		t.Fatalf("saveSaleCardSchedule: %v", err)
	}

	// Wrong minute → nothing fires.
	runDueSaleCardSlots(time.Date(2026, 6, 13, 8, 29, 0, 0, time.UTC))
	if searchHits.Load() != 0 || uploadHits.Load() != 0 {
		t.Fatalf("slot fired off-schedule: search=%d upload=%d", searchHits.Load(), uploadHits.Load())
	}

	// Matching minute → the 55卡 slot fires once and restocks 20-8=12.
	runDueSaleCardSlots(time.Date(2026, 6, 13, 8, 30, 30, 0, time.UTC))
	if searchHits.Load() != 1 || createHits.Load() != 1 || uploadHits.Load() != 1 {
		t.Fatalf("expected one restock: search=%d create=%d upload=%d", searchHits.Load(), createHits.Load(), uploadHits.Load())
	}

	// Same day, same minute → dedup guard blocks a second run.
	runDueSaleCardSlots(time.Date(2026, 6, 13, 8, 30, 45, 0, time.UTC))
	if searchHits.Load() != 1 {
		t.Fatalf("slot should fire at most once per day, search=%d", searchHits.Load())
	}
}

func TestRunDueSaleCardSlotsSkipsWhenDisabled(t *testing.T) {
	setupSaleCardConfigTestRoot(t)
	resetSaleCardFiredGuard(t)
	oldSvc := tokenSvc
	tokenSvc = tokens.NewService(newapi.NewClient(newapi.Site{URL: "http://127.0.0.1:0", Token: "x", UserID: "1"}))
	t.Cleanup(func() { tokenSvc = oldSvc })

	schedule := SaleCardScheduleConfig{
		Enabled:  false,
		Timezone: "UTC",
		Slots: []SaleCardScheduleSlot{
			{Group: "special55", Time: "08:30", Enabled: true, Jobs: []SaleCardScheduleJob{
				{Plan: "fufu-mix-special-55", TargetStock: 20, Enabled: true},
			}},
		},
	}
	if err := saveSaleCardSchedule(schedule); err != nil {
		t.Fatalf("saveSaleCardSchedule: %v", err)
	}
	// Schedule disabled at the top level → no panic, no fire (would error on the
	// unreachable NewAPI host if it tried).
	runDueSaleCardSlots(time.Date(2026, 6, 13, 8, 30, 0, 0, time.UTC))
}

func TestSaleCardSchedulerStartIsOverridable(t *testing.T) {
	// Guard against accidental real-goroutine starts in embedded tests.
	called := false
	old := startSaleCardScheduler
	startSaleCardScheduler = func() { called = true }
	t.Cleanup(func() { startSaleCardScheduler = old })
	startSaleCardScheduler()
	if !called {
		t.Fatal("startSaleCardScheduler override not honored")
	}
}
