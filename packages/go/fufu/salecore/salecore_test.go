package salecore

import (
	"errors"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestValidatePlanAcceptsCountAndRestockModes(t *testing.T) {
	base := SaleCardPlan{Count: 2, Quota: 55, Group: "mix", IntervalUnit: 9}
	if err := ValidatePlan(base); err != nil {
		t.Fatalf("count mode: %v", err)
	}
	base.Count = 0
	base.TargetStock = 500
	if err := ValidatePlan(base); err != nil {
		t.Fatalf("restock mode: %v", err)
	}
}

func TestValidatePlanRejectsInvalidFields(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*SaleCardPlan)
	}{
		{"target over 2000", func(p *SaleCardPlan) { p.TargetStock = 2001 }},
		{"blank group", func(p *SaleCardPlan) { p.Group = "" }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			base := SaleCardPlan{Count: 2, Quota: 55, Group: "mix", IntervalUnit: 9}
			c.mutate(&base)
			err := ValidatePlan(base)
			if !errors.Is(err, ErrInvalidPlan) {
				t.Fatalf("err=%v, want ErrInvalidPlan", err)
			}
		})
	}
}

func TestBuildSaleTokenNameUsesCallerProvidedSlugUnderLengthLimit(t *testing.T) {
	now := time.Date(2026, 6, 16, 12, 34, 56, 0, time.FixedZone("CST", 8*60*60))
	cases := []struct {
		plan SaleCardPlan
		want string
	}{
		{SaleCardPlan{TokenNameSlug: "fms55"}, "fms55-20260616-043456"},
		{SaleCardPlan{TokenNameSlug: "fmm100"}, "fmm100-20260616-043456"},
		{SaleCardPlan{TokenNameSlug: "fmm1000"}, "fmm1000-20260616-043456"},
	}
	for _, c := range cases {
		t.Run(c.want, func(t *testing.T) {
			got := BuildSaleTokenName(c.plan, now)
			if got != c.want {
				t.Fatalf("name=%q, want %q", got, c.want)
			}
			if utf8.RuneCountInString(got) > MaxSaleTokenNameRunes {
				t.Fatalf("name=%q has %d runes, want <= %d", got, utf8.RuneCountInString(got), MaxSaleTokenNameRunes)
			}
		})
	}
}

func TestBuildSaleTokenBatchNameKeepsSequenceUnderLengthLimit(t *testing.T) {
	now := time.Date(2026, 6, 16, 12, 34, 56, 0, time.UTC)

	got := BuildSaleTokenBatchName(SaleCardPlan{TokenNameSlug: "fmm1000"}, now, 0, 12)

	if got != "fmm1000-20260616-123456-01" {
		t.Fatalf("name=%q", got)
	}
	if utf8.RuneCountInString(got) > MaxSaleTokenNameRunes {
		t.Fatalf("name=%q has %d runes, want <= %d", got, utf8.RuneCountInString(got), MaxSaleTokenNameRunes)
	}
}

func TestBuildSaleTokenNameTruncatesCustomSlugButKeepsTimestamp(t *testing.T) {
	now := time.Date(2026, 6, 16, 12, 34, 56, 0, time.UTC)
	got := BuildSaleTokenName(SaleCardPlan{TokenNameSlug: "very-long-custom-sale-card-name"}, now)

	if got != "very-long-cust-20260616-123456" {
		t.Fatalf("name=%q", got)
	}
	if utf8.RuneCountInString(got) > MaxSaleTokenNameRunes {
		t.Fatalf("name=%q has %d runes, want <= %d", got, utf8.RuneCountInString(got), MaxSaleTokenNameRunes)
	}
}

func TestSlug(t *testing.T) {
	if got := SaleCardSlugPrefix(SaleCardPlan{TokenNameSlug: " FuFu 月卡! "}); got != "fufu-" {
		t.Fatalf("slug = %q", got)
	}
}

func TestScheduleTimeAndDueRules(t *testing.T) {
	for _, value := range []string{"00:00", "09:30", "23:59"} {
		if !ValidScheduleTime(value) {
			t.Fatalf("%s should be a valid schedule time", value)
		}
	}
	for _, value := range []string{"", "9:30", "24:00", "23:60", "99:99"} {
		if ValidScheduleTime(value) {
			t.Fatalf("%s should be invalid schedule time", value)
		}
	}
	if SlotDue("09:00", "09:30") || !SlotDue("09:00", "09:00") {
		t.Fatal("slot should be due only during the configured minute")
	}
	if SlotDue("09:30", "09:00") || SlotDue("bad", "09:00") {
		t.Fatal("slot should not be due before time or with invalid input")
	}
}

func TestSlotFireKeyIncludesRunnableJobShape(t *testing.T) {
	got := SlotFireKey(ScheduleSlot{
		Group: "special55",
		Time:  "20:04",
		Jobs: []ScheduleJob{
			{Plan: "fufu-mix-special-55", TargetStock: 2, Enabled: true},
			{Plan: "fufu-mix-month-100", TargetStock: 0, Enabled: false},
		},
	})

	want := "special55|20:04|fufu-mix-special-55:2:true|fufu-mix-month-100:0:false"
	if got != want {
		t.Fatalf("SlotFireKey()=%q, want %q", got, want)
	}
}

func TestNormalizeScheduleFillsConfiguredSlotsAndJobs(t *testing.T) {
	catalog := ScheduleCatalog{
		Slots: []ScheduleSlotDefinition{
			{Group: "special55", Label: "55 次混合特惠卡", Time: "09:00"},
			{Group: "month", Label: "月次卡", Time: "09:30"},
		},
		Plans: []SchedulePlan{
			{ID: "fufu-mix-special-55", Slot: "special55"},
			{ID: "fufu-mix-month-100", Slot: "month"},
		},
	}

	got, err := NormalizeSchedule(ScheduleConfig{
		Enabled: true,
		Slots: []ScheduleSlot{
			{Group: "special55", Time: "20:04", Enabled: true, Jobs: []ScheduleJob{
				{Plan: " fufu-mix-special-55 ", TargetStock: 2, Enabled: true},
			}},
		},
	}, catalog)
	if err != nil {
		t.Fatalf("NormalizeSchedule() error = %v", err)
	}
	if got.Timezone != "Asia/Shanghai" || !got.Enabled || len(got.Slots) != 2 {
		t.Fatalf("normalized schedule = %#v", got)
	}
	if got.Slots[0].Group != "special55" || got.Slots[0].Label != "55 次混合特惠卡" || got.Slots[0].Time != "20:04" || !got.Slots[0].Enabled {
		t.Fatalf("special55 slot = %#v", got.Slots[0])
	}
	if got.Slots[0].Jobs[0].Plan != "fufu-mix-special-55" || got.Slots[0].Jobs[0].TargetStock != 2 || !got.Slots[0].Jobs[0].Enabled {
		t.Fatalf("special55 jobs = %#v", got.Slots[0].Jobs)
	}
	if len(got.Slots[1].Jobs) != 1 || got.Slots[1].Jobs[0].Plan != "fufu-mix-month-100" {
		t.Fatalf("missing default month jobs: %#v", got.Slots[1].Jobs)
	}
}

func TestDefaultSaleCardPlanTemplatesEncodeBusinessTiers(t *testing.T) {
	templates := DefaultSaleCardPlanTemplates()
	special := templates["fufu-mix-special-55"]
	if special.Quota != 55 || special.IntervalUnit != 3 || special.ItemID != 29 || special.SKUID != 66 || special.Slot != DefaultSaleCardSlotSpecial55 {
		t.Fatalf("special plan = %#v", special)
	}
	if special.TokenNameSlug != "fms55" || !special.Unique {
		t.Fatalf("special token metadata = %#v", special)
	}
	month := templates["fufu-mix-month-100"]
	if month.Quota != 100 || month.IntervalUnit != 9 || month.ItemID != 28 || month.SKUID != 65 || month.Slot != DefaultSaleCardSlotMonth {
		t.Fatalf("month plan = %#v", month)
	}
	if SaleCardPlanSlot(templates, " fufu-mix-month-1000 ") != DefaultSaleCardSlotMonth {
		t.Fatalf("month plan slot lookup failed")
	}
	if SaleCardPlanSlot(templates, "missing") != "" {
		t.Fatalf("missing plan should not map to a slot")
	}
}

func TestDefaultSaleCardScheduleBuildsJobsFromCatalog(t *testing.T) {
	templates := DefaultSaleCardPlanTemplates()
	plans := make([]SchedulePlan, 0, len(templates))
	for id, plan := range templates {
		plans = append(plans, SchedulePlan{ID: id, Slot: plan.Slot})
	}
	schedule := DefaultSaleCardSchedule(ScheduleCatalog{
		Slots: DefaultSaleCardSlotDefinitions(),
		Plans: plans,
	})
	if schedule.Enabled || schedule.Timezone != "Asia/Shanghai" || len(schedule.Slots) != 2 {
		t.Fatalf("default schedule = %#v", schedule)
	}
	if schedule.Slots[0].Group != DefaultSaleCardSlotSpecial55 || schedule.Slots[0].Time != "09:00" || len(schedule.Slots[0].Jobs) != 1 {
		t.Fatalf("special slot = %#v", schedule.Slots[0])
	}
	if schedule.Slots[0].Jobs[0].Plan != "fufu-mix-special-55" || schedule.Slots[0].Jobs[0].Enabled {
		t.Fatalf("special jobs = %#v", schedule.Slots[0].Jobs)
	}
	if schedule.Slots[1].Group != DefaultSaleCardSlotMonth || schedule.Slots[1].Time != "09:30" || len(schedule.Slots[1].Jobs) != 5 {
		t.Fatalf("month slot = %#v", schedule.Slots[1])
	}
}

func TestNormalizeScheduleRejectsInvalidJobs(t *testing.T) {
	catalog := ScheduleCatalog{
		Slots: []ScheduleSlotDefinition{{Group: "special55", Label: "55", Time: "09:00"}},
		Plans: []SchedulePlan{{ID: "fufu-mix-special-55", Slot: "special55"}, {ID: "fufu-mix-month-100", Slot: "month"}},
	}
	cases := []struct {
		name string
		jobs []ScheduleJob
		want string
	}{
		{"unknown", []ScheduleJob{{Plan: "ghost"}}, "未知上架计划"},
		{"wrong slot", []ScheduleJob{{Plan: "fufu-mix-month-100"}}, "上架计划与时段不匹配"},
		{"duplicate", []ScheduleJob{{Plan: "fufu-mix-special-55"}, {Plan: "fufu-mix-special-55"}}, "上架计划重复"},
		{"target too high", []ScheduleJob{{Plan: "fufu-mix-special-55", TargetStock: 2001}}, "补卡目标库存"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := NormalizeSchedule(ScheduleConfig{Slots: []ScheduleSlot{{Group: "special55", Jobs: c.jobs}}}, catalog)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err=%v, want substring %q", err, c.want)
			}
		})
	}
}
