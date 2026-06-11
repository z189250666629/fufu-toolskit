package main

import (
	"errors"
	"fufu/auth"
	"net/http"
	"os"
	"strings"
)

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
