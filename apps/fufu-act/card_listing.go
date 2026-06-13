package activityapp

import (
	"context"
	"errors"
	"fmt"
	"fufu/config"
	"fufu/newapi"
	"fufu/tokens"
	"net/http"
	"strings"
	"time"
)

var (
	ErrSaleCardInvalidPlan      = errors.New("sale card plan invalid")
	ErrSaleCardGenerationFailed = errors.New("sale card generation failed")
)

var saleCardNow = time.Now

type SaleCardPlan struct {
	ID            string  `json:"id,omitempty"`
	Name          string  `json:"name,omitempty"`
	Count         int     `json:"count"`
	TargetStock   int     `json:"targetStock,omitempty"`
	Quota         float64 `json:"quota"`
	Group         string  `json:"group,omitempty"`
	Slot          string  `json:"slot,omitempty"`
	IntervalUnit  int     `json:"intervalUnit"`
	ItemID        int     `json:"itemId"`
	SKUID         int     `json:"skuId"`
	Remark        string  `json:"remark,omitempty"`
	Unique        bool    `json:"unique"`
	TokenNameSlug string  `json:"tokenNameSlug,omitempty"`
}

// Sale-card 补卡时段分组：55 次混合特惠卡与月次卡各占一个独立时段。
const (
	saleCardSlotSpecial55 = "special55"
	saleCardSlotMonth     = "month"
)

// saleCardPlanSlot maps a plan id to its 补卡时段分组。
func saleCardPlanSlot(planID string) string {
	switch {
	case planID == "fufu-mix-special-55":
		return saleCardSlotSpecial55
	case strings.HasPrefix(planID, "fufu-mix-month-"):
		return saleCardSlotMonth
	default:
		return ""
	}
}

type SaleCardListingResult struct {
	PlanID       string   `json:"planId,omitempty"`
	PlanName     string   `json:"planName,omitempty"`
	ItemID       int      `json:"itemId"`
	SKUID        int      `json:"skuId"`
	Generated    int      `json:"generated"`
	Uploaded     int      `json:"uploaded"`
	CurrentStock int      `json:"currentStock"`
	TargetStock  int      `json:"targetStock"`
	ToUpload     int      `json:"toUpload"`
	Keys         []string `json:"keys"`
}

func saleCardPlanTemplates() map[string]SaleCardPlan {
	return map[string]SaleCardPlan{
		"fufu-mix-special-55": {
			ID:            "fufu-mix-special-55",
			Name:          "FuFu 55次混合特惠卡",
			Quota:         55,
			Group:         "mix",
			Slot:          saleCardSlotSpecial55,
			IntervalUnit:  9,
			ItemID:        29,
			SKUID:         66,
			Remark:        "FuFu 55次混合特惠卡",
			Unique:        true,
			TokenNameSlug: "fufu-mix-special-55",
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
		Group:         "mix",
		Slot:          saleCardSlotMonth,
		IntervalUnit:  9,
		ItemID:        28,
		SKUID:         skuID,
		Remark:        name,
		Unique:        true,
		TokenNameSlug: id,
	}
}

func generateAndUploadSaleCards(ctx context.Context, svc *tokens.Service, plan SaleCardPlan) (SaleCardListingResult, error) {
	plan = normalizeSaleCardPlan(plan)
	result := SaleCardListingResult{
		PlanID:   plan.ID,
		PlanName: plan.Name,
		ItemID:   plan.ItemID,
		SKUID:    plan.SKUID,
		Keys:     []string{},
	}
	if err := validateSaleCardPlan(plan); err != nil {
		return result, err
	}
	if svc == nil {
		return result, fmt.Errorf("%w: token service is not configured", ErrSaleCardGenerationFailed)
	}

	// 补卡：设了目标库存时，先查 MCY 商城当前可用（未售, status:0）卡量，补齐到
	// 目标（补 target-current）。库存来源是商城真实在售数，不是 NewAPI 令牌数。
	uploadCount := plan.Count
	if plan.TargetStock > 0 {
		current, err := queryMCYUsableStock(ctx, plan.ItemID, plan.SKUID)
		if err != nil {
			return result, err
		}
		result.CurrentStock = current
		result.TargetStock = plan.TargetStock
		uploadCount = max(0, plan.TargetStock-current)
	}
	result.ToUpload = uploadCount
	if uploadCount <= 0 {
		return result, nil
	}

	createBody := buildSaleTokenCreateBody(saleCardTokenName(plan), svc.DollarsToQuota(plan.Quota), plan.Group, plan.IntervalUnit)
	res, data, err := svc.CreateTokens(ctx, uploadCount, createBody)
	if err != nil {
		return result, fmt.Errorf("%w: %v", ErrSaleCardGenerationFailed, err)
	}
	if !res.OK() {
		return result, fmt.Errorf("%w: %s", ErrSaleCardGenerationFailed, res.BodyOr(http.StatusText(res.StatusCode)))
	}
	if !newapi.IsSuccess(data) {
		return result, fmt.Errorf("%w: %s", ErrSaleCardGenerationFailed, newapi.ErrorMessage(data, res.StatusCode, "NewAPI 创建卡密失败"))
	}
	keys := extractCreatedSaleCardKeys(data)
	if len(keys) != uploadCount {
		return result, fmt.Errorf("%w: NewAPI returned %d keys for %d requested cards", ErrSaleCardGenerationFailed, len(keys), uploadCount)
	}
	result.Keys = keys
	result.Generated = len(keys)

	if err := uploadCardsToMCY(ctx, plan, keys); err != nil {
		return result, err
	}
	result.Uploaded = len(keys)
	return result, nil
}

func normalizeSaleCardPlan(plan SaleCardPlan) SaleCardPlan {
	plan.ID = strings.TrimSpace(plan.ID)
	plan.Name = strings.TrimSpace(plan.Name)
	plan.Group = strings.TrimSpace(plan.Group)
	plan.Remark = strings.TrimSpace(plan.Remark)
	plan.TokenNameSlug = strings.TrimSpace(plan.TokenNameSlug)
	if plan.Group == "" {
		plan.Group = "mix"
	}
	if plan.Remark == "" {
		plan.Remark = plan.Name
	}
	if plan.TokenNameSlug == "" {
		plan.TokenNameSlug = firstNonEmpty(plan.ID, plan.Name, "sale-card")
	}
	return plan
}

func validateSaleCardPlan(plan SaleCardPlan) error {
	switch {
	case plan.TargetStock < 0:
		return fmt.Errorf("%w: target stock cannot be negative", ErrSaleCardInvalidPlan)
	case plan.TargetStock > 2000:
		return fmt.Errorf("%w: target stock must be 2000 or fewer", ErrSaleCardInvalidPlan)
	case plan.TargetStock == 0 && (plan.Count < 1 || plan.Count > 100):
		return fmt.Errorf("%w: count must be between 1 and 100", ErrSaleCardInvalidPlan)
	case plan.Quota <= 0:
		return fmt.Errorf("%w: quota must be positive", ErrSaleCardInvalidPlan)
	case plan.IntervalUnit == 0:
		return fmt.Errorf("%w: interval unit is required", ErrSaleCardInvalidPlan)
	case plan.ItemID <= 0 || plan.SKUID <= 0:
		return fmt.Errorf("%w: MCY item_id and sku_id are required", ErrSaleCardInvalidPlan)
	}
	return nil
}

func buildSaleTokenCreateBody(name string, quota int64, group string, intervalUnit int) map[string]any {
	return map[string]any{
		"name":              name,
		"remain_quota":      quota,
		"unlimited_quota":   false,
		"expired_time":      -1,
		"group":             group,
		"interval_quota":    quota,
		"interval_time":     -1,
		"trigger_last_time": 0,
		"interval_unit":     intervalUnit,
	}
}

func saleCardTokenName(plan SaleCardPlan) string {
	return saleCardSlugPrefix(plan) + saleCardNow().UTC().Format("20060102-150405")
}

// saleCardSlugPrefix is the stable "<slug>-" prefix shared by every batch of a
// plan's card tokens. saleCardTokenName appends a timestamp to it.
func saleCardSlugPrefix(plan SaleCardPlan) string {
	slug := sanitizeSaleCardSlug(plan.TokenNameSlug)
	if slug == "" {
		slug = "sale-card"
	}
	return slug + "-"
}

func sanitizeSaleCardSlug(value string) string {
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

func extractCreatedSaleCardKeys(data map[string]any) []string {
	keys := []string{}
	for _, item := range tokens.DataList(data) {
		raw := strings.TrimSpace(fmt.Sprint(item["key"]))
		if raw == "" || raw == "<nil>" {
			continue
		}
		key := tokens.EnsureFullKey(raw)
		keys = append(keys, key)
	}
	return keys
}

func uploadCardsToMCY(ctx context.Context, plan SaleCardPlan, keys []string) error {
	if len(keys) == 0 {
		return fmt.Errorf("%w: no generated cards to upload", ErrSaleCardGenerationFailed)
	}
	if err := ensureMCYCookie(ctx); err != nil {
		return fmt.Errorf("%w: %v", ErrShopLoginFailed, err)
	}
	payload := map[string]any{
		"item_id":     plan.ItemID,
		"sku_id":      plan.SKUID,
		"remark":      plan.Remark,
		"unique":      boolInt(plan.Unique),
		"upload_type": 0,
		"card":        strings.Join(keys, "\n"),
	}
	endpoint := firstNonEmpty(config.Env("MCY_UPLOAD_ENDPOINT"), "/plugin/virtual-card-ship/card/add")
	data, err := mcyEncryptedPost(ctx, endpoint, payload)
	if err != nil {
		return classifyShopRequestError(err)
	}
	if !mcyPayloadOK(data) {
		return fmt.Errorf("%w: %s", ErrShopRequestFailed, mcyPayloadMessage(data, "MCY card upload failed"))
	}
	return nil
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
