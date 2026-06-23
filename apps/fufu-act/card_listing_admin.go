package activityapp

import (
	"errors"
	"fmt"
	"fufu/auth"
	"net/http"
	"os"
	"strings"
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
	Plans         []SaleCardPlan         `json:"plans"`
	Schedule      SaleCardScheduleConfig `json:"schedule"`
	RestockStatus SaleCardRestockStatus  `json:"restockStatus"`
}

type saleCardAdminConfigRequest struct {
	Schedule SaleCardScheduleConfig `json:"schedule"`
}

const saleCardMCYIntegrationPausedMessage = "自动补卡和 MCY 库存检测已暂时下线，当前不对接商城"

func handleAdminSaleCardsRun(w http.ResponseWriter, r *http.Request) {
	if !auth.CheckAdminToken(adminBearerToken(r), os.Getenv("ADMIN_TOKEN"), "") {
		writeJSONError(w, http.StatusUnauthorized, "未授权")
		return
	}
	writeJSONError(w, http.StatusServiceUnavailable, saleCardMCYIntegrationPausedMessage)
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
		writeJSON(w, http.StatusOK, saleCardAdminConfigResponse{Plans: saleCardPlanList(), Schedule: schedule, RestockStatus: loadSaleCardRestockStatus()})
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
		writeJSON(w, http.StatusOK, saleCardAdminConfigResponse{Plans: saleCardPlanList(), Schedule: schedule, RestockStatus: loadSaleCardRestockStatus()})
	default:
		w.Header().Set("Allow", "GET, POST")
		writeJSONError(w, http.StatusMethodNotAllowed, "Only GET, POST")
	}
}

// handleAdminSaleCardsStock is intentionally paused with the auto-restock
// integration. It must not contact MCY while the feature is offline.
func handleAdminSaleCardsStock(w http.ResponseWriter, r *http.Request) {
	if !auth.CheckAdminToken(adminBearerToken(r), os.Getenv("ADMIN_TOKEN"), "") {
		writeJSONError(w, http.StatusUnauthorized, "未授权")
		return
	}
	writeJSONError(w, http.StatusServiceUnavailable, saleCardMCYIntegrationPausedMessage)
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
