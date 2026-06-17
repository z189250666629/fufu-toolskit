package activityapp

import (
	"fmt"
	"strings"
)

const (
	saleCardSlotSpecial55     = "special55"
	saleCardSlotMonth         = "month"
	saleCardDefaultTokenGroup = "mix"
)

var saleCardSlotDefs = []saleCardScheduleSlotDefinition{
	{Group: saleCardSlotSpecial55, Label: "55 次混合特惠卡", Time: "09:00"},
	{Group: saleCardSlotMonth, Label: "月次卡", Time: "09:30"},
}

func saleCardPlanSlot(planID string) string {
	template, ok := saleCardPlanTemplates()[strings.TrimSpace(planID)]
	if !ok {
		return ""
	}
	return strings.TrimSpace(template.Slot)
}

func saleCardPlanTemplates() map[string]SaleCardPlan {
	return map[string]SaleCardPlan{
		"fufu-mix-special-55": {
			ID:            "fufu-mix-special-55",
			Name:          "FuFu 55次混合特惠卡",
			Quota:         55,
			Group:         saleCardDefaultTokenGroup,
			Slot:          saleCardSlotSpecial55,
			IntervalUnit:  3,
			ItemID:        29,
			SKUID:         66,
			Remark:        "FuFu 55次混合特惠卡",
			Unique:        true,
			TokenNameSlug: "fms55",
		},
		"fufu-mix-month-100":  fufuMixMonthPlan("fufu-mix-month-100", "混合卡 月一百次卡", 65, 100),
		"fufu-mix-month-150":  fufuMixMonthPlan("fufu-mix-month-150", "混合卡 月一百五十次卡", 64, 150),
		"fufu-mix-month-300":  fufuMixMonthPlan("fufu-mix-month-300", "混合卡 月三百次卡", 60, 300),
		"fufu-mix-month-500":  fufuMixMonthPlan("fufu-mix-month-500", "混合卡 月五百次卡", 61, 500),
		"fufu-mix-month-1000": fufuMixMonthPlan("fufu-mix-month-1000", "混合卡 月一千次卡", 62, 1000),
	}
}

func fufuMixMonthPlan(id, name string, skuID int, quota float64) SaleCardPlan {
	return SaleCardPlan{
		ID:            id,
		Name:          name,
		Quota:         quota,
		Group:         saleCardDefaultTokenGroup,
		Slot:          saleCardSlotMonth,
		IntervalUnit:  9,
		ItemID:        28,
		SKUID:         skuID,
		Remark:        name,
		Unique:        true,
		TokenNameSlug: "fmm" + saleCardQuotaSlug(quota),
	}
}

func defaultSaleCardSchedule() SaleCardScheduleConfig {
	slots := make([]SaleCardScheduleSlot, 0, len(saleCardSlotDefs))
	for _, def := range saleCardSlotDefs {
		slots = append(slots, SaleCardScheduleSlot{
			Group:   def.Group,
			Label:   def.Label,
			Time:    def.Time,
			Enabled: false,
			Jobs:    defaultSlotJobs(def.Group),
		})
	}
	return SaleCardScheduleConfig{
		Enabled:  false,
		Timezone: "Asia/Shanghai",
		Slots:    slots,
	}
}

func defaultSlotJobs(group string) []SaleCardScheduleJob {
	jobs := []SaleCardScheduleJob{}
	for _, plan := range saleCardPlanList() {
		if saleCardPlanSlot(plan.ID) == group {
			jobs = append(jobs, SaleCardScheduleJob{Plan: plan.ID, TargetStock: 0, Enabled: false})
		}
	}
	return jobs
}

func saleCardQuotaSlug(quota float64) string {
	text := strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", quota), "0"), ".")
	return strings.ReplaceAll(text, ".", "p")
}
