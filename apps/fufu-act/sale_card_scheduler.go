package activityapp

import (
	"context"
	"fmt"
	"sync"
	"time"
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
		runDueSaleCardSlots(saleCardNowFunc())
		<-ticker.C
	}
}

// runDueSaleCardSlots fires every enabled slot whose configured time has arrived
// in the schedule's timezone and that has not yet fired today.
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
		if !slot.Enabled || slot.Time != hhmm {
			continue
		}
		if !markSaleCardSlotFired(slot.Group, day) {
			continue
		}
		runSaleCardSlot(slot)
	}
}

// markSaleCardSlotFired records that a slot fired on the given day and reports
// whether this call won the race (true = caller should run the slot).
func markSaleCardSlotFired(group, day string) bool {
	saleCardFiredGuard.Lock()
	defer saleCardFiredGuard.Unlock()
	if saleCardFiredGuard.day[group] == day {
		return false
	}
	saleCardFiredGuard.day[group] = day
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
	if tokenSvc == nil {
		fmt.Printf("[sale-card] slot %s skipped: 次数 fufu 未配置\n", slot.Group)
		return
	}
	templates := saleCardPlanTemplates()
	for _, job := range slot.Jobs {
		if !job.Enabled || job.TargetStock <= 0 {
			continue
		}
		plan, ok := templates[job.Plan]
		if !ok {
			continue
		}
		plan.TargetStock = job.TargetStock
		plan.Count = 0
		ctx, cancel := context.WithTimeout(context.Background(), saleCardRestockTimeout)
		result, err := generateAndUploadSaleCards(ctx, tokenSvc, plan)
		cancel()
		if err != nil {
			fmt.Printf("[sale-card] restock %s failed: %v\n", job.Plan, err)
			continue
		}
		fmt.Printf("[sale-card] restock %s: current=%d target=%d uploaded=%d\n", job.Plan, result.CurrentStock, result.TargetStock, result.Uploaded)
	}
}
