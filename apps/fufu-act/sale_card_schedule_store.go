package activityapp

import (
	"encoding/json"
	"errors"
	"fufu/salecore"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type SaleCardScheduleConfig = salecore.ScheduleConfig
type SaleCardScheduleSlot = salecore.ScheduleSlot
type SaleCardScheduleJob = salecore.ScheduleJob
type saleCardScheduleSlotDefinition = salecore.ScheduleSlotDefinition

func loadSaleCardSchedule() (SaleCardScheduleConfig, error) {
	data, err := os.ReadFile(saleCardSchedulePath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return defaultSaleCardSchedule(), nil
		}
		return SaleCardScheduleConfig{}, err
	}
	var schedule SaleCardScheduleConfig
	if err := json.Unmarshal(data, &schedule); err != nil {
		return SaleCardScheduleConfig{}, err
	}
	return normalizeSaleCardSchedule(schedule)
}

func saveSaleCardSchedule(schedule SaleCardScheduleConfig) error {
	path := saleCardSchedulePath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(schedule, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0600)
}

func saleCardSchedulePath() string {
	base := rootDir
	if strings.TrimSpace(base) == "" {
		base = "."
	}
	return filepath.Join(base, "data", "sale-card-schedule.json")
}

func normalizeSaleCardSchedule(schedule SaleCardScheduleConfig) (SaleCardScheduleConfig, error) {
	var err error
	schedule, err = salecore.NormalizeSchedule(schedule, saleCardScheduleCatalog())
	if err != nil {
		return SaleCardScheduleConfig{}, err
	}
	if _, err := time.LoadLocation(schedule.Timezone); err != nil {
		return SaleCardScheduleConfig{}, errors.New("时区不存在或格式错误")
	}
	return schedule, nil
}

func saleCardScheduleCatalog() salecore.ScheduleCatalog {
	templates := saleCardPlanTemplates()
	plans := make([]salecore.SchedulePlan, 0, len(templates))
	for id, plan := range templates {
		plans = append(plans, salecore.SchedulePlan{ID: id, Slot: strings.TrimSpace(plan.Slot)})
	}
	return salecore.ScheduleCatalog{Slots: saleCardSlotDefs, Plans: plans}
}

func normalizeSlotJobs(group string, raw []SaleCardScheduleJob) ([]SaleCardScheduleJob, error) {
	return salecore.NormalizeScheduleJobs(group, raw, saleCardScheduleCatalog().Plans)
}

func validSaleCardScheduleTime(value string) bool {
	return salecore.ValidScheduleTime(value)
}
