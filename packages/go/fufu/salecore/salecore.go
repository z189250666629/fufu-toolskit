package salecore

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var ErrInvalidPlan = errors.New("sale card plan invalid")

const MaxSaleTokenNameRunes = 30

type SaleCardPlan struct {
	ID            string  `json:"id,omitempty"`
	Name          string  `json:"name,omitempty"`
	Count         int     `json:"count"`
	TargetStock   int     `json:"targetStock,omitempty"`
	Quota         float64 `json:"quota"`
	Group         string  `json:"group,omitempty"`
	Slot          string  `json:"slot,omitempty"`
	IntervalUnit  int     `json:"intervalUnit"`
	TokenNameSlug string  `json:"tokenNameSlug,omitempty"`
}

type ScheduleConfig struct {
	Enabled  bool           `json:"enabled"`
	Timezone string         `json:"timezone"`
	Slots    []ScheduleSlot `json:"slots"`
}

type ScheduleSlot struct {
	Group   string        `json:"group"`
	Label   string        `json:"label"`
	Time    string        `json:"time"`
	Enabled bool          `json:"enabled"`
	Jobs    []ScheduleJob `json:"jobs"`
}

type ScheduleJob struct {
	Plan        string `json:"plan"`
	TargetStock int    `json:"targetStock"`
	Enabled     bool   `json:"enabled"`
}

type ScheduleSlotDefinition struct {
	Group string
	Label string
	Time  string
}

type SchedulePlan struct {
	ID   string
	Slot string
}

type ScheduleCatalog struct {
	Slots []ScheduleSlotDefinition
	Plans []SchedulePlan
}

func NormalizePlan(plan SaleCardPlan) SaleCardPlan {
	plan.ID = strings.TrimSpace(plan.ID)
	plan.Name = strings.TrimSpace(plan.Name)
	plan.Group = strings.TrimSpace(plan.Group)
	plan.TokenNameSlug = strings.TrimSpace(plan.TokenNameSlug)
	return plan
}

func ValidatePlan(plan SaleCardPlan) error {
	switch {
	case plan.TargetStock < 0:
		return fmt.Errorf("%w: target stock cannot be negative", ErrInvalidPlan)
	case plan.TargetStock > 2000:
		return fmt.Errorf("%w: target stock must be 2000 or fewer", ErrInvalidPlan)
	case plan.TargetStock == 0 && (plan.Count < 1 || plan.Count > 100):
		return fmt.Errorf("%w: count must be between 1 and 100", ErrInvalidPlan)
	case plan.Quota <= 0:
		return fmt.Errorf("%w: quota must be positive", ErrInvalidPlan)
	case strings.TrimSpace(plan.Group) == "":
		return fmt.Errorf("%w: group is required", ErrInvalidPlan)
	case plan.IntervalUnit == 0:
		return fmt.Errorf("%w: interval unit is required", ErrInvalidPlan)
	}
	return nil
}

func BuildSaleTokenName(plan SaleCardPlan, now time.Time) string {
	plan = NormalizePlan(plan)
	stamp := now.UTC().Format("20060102-150405")
	prefix := compactSaleTokenSlug(plan)
	maxPrefixRunes := MaxSaleTokenNameRunes - len([]rune(stamp)) - 1
	prefix = truncateSaleTokenSlug(prefix, maxPrefixRunes)
	if prefix == "" {
		prefix = "sale"
	}
	return prefix + "-" + stamp
}

func BuildSaleTokenBatchName(plan SaleCardPlan, now time.Time, index, total int) string {
	if total <= 1 {
		return BuildSaleTokenName(plan, now)
	}
	if index < 0 {
		index = 0
	}
	seqWidth := len(strconv.Itoa(total))
	if seqWidth < 2 {
		seqWidth = 2
	}
	seq := fmt.Sprintf("%0*d", seqWidth, index+1)
	stamp := now.UTC().Format("20060102-150405")
	prefix := compactSaleTokenSlug(NormalizePlan(plan))
	maxPrefixRunes := MaxSaleTokenNameRunes - len([]rune(stamp)) - len([]rune(seq)) - 2
	prefix = truncateSaleTokenSlug(prefix, maxPrefixRunes)
	if prefix == "" {
		prefix = "sale"
	}
	return prefix + "-" + stamp + "-" + seq
}

func compactSaleTokenSlug(plan SaleCardPlan) string {
	slug := SanitizeSaleCardSlug(plan.TokenNameSlug)
	if slug == "" {
		return "token"
	}
	return slug
}

func truncateSaleTokenSlug(slug string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	slug = strings.Trim(strings.TrimSpace(slug), "-")
	runes := []rune(slug)
	if len(runes) > maxRunes {
		slug = string(runes[:maxRunes])
	}
	return strings.Trim(slug, "-")
}

func SaleCardSlugPrefix(plan SaleCardPlan) string {
	slug := SanitizeSaleCardSlug(plan.TokenNameSlug)
	if slug == "" {
		slug = "token"
	}
	return slug + "-"
}

func SanitizeSaleCardSlug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if ok {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func BoolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func ValidScheduleTime(value string) bool {
	if len(value) != 5 || value[2] != ':' {
		return false
	}
	for _, idx := range []int{0, 1, 3, 4} {
		if value[idx] < '0' || value[idx] > '9' {
			return false
		}
	}
	hour := int(value[0]-'0')*10 + int(value[1]-'0')
	minute := int(value[3]-'0')*10 + int(value[4]-'0')
	return hour >= 0 && hour <= 23 && minute >= 0 && minute <= 59
}

func SlotDue(slotTime, currentHHMM string) bool {
	return ValidScheduleTime(slotTime) && ValidScheduleTime(currentHHMM) && slotTime <= currentHHMM
}

func SlotFireKey(slot ScheduleSlot) string {
	key := fmt.Sprintf("%s|%s", slot.Group, slot.Time)
	for _, job := range slot.Jobs {
		key += fmt.Sprintf("|%s:%d:%t", job.Plan, job.TargetStock, job.Enabled)
	}
	return key
}

func NormalizeSchedule(schedule ScheduleConfig, catalog ScheduleCatalog) (ScheduleConfig, error) {
	if strings.TrimSpace(schedule.Timezone) == "" {
		schedule.Timezone = "Asia/Shanghai"
	}
	schedule.Timezone = strings.TrimSpace(schedule.Timezone)
	if len([]rune(schedule.Timezone)) > 64 {
		return ScheduleConfig{}, errors.New("时区格式错误")
	}

	submitted := map[string]ScheduleSlot{}
	for _, slot := range schedule.Slots {
		submitted[strings.TrimSpace(slot.Group)] = slot
	}

	slots := make([]ScheduleSlot, 0, len(catalog.Slots))
	for _, def := range catalog.Slots {
		def.Group = strings.TrimSpace(def.Group)
		def.Label = strings.TrimSpace(def.Label)
		def.Time = strings.TrimSpace(def.Time)
		if def.Group == "" {
			continue
		}
		slot := ScheduleSlot{Group: def.Group, Label: def.Label, Time: def.Time}
		if provided, ok := submitted[def.Group]; ok {
			slot.Enabled = provided.Enabled
			if t := strings.TrimSpace(provided.Time); t != "" {
				slot.Time = t
			}
			jobs, err := normalizeScheduleJobs(def.Group, provided.Jobs, catalog.Plans)
			if err != nil {
				return ScheduleConfig{}, err
			}
			slot.Jobs = jobs
		} else {
			slot.Jobs = defaultScheduleJobs(def.Group, catalog.Plans)
		}
		if !ValidScheduleTime(slot.Time) {
			return ScheduleConfig{}, errors.New("补卡时间格式错误")
		}
		slots = append(slots, slot)
	}
	schedule.Slots = slots
	return schedule, nil
}

func normalizeScheduleJobs(group string, raw []ScheduleJob, plans []SchedulePlan) ([]ScheduleJob, error) {
	planSlots := schedulePlanSlots(plans)
	seen := map[string]bool{}
	jobs := make([]ScheduleJob, 0, len(raw))
	for _, job := range raw {
		job.Plan = strings.TrimSpace(job.Plan)
		slot, ok := planSlots[job.Plan]
		if !ok {
			return nil, errors.New("未知上架计划")
		}
		if slot != group {
			return nil, errors.New("上架计划与时段不匹配")
		}
		if seen[job.Plan] {
			return nil, errors.New("上架计划重复")
		}
		seen[job.Plan] = true
		if job.TargetStock < 0 || job.TargetStock > 2000 {
			return nil, errors.New("补卡目标库存必须在 0 到 2000 之间")
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}

func NormalizeScheduleJobs(group string, raw []ScheduleJob, plans []SchedulePlan) ([]ScheduleJob, error) {
	return normalizeScheduleJobs(strings.TrimSpace(group), raw, plans)
}

func defaultScheduleJobs(group string, plans []SchedulePlan) []ScheduleJob {
	jobs := []ScheduleJob{}
	for _, plan := range plans {
		if strings.TrimSpace(plan.Slot) == group {
			jobs = append(jobs, ScheduleJob{Plan: strings.TrimSpace(plan.ID), TargetStock: 0, Enabled: false})
		}
	}
	return jobs
}

func schedulePlanSlots(plans []SchedulePlan) map[string]string {
	out := map[string]string{}
	for _, plan := range plans {
		id := strings.TrimSpace(plan.ID)
		if id == "" {
			continue
		}
		out[id] = strings.TrimSpace(plan.Slot)
	}
	return out
}
