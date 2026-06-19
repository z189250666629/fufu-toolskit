package activityapp

import (
	"context"
	"errors"
	"fmt"
	"fufu/auth"
	"net/http"
	"os"
	"strings"
	"time"
)

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

func handleAdminSaleCardsRun(w http.ResponseWriter, r *http.Request) {
	if !auth.CheckAdminToken(adminBearerToken(r), os.Getenv("ADMIN_TOKEN"), "") {
		writeJSONError(w, http.StatusUnauthorized, "未授权")
		return
	}
	service, configErr := snapshotTokenRuntime()
	if configErr != nil || service == nil {
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
	result, err := generateAndUploadSaleCards(r.Context(), service, plan)
	if err != nil {
		writeSaleCardRunError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func handleAdminSaleCardsTestKey(w http.ResponseWriter, r *http.Request) {
	if !auth.CheckAdminToken(adminBearerToken(r), os.Getenv("ADMIN_TOKEN"), "") {
		writeJSONError(w, http.StatusUnauthorized, "未授权")
		return
	}
	service, configErr := snapshotTokenRuntime()
	if configErr != nil || service == nil {
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
	if req.Count <= 0 {
		req.Count = 1
	}
	req.TargetStock = 0
	plan, err := saleCardPlanFromRunRequest(req)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := generateSaleCardTestKeys(r.Context(), service, plan, req.Count, SnapshotRuntimeConfig())
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

// saleCardStockTimeout bounds the whole stock scan so a slow or unreachable MCY
// shop can't hang the admin refresh.
var saleCardStockTimeout = 30 * time.Second

// handleAdminSaleCardsStock reports the current MCY shop stock for every plan so
// the admin can see live counts before deciding restock targets. Each plan is a
// precise per-SKU card/get (equal-* filter → data.total). The queries run
// SEQUENTIALLY — the shop rejects concurrent requests on one session with
// 登录已过期 — which is fast enough (~one round-trip per plan after login).
func handleAdminSaleCardsStock(w http.ResponseWriter, r *http.Request) {
	if !auth.CheckAdminToken(adminBearerToken(r), os.Getenv("ADMIN_TOKEN"), "") {
		writeJSONError(w, http.StatusUnauthorized, "未授权")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), saleCardStockTimeout)
	defer cancel()
	plans := saleCardPlanList()
	out := make([]saleCardStockEntry, len(plans))
	for i, plan := range plans {
		current, err := queryMCYUsableStock(ctx, plan.ItemID, plan.SKUID)
		if err != nil {
			// Surface the actual MCY reason (admin-only) instead of a generic message
			// so the failure is diagnosable.
			fmt.Printf("[sale-card] stock query failed: %v\n", err)
			writeJSONError(w, http.StatusBadGateway, "查询库存失败: "+saleCardShopErrorMessage(err))
			return
		}
		out[i] = saleCardStockEntry{
			PlanID:       plan.ID,
			PlanName:     plan.Name,
			Slot:         plan.Slot,
			CurrentStock: current,
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"stock": out})
}

func writeSaleCardRunError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrSaleCardInvalidPlan):
		writeJSONError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, ErrShopLoginFailed), errors.Is(err, ErrShopRequestFailed), errors.Is(err, ErrShopInvalidResponse):
		writeJSONError(w, http.StatusBadGateway, "MCY 上架失败: "+saleCardShopErrorMessage(err))
	case errors.Is(err, ErrSaleCardGenerationFailed):
		message := saleCardGenerationFailureMessage(err)
		fmt.Printf("[sale-card] token generation failed: %s\n", message)
		writeJSONError(w, http.StatusBadGateway, message)
	default:
		writeJSONError(w, http.StatusInternalServerError, "服务器错误")
	}
}

func saleCardShopErrorMessage(err error) string {
	switch {
	case errors.Is(err, ErrShopCredentialInvalid):
		return "MCY 登录失败：请检查商城账号或密码"
	case errors.Is(err, ErrShopLoginFailed):
		return "MCY 登录失败，请检查商城配置"
	case errors.Is(err, ErrShopInvalidResponse):
		return "MCY 返回格式异常"
	case errors.Is(err, ErrShopRequestFailed):
		msg := err.Error()
		prefix := ErrShopRequestFailed.Error() + ": "
		if strings.HasPrefix(msg, prefix) {
			msg = strings.TrimSpace(strings.TrimPrefix(msg, prefix))
		}
		if msg != "" {
			return msg
		}
	}
	return "请稍后重试"
}
