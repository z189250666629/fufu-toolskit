package activityapp

import (
	"context"
	"encoding/json"
	"errors"
	"fufu/auth"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

var saleCardScheduleTimeRE = regexp.MustCompile(`^\d{2}:\d{2}$`)

type saleCardRunRequest struct {
	Plan          string  `json:"plan"`
	Count         int     `json:"count"`
	TargetStock   int     `json:"targetStock"`
	Name          string  `json:"name"`
	Quota         float64 `json:"quota"`
	Group         string  `json:"group"`
	IntervalUnit  int     `json:"intervalUnit"`
	ItemID        int     `json:"itemId"`
	SKUID         int     `json:"skuId"`
	Remark        string  `json:"remark"`
	Unique        *bool   `json:"unique"`
	TokenNameSlug string  `json:"tokenNameSlug"`
}

type saleCardAdminConfigResponse struct {
	Plans    []SaleCardPlan         `json:"plans"`
	Schedule SaleCardScheduleConfig `json:"schedule"`
}

type saleCardAdminConfigRequest struct {
	Schedule SaleCardScheduleConfig `json:"schedule"`
}

// SaleCardScheduleConfig drives 自动补卡. 月次卡与 55 次混合特惠卡各占一个独立
// 时段（slot），每个时段有自己的触发时间和补卡目标库存。
type SaleCardScheduleConfig struct {
	Enabled  bool                   `json:"enabled"`
	Timezone string                 `json:"timezone"`
	Slots    []SaleCardScheduleSlot `json:"slots"`
}

type SaleCardScheduleSlot struct {
	Group   string                `json:"group"` // saleCardSlotSpecial55 | saleCardSlotMonth
	Label   string                `json:"label"`
	Time    string                `json:"time"` // HH:MM
	Enabled bool                  `json:"enabled"`
	Jobs    []SaleCardScheduleJob `json:"jobs"`
}

type SaleCardScheduleJob struct {
	Plan        string `json:"plan"`
	TargetStock int    `json:"targetStock"`
	Enabled     bool   `json:"enabled"`
}

// saleCardSlotDefs lists the canonical 补卡时段 in display order.
var saleCardSlotDefs = []struct {
	Group string
	Label string
	Time  string
}{
	{Group: saleCardSlotSpecial55, Label: "55 次混合特惠卡", Time: "09:00"},
	{Group: saleCardSlotMonth, Label: "月次卡", Time: "09:30"},
}

func handleAdminSaleCardsRun(w http.ResponseWriter, r *http.Request) {
	if !auth.CheckAdminToken(adminBearerToken(r), os.Getenv("ADMIN_TOKEN"), "") {
		writeJSONError(w, http.StatusUnauthorized, "未授权")
		return
	}
	if tokenConfigErr != nil || tokenSvc == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "次数 fufu 未配置")
		return
	}
	var req saleCardRunRequest
	if err := readBody(r, &req); err != nil {
		if errors.Is(err, errRequestBodyTooLarge) {
			writeJSONError(w, http.StatusRequestEntityTooLarge, "请求体过大")
			return
		}
		writeJSONError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	plan, err := saleCardPlanFromRunRequest(req)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := generateAndUploadSaleCards(r.Context(), tokenSvc, plan)
	if err != nil {
		writeSaleCardRunError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func handleAdminSaleCardsConfig(w http.ResponseWriter, r *http.Request) {
	if !auth.CheckAdminToken(adminBearerToken(r), os.Getenv("ADMIN_TOKEN"), "") {
		writeJSONError(w, http.StatusUnauthorized, "未授权")
		return
	}
	switch r.Method {
	case http.MethodGet:
		schedule, err := loadSaleCardSchedule()
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "读取上架配置失败")
			return
		}
		writeJSON(w, http.StatusOK, saleCardAdminConfigResponse{Plans: saleCardPlanList(), Schedule: schedule})
	case http.MethodPost:
		var req saleCardAdminConfigRequest
		if err := readBody(r, &req); err != nil {
			if errors.Is(err, errRequestBodyTooLarge) {
				writeJSONError(w, http.StatusRequestEntityTooLarge, "请求体过大")
				return
			}
			writeJSONError(w, http.StatusBadRequest, "请求格式错误")
			return
		}
		schedule, err := normalizeSaleCardSchedule(req.Schedule)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := saveSaleCardSchedule(schedule); err != nil {
			writeJSONError(w, http.StatusInternalServerError, "保存上架配置失败")
			return
		}
		writeJSON(w, http.StatusOK, saleCardAdminConfigResponse{Plans: saleCardPlanList(), Schedule: schedule})
	default:
		w.Header().Set("Allow", "GET, POST")
		writeJSONError(w, http.StatusMethodNotAllowed, "Only GET, POST")
	}
}

type saleCardStockEntry struct {
	PlanID       string `json:"planId"`
	PlanName     string `json:"planName"`
	Slot         string `json:"slot"`
	CurrentStock int    `json:"currentStock"`
}

// saleCardStockTimeout bounds the whole multi-plan stock lookup so a slow or
// unreachable NewAPI can't hang the admin refresh.
var saleCardStockTimeout = 15 * time.Second

// handleAdminSaleCardsStock reports the current NewAPI stock for every plan so
// the admin can see live counts before deciding restock targets. Each plan is a
// separate name query; they run concurrently so the refresh takes one round-trip,
// not one per plan.
func handleAdminSaleCardsStock(w http.ResponseWriter, r *http.Request) {
	if !auth.CheckAdminToken(adminBearerToken(r), os.Getenv("ADMIN_TOKEN"), "") {
		writeJSONError(w, http.StatusUnauthorized, "未授权")
		return
	}
	plans := saleCardPlanList()
	out := make([]saleCardStockEntry, len(plans))
	ctx, cancel := context.WithTimeout(r.Context(), saleCardStockTimeout)
	defer cancel()
	errCh := make(chan error, len(plans))
	var wg sync.WaitGroup
	for i := range plans {
		plan := plans[i]
		out[i] = saleCardStockEntry{PlanID: plan.ID, PlanName: plan.Name, Slot: plan.Slot}
		wg.Add(1)
		go func(idx int, plan SaleCardPlan) {
			defer wg.Done()
			current, err := queryMCYUsableStock(ctx, plan.ItemID, plan.SKUID)
			if err != nil {
				errCh <- err
				cancel() // fail fast: abort the other in-flight queries
				return
			}
			out[idx].CurrentStock = current
		}(i, plan)
	}
	wg.Wait()
	close(errCh)
	if err := <-errCh; err != nil {
		writeJSONError(w, http.StatusBadGateway, "查询库存失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"stock": out})
}

func saleCardPlanFromRunRequest(req saleCardRunRequest) (SaleCardPlan, error) {
	plan := SaleCardPlan{}
	planID := strings.TrimSpace(req.Plan)
	if planID != "" {
		template, ok := saleCardPlanTemplates()[planID]
		if !ok {
			return SaleCardPlan{}, errors.New("未知上架计划")
		}
		plan = template
	}
	if req.Name != "" {
		plan.Name = req.Name
	}
	if req.Count > 0 {
		plan.Count = req.Count
	}
	if req.TargetStock > 0 {
		plan.TargetStock = req.TargetStock
	}
	if req.Quota > 0 {
		plan.Quota = req.Quota
	}
	if req.Group != "" {
		plan.Group = req.Group
	}
	if req.IntervalUnit != 0 {
		plan.IntervalUnit = req.IntervalUnit
	}
	if req.ItemID > 0 {
		plan.ItemID = req.ItemID
	}
	if req.SKUID > 0 {
		plan.SKUID = req.SKUID
	}
	if req.Remark != "" {
		plan.Remark = req.Remark
	}
	if req.Unique != nil {
		plan.Unique = *req.Unique
	}
	if req.TokenNameSlug != "" {
		plan.TokenNameSlug = req.TokenNameSlug
	}
	if plan.ID == "" {
		plan.ID = planID
	}
	plan = normalizeSaleCardPlan(plan)
	if err := validateSaleCardPlan(plan); err != nil {
		return SaleCardPlan{}, err
	}
	return plan, nil
}

func saleCardPlanList() []SaleCardPlan {
	templates := saleCardPlanTemplates()
	keys := make([]string, 0, len(templates))
	for key := range templates {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	plans := make([]SaleCardPlan, 0, len(keys))
	for _, key := range keys {
		plan := templates[key]
		plan.Count = 0
		plans = append(plans, plan)
	}
	return plans
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

// defaultSlotJobs lists every plan that belongs to a slot, disabled with no
// target — a ready-to-fill row per card type.
func defaultSlotJobs(group string) []SaleCardScheduleJob {
	jobs := []SaleCardScheduleJob{}
	for _, plan := range saleCardPlanList() {
		if saleCardPlanSlot(plan.ID) == group {
			jobs = append(jobs, SaleCardScheduleJob{Plan: plan.ID, TargetStock: 0, Enabled: false})
		}
	}
	return jobs
}

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
	defaultSchedule := defaultSaleCardSchedule()
	if strings.TrimSpace(schedule.Timezone) == "" {
		schedule.Timezone = defaultSchedule.Timezone
	}
	schedule.Timezone = strings.TrimSpace(schedule.Timezone)
	if len([]rune(schedule.Timezone)) > 64 {
		return SaleCardScheduleConfig{}, errors.New("时区格式错误")
	}
	// Reject a timezone the scheduler can't load, so a typo (e.g. "Asia/Shangha")
	// fails at save time instead of silently firing in UTC at runtime.
	if _, err := time.LoadLocation(schedule.Timezone); err != nil {
		return SaleCardScheduleConfig{}, errors.New("时区不存在或格式错误")
	}

	// Index the submitted slots by group so we can rebuild the canonical two-slot
	// layout regardless of order or missing entries.
	submitted := map[string]SaleCardScheduleSlot{}
	for _, slot := range schedule.Slots {
		submitted[strings.TrimSpace(slot.Group)] = slot
	}

	slots := make([]SaleCardScheduleSlot, 0, len(saleCardSlotDefs))
	for _, def := range saleCardSlotDefs {
		slot := SaleCardScheduleSlot{Group: def.Group, Label: def.Label, Time: def.Time}
		if provided, ok := submitted[def.Group]; ok {
			slot.Enabled = provided.Enabled
			if t := strings.TrimSpace(provided.Time); t != "" {
				slot.Time = t
			}
			jobs, err := normalizeSlotJobs(def.Group, provided.Jobs)
			if err != nil {
				return SaleCardScheduleConfig{}, err
			}
			slot.Jobs = jobs
		} else {
			slot.Jobs = defaultSlotJobs(def.Group)
		}
		if !validSaleCardScheduleTime(slot.Time) {
			return SaleCardScheduleConfig{}, errors.New("补卡时间格式错误")
		}
		slots = append(slots, slot)
	}
	schedule.Slots = slots
	return schedule, nil
}

// normalizeSlotJobs validates a slot's per-plan jobs: each plan must belong to
// the slot's group, with no duplicates and a target stock within [0, 2000].
func normalizeSlotJobs(group string, raw []SaleCardScheduleJob) ([]SaleCardScheduleJob, error) {
	templates := saleCardPlanTemplates()
	seen := map[string]bool{}
	jobs := make([]SaleCardScheduleJob, 0, len(raw))
	for _, job := range raw {
		job.Plan = strings.TrimSpace(job.Plan)
		if _, ok := templates[job.Plan]; !ok {
			return nil, errors.New("未知上架计划")
		}
		if saleCardPlanSlot(job.Plan) != group {
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

func validSaleCardScheduleTime(value string) bool {
	if !saleCardScheduleTimeRE.MatchString(value) {
		return false
	}
	hour := int(value[0]-'0')*10 + int(value[1]-'0')
	minute := int(value[3]-'0')*10 + int(value[4]-'0')
	return hour >= 0 && hour <= 23 && minute >= 0 && minute <= 59
}

func writeSaleCardRunError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrSaleCardInvalidPlan):
		writeJSONError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, ErrShopLoginFailed), errors.Is(err, ErrShopRequestFailed), errors.Is(err, ErrShopInvalidResponse):
		writeJSONError(w, http.StatusBadGateway, "MCY 上架失败")
	case errors.Is(err, ErrSaleCardGenerationFailed):
		writeJSONError(w, http.StatusBadGateway, "次数 fufu 生成卡密失败")
	default:
		writeJSONError(w, http.StatusInternalServerError, "服务器错误")
	}
}
