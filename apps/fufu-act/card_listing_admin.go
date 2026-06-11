package main

import (
	"encoding/json"
	"errors"
	"fufu/auth"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var saleCardScheduleTimeRE = regexp.MustCompile(`^\d{2}:\d{2}$`)

type saleCardRunRequest struct {
	Plan          string  `json:"plan"`
	Count         int     `json:"count"`
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

type SaleCardScheduleConfig struct {
	Enabled  bool                  `json:"enabled"`
	Time     string                `json:"time"`
	Timezone string                `json:"timezone"`
	Jobs     []SaleCardScheduleJob `json:"jobs"`
}

type SaleCardScheduleJob struct {
	Plan    string `json:"plan"`
	Count   int    `json:"count"`
	Enabled bool   `json:"enabled"`
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
	return SaleCardScheduleConfig{
		Enabled:  false,
		Time:     "09:00",
		Timezone: "Asia/Shanghai",
		Jobs:     []SaleCardScheduleJob{},
	}
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
	if strings.TrimSpace(schedule.Time) == "" {
		schedule.Time = defaultSchedule.Time
	}
	if strings.TrimSpace(schedule.Timezone) == "" {
		schedule.Timezone = defaultSchedule.Timezone
	}
	schedule.Time = strings.TrimSpace(schedule.Time)
	schedule.Timezone = strings.TrimSpace(schedule.Timezone)
	if !validSaleCardScheduleTime(schedule.Time) {
		return SaleCardScheduleConfig{}, errors.New("上架时间格式错误")
	}
	if len([]rune(schedule.Timezone)) > 64 {
		return SaleCardScheduleConfig{}, errors.New("时区格式错误")
	}
	templates := saleCardPlanTemplates()
	seen := map[string]bool{}
	jobs := make([]SaleCardScheduleJob, 0, len(schedule.Jobs))
	for _, job := range schedule.Jobs {
		job.Plan = strings.TrimSpace(job.Plan)
		if _, ok := templates[job.Plan]; !ok {
			return SaleCardScheduleConfig{}, errors.New("未知上架计划")
		}
		if seen[job.Plan] {
			return SaleCardScheduleConfig{}, errors.New("上架计划重复")
		}
		seen[job.Plan] = true
		if job.Count < 1 || job.Count > 100 {
			return SaleCardScheduleConfig{}, errors.New("上架数量必须在 1 到 100 之间")
		}
		jobs = append(jobs, job)
	}
	schedule.Jobs = jobs
	return schedule, nil
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
