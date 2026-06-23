package activityapp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
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

// restockBackend wires the real restock dependencies: an MCY stub answering the
// encrypted card/get (reports stockTotal as data.total) and card/add (upload),
// plus a NewAPI stub that mints token keys. The returned counters track stock
// queries, NewAPI create requests and MCY uploads.
func restockBackend(t *testing.T, stockTotal int) (*atomic.Int32, *atomic.Int32, *atomic.Int32) {
	t.Helper()
	setMCYCookieForTest(t, "manage_token=test")
	var stockQ, create, upload, lastBatchCount atomic.Int32
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/token/tokens":
			create.Add(1)
			count, err := strconv.Atoi(r.URL.Query().Get("tokenCount"))
			if err != nil || count <= 1 {
				t.Fatalf("bad tokenCount=%q", r.URL.Query().Get("tokenCount"))
			}
			lastBatchCount.Store(int32(count))
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "message": fmt.Sprintf("成功添加%d个令牌", count)})
		case r.Method == http.MethodGet && r.URL.Path == "/api/token/search":
			prefix := r.URL.Query().Get("keyword")
			count := int(lastBatchCount.Load())
			items := []any{}
			for i := 1; i <= count; i++ {
				items = append(items, map[string]any{"id": i, "name": fmt.Sprintf("%s-%02d", prefix, i), "key": fmt.Sprintf("key-%d", i)})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": items})
		case r.Method == http.MethodPost && r.URL.Path == "/api/token/":
			idx := create.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]any{"id": idx, "key": fmt.Sprintf("key-%d", idx)}})
		default:
			t.Fatalf("unexpected NewAPI request %s %s", r.Method, r.URL.String())
		}
	}))
	t.Cleanup(tokenSrv.Close)
	mcySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/plugin/virtual-card-ship/card/get":
			stockQ.Add(1)
			testWriteEncryptedMCYResponse(t, w, map[string]any{"code": 200, "data": map[string]any{"total": stockTotal, "list": []any{}}})
		case "/plugin/virtual-card-ship/card/add":
			upload.Add(1)
			testWriteEncryptedMCYResponse(t, w, map[string]any{"code": 200, "msg": "ok"})
		default:
			t.Fatalf("unexpected MCY request %s", r.URL.Path)
		}
	}))
	t.Cleanup(mcySrv.Close)
	t.Setenv("MCY_BASE_URL", mcySrv.URL)
	oldSvc := tokenSvc
	tokenSvc = tokens.NewService(newapi.NewClient(newapi.Site{URL: tokenSrv.URL, Token: "test-token", UserID: "1", QuotaUnit: 1000}))
	t.Cleanup(func() { tokenSvc = oldSvc })
	return &stockQ, &create, &upload
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
			stockQ, _, _ := restockBackend(t, 0)
			runSaleCardSlot(SaleCardScheduleSlot{Group: "special55", Jobs: c.jobs})
			if got := stockQ.Load(); got != c.runs {
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
	stockQ, _, _ := restockBackend(t, 0)

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
	if stockQ.Load() != 1 {
		t.Fatalf("after 08:00 only special55 should fire, stockQueries=%d", stockQ.Load())
	}
	runDueSaleCardSlots(time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC)) // only month
	if stockQ.Load() != 2 {
		t.Fatalf("after 09:00 month should also fire, stockQueries=%d", stockQ.Load())
	}
	runDueSaleCardSlots(time.Date(2026, 6, 13, 8, 0, 0, 0, time.UTC)) // same-day dedup
	if stockQ.Load() != 2 {
		t.Fatalf("same-day re-run must not refire, stockQueries=%d", stockQ.Load())
	}
	runDueSaleCardSlots(time.Date(2026, 6, 14, 8, 0, 0, 0, time.UTC)) // next day special55 again
	if stockQ.Load() != 3 {
		t.Fatalf("next-day special55 should fire again, stockQueries=%d", stockQ.Load())
	}
}

func TestRunDueSaleCardSlotsDoesNotCatchUpEarlierEnabledSpecial55Slot(t *testing.T) {
	setupSaleCardConfigTestRoot(t)
	resetSaleCardFiredGuard(t)
	stockQ, create, upload := restockBackend(t, 1)

	schedule := SaleCardScheduleConfig{
		Enabled:  true,
		Timezone: "Asia/Shanghai",
		Slots: []SaleCardScheduleSlot{
			{Group: "special55", Time: "19:54", Enabled: true, Jobs: []SaleCardScheduleJob{{Plan: "fufu-mix-special-55", TargetStock: 2, Enabled: true}}},
			{Group: "month", Time: "23:30", Enabled: true, Jobs: []SaleCardScheduleJob{{Plan: "fufu-mix-month-100", TargetStock: 10, Enabled: true}}},
		},
	}
	if err := saveSaleCardSchedule(schedule); err != nil {
		t.Fatalf("saveSaleCardSchedule: %v", err)
	}

	runDueSaleCardSlots(time.Date(2026, 6, 16, 19, 58, 0, 0, time.FixedZone("CST", 8*60*60)))

	if stockQ.Load() != 0 || create.Load() != 0 || upload.Load() != 0 {
		t.Fatalf("special55 should not catch up after its configured minute: stock=%d create=%d upload=%d", stockQ.Load(), create.Load(), upload.Load())
	}
	runDueSaleCardSlots(time.Date(2026, 6, 16, 23, 30, 0, 0, time.FixedZone("CST", 8*60*60)))
	if stockQ.Load() != 1 || create.Load() != 1 || upload.Load() != 1 {
		t.Fatalf("month should fire only at its configured minute: stock=%d create=%d upload=%d", stockQ.Load(), create.Load(), upload.Load())
	}
	runDueSaleCardSlots(time.Date(2026, 6, 16, 23, 31, 0, 0, time.FixedZone("CST", 8*60*60)))
	if stockQ.Load() != 1 {
		t.Fatalf("same-day after-minute run must not refire, stock=%d", stockQ.Load())
	}
}

func TestRunDueSaleCardSlotsRefiresWhenSpecial55TimeChangesSameDay(t *testing.T) {
	setupSaleCardConfigTestRoot(t)
	resetSaleCardFiredGuard(t)
	stockQ, create, upload := restockBackend(t, 1)

	schedule := SaleCardScheduleConfig{
		Enabled:  true,
		Timezone: "Asia/Shanghai",
		Slots: []SaleCardScheduleSlot{
			{Group: "special55", Time: "19:54", Enabled: true, Jobs: []SaleCardScheduleJob{{Plan: "fufu-mix-special-55", TargetStock: 2, Enabled: true}}},
		},
	}
	if err := saveSaleCardSchedule(schedule); err != nil {
		t.Fatalf("saveSaleCardSchedule: %v", err)
	}

	runDueSaleCardSlots(time.Date(2026, 6, 16, 19, 54, 0, 0, time.FixedZone("CST", 8*60*60)))
	if stockQ.Load() != 1 || create.Load() != 1 || upload.Load() != 1 {
		t.Fatalf("initial special55 fire failed: stock=%d create=%d upload=%d", stockQ.Load(), create.Load(), upload.Load())
	}

	schedule.Slots[0].Time = "20:04"
	if err := saveSaleCardSchedule(schedule); err != nil {
		t.Fatalf("saveSaleCardSchedule updated time: %v", err)
	}
	runDueSaleCardSlots(time.Date(2026, 6, 16, 20, 4, 0, 0, time.FixedZone("CST", 8*60*60)))
	if stockQ.Load() != 2 || create.Load() != 2 || upload.Load() != 2 {
		t.Fatalf("special55 should re-evaluate after same-day time change: stock=%d create=%d upload=%d", stockQ.Load(), create.Load(), upload.Load())
	}
}

// TestRunDueSaleCardSlotsFiresEnabledSlotAtItsTime exercises the full scheduler
// path: the 55卡 slot fires only at its configured minute, restocks to target
// (MCY reports 8 in stock, target 20 → create 12), and refuses to re-run the
// same day.
func TestRunDueSaleCardSlotsFiresEnabledSlotAtItsTime(t *testing.T) {
	setupSaleCardConfigTestRoot(t)
	resetSaleCardFiredGuard(t)
	setMCYCookieForTest(t, "manage_token=test")

	var stockHits, createHits, uploadHits, lastBatchCount atomic.Int32
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/token/tokens":
			createHits.Add(1)
			count, err := strconv.Atoi(r.URL.Query().Get("tokenCount"))
			if err != nil || count <= 1 {
				t.Fatalf("bad tokenCount=%q", r.URL.Query().Get("tokenCount"))
			}
			lastBatchCount.Store(int32(count))
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "message": fmt.Sprintf("成功添加%d个令牌", count)})
		case r.Method == http.MethodGet && r.URL.Path == "/api/token/search":
			prefix := r.URL.Query().Get("keyword")
			items := []any{}
			for i := 1; i <= int(lastBatchCount.Load()); i++ {
				items = append(items, map[string]any{"id": i, "name": fmt.Sprintf("%s-%02d", prefix, i), "key": "scheduled-key"})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": items})
		default:
			t.Fatalf("unexpected NewAPI request %s %s", r.Method, r.URL.String())
		}
	}))
	t.Cleanup(tokenSrv.Close)

	mcySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/plugin/virtual-card-ship/card/get":
			stockHits.Add(1)
			// Shop reports 8 unsold 55卡 (equal-* → data.total); target 20 → create 12.
			testWriteEncryptedMCYResponse(t, w, map[string]any{"code": 200, "data": map[string]any{"total": 8, "list": []any{}}})
		case "/plugin/virtual-card-ship/card/add":
			uploadHits.Add(1)
			testWriteEncryptedMCYResponse(t, w, map[string]any{"code": 200, "msg": "ok"})
		default:
			t.Fatalf("unexpected MCY request %s", r.URL.Path)
		}
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
	if stockHits.Load() != 0 || uploadHits.Load() != 0 {
		t.Fatalf("slot fired off-schedule: search=%d upload=%d", stockHits.Load(), uploadHits.Load())
	}

	// Matching minute → the 55卡 slot fires once and restocks 20-8=12.
	runDueSaleCardSlots(time.Date(2026, 6, 13, 8, 30, 30, 0, time.UTC))
	if stockHits.Load() != 1 || createHits.Load() != 1 || uploadHits.Load() != 1 {
		t.Fatalf("expected one restock: search=%d create=%d upload=%d", stockHits.Load(), createHits.Load(), uploadHits.Load())
	}

	// Same day, same minute → dedup guard blocks a second run.
	runDueSaleCardSlots(time.Date(2026, 6, 13, 8, 30, 45, 0, time.UTC))
	if stockHits.Load() != 1 {
		t.Fatalf("slot should fire at most once per day, search=%d", stockHits.Load())
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
