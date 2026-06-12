package activityapp

import (
	"encoding/json"
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
