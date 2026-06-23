package activityapp

import (
	"context"
	"fmt"
	"fufu/salecore"
	"sync"
	"time"
	// Embed the IANA timezone database so time.LoadLocation works on any host
	// (Windows / minimal containers) without relying on system tzdata. The
	// 补卡 scheduler fires per the admin-configured timezone.
	_ "time/tzdata"
)

// saleCardSchedulerTick controls how often the scheduler re-checks the clock.
// A sub-minute interval guarantees each HH:MM slot is observed at least once.
var saleCardSchedulerTick = 30 * time.Second

// saleCardRestockTimeout bounds a single slot run (stock query + token create +
// MCY upload across every enabled job).
var saleCardRestockTimeout = 5 * time.Minute

// saleCardNowFunc is overridable in tests; production uses the wall clock.
var saleCardNowFunc = time.Now

var startSaleCardScheduler = func() {
	go saleCardScheduler()
}

// saleCardFiredGuard remembers the last calendar day each slot fired, so a slot
// runs at most once per day even though the ticker polls every 30s. Restock is
// idempotent (it tops up to target), so this is a politeness guard, not a
// correctness requirement.
var saleCardFiredGuard = struct {
	sync.Mutex
	day map[string]string
}{day: map[string]string{}}

func saleCardScheduler() {
	ticker := time.NewTicker(saleCardSchedulerTick)
	defer ticker.Stop()
	for {
		runSaleCardSchedulerOnce(saleCardNowFunc())
		<-ticker.C
	}
}

func runSaleCardSchedulerOnce(now time.Time) {
	runDueSaleCardSlots(now)
	processSaleCardRestockJobs(now)
}

// runDueSaleCardSlots enqueues every enabled slot whose configured HH:MM
// matches the current minute in the schedule's timezone. The DB unique key
// guarantees a plan is only queued once per business day; the worker then
// retries only incomplete per-plan jobs.
func runDueSaleCardSlots(now time.Time) {
	schedule, err := loadSaleCardSchedule()
	if err != nil || !schedule.Enabled {
		return
	}
	loc := saleCardScheduleLocation(schedule.Timezone)
	local := now.In(loc)
	hhmm := local.Format("15:04")
	day := local.Format("2006-01-02")
	for _, slot := range schedule.Slots {
		if !slot.Enabled || !saleCardSlotDue(slot.Time, hhmm) {
			continue
		}
		if err := enqueueSaleCardRestockJobs(day, slot); err != nil {
			fmt.Printf("[sale-card] enqueue slot %s failed: %v\n", slot.Group, err)
		}
	}
}

func saleCardSlotDue(slotTime, currentHHMM string) bool {
	return salecore.SlotDue(slotTime, currentHHMM)
}

func saleCardSlotFireKey(slot SaleCardScheduleSlot) string {
	return salecore.SlotFireKey(slot)
}

// markSaleCardSlotFired records that a slot fired on the given day and reports
// whether this call won the race (true = caller should run the slot).
func markSaleCardSlotFired(key, day string) bool {
	saleCardFiredGuard.Lock()
	defer saleCardFiredGuard.Unlock()
	if saleCardFiredGuard.day[key] == day {
		return false
	}
	saleCardFiredGuard.day[key] = day
	return true
}

func saleCardScheduleLocation(timezone string) *time.Location {
	if loc, err := time.LoadLocation(timezone); err == nil {
		return loc
	}
	return time.UTC
}

// runSaleCardSlot restocks every enabled job in the slot to its target stock.
func runSaleCardSlot(slot SaleCardScheduleSlot) {
	service, _ := snapshotTokenRuntime()
	if service == nil {
		fmt.Printf("[sale-card] slot %s skipped: 次数 fufu 未配置\n", slot.Group)
		return
	}
	templates := saleCardPlanTemplates()
	for _, job := range slot.Jobs {
		if !job.Enabled {
			continue
		}
		// An enabled job that gets skipped is a misconfiguration — make it audible
		// instead of silently never firing.
		if job.TargetStock <= 0 {
			fmt.Printf("[sale-card] skip %s: enabled but target=%d\n", job.Plan, job.TargetStock)
			continue
		}
		plan, ok := templates[job.Plan]
		if !ok {
			fmt.Printf("[sale-card] skip %s: unknown plan in slot %s\n", job.Plan, slot.Group)
			continue
		}
		plan.TargetStock = job.TargetStock
		plan.Count = 0
		ctx, cancel := context.WithTimeout(context.Background(), saleCardRestockTimeout)
		result, err := generateAndUploadSaleCards(ctx, service, plan)
		cancel()
		if err != nil {
			fmt.Printf("[sale-card] restock %s failed: %v\n", job.Plan, err)
			continue
		}
		fmt.Printf("[sale-card] restock %s: current=%d target=%d uploaded=%d\n", job.Plan, result.CurrentStock, result.TargetStock, result.Uploaded)
	}
}
