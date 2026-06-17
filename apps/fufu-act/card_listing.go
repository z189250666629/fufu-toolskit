package activityapp

import (
	"context"
	"errors"
	"fmt"
	"fufu/salecore"
	"fufu/tokens"
	"strings"
	"time"
)

var (
	ErrSaleCardInvalidPlan      = salecore.ErrInvalidPlan
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

	keys, err := createSaleCardTokenKeys(ctx, svc, plan, uploadCount)
	if err != nil {
		return result, err
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
	core := salecore.NormalizePlan(saleCardTokenPlan(plan))
	plan.ID = core.ID
	plan.Name = core.Name
	plan.Group = core.Group
	plan.TokenNameSlug = core.TokenNameSlug
	if plan.Group == "" {
		plan.Group = saleCardDefaultTokenGroup
	}
	if plan.TokenNameSlug == "" {
		plan.TokenNameSlug = firstSaleCardText(plan.ID, plan.Name, "sale-card")
	}
	plan.Remark = strings.TrimSpace(plan.Remark)
	if plan.Remark == "" {
		plan.Remark = plan.Name
	}
	return plan
}

func validateSaleCardPlan(plan SaleCardPlan) error {
	if err := salecore.ValidatePlan(saleCardTokenPlan(plan)); err != nil {
		return err
	}
	if plan.ItemID <= 0 || plan.SKUID <= 0 {
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
	return salecore.BuildSaleTokenName(saleCardTokenPlan(plan), saleCardNow())
}

func saleCardTokenBatchName(plan SaleCardPlan, now time.Time, index, total int) string {
	return salecore.BuildSaleTokenBatchName(saleCardTokenPlan(plan), now, index, total)
}

// saleCardSlugPrefix is the stable "<slug>-" prefix shared by every batch of a
// plan's card tokens. saleCardTokenName appends a timestamp to it.
func saleCardSlugPrefix(plan SaleCardPlan) string {
	return salecore.SaleCardSlugPrefix(saleCardTokenPlan(plan))
}

func sanitizeSaleCardSlug(value string) string {
	return salecore.SanitizeSaleCardSlug(value)
}

func createSaleCardTokenKeys(ctx context.Context, svc *tokens.Service, plan SaleCardPlan, count int) ([]string, error) {
	batchTime := saleCardNow()
	keys := make([]string, 0, count)
	quota := svc.DollarsToQuota(plan.Quota)
	for i := 0; i < count; i++ {
		name := saleCardTokenBatchName(plan, batchTime, i, count)
		createBody := buildSaleTokenCreateBody(name, quota, plan.Group, plan.IntervalUnit)
		created, err := svc.CreateTokenAndResolveKey(ctx, createBody, name)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrSaleCardGenerationFailed, err)
		}
		key := strings.TrimSpace(created.Key)
		if key == "" {
			return nil, fmt.Errorf("%w: NewAPI 返回空卡密", ErrSaleCardGenerationFailed)
		}
		keys = append(keys, key)
	}
	return keys, nil
}

func saleCardTokenPlan(plan SaleCardPlan) salecore.SaleCardPlan {
	return salecore.SaleCardPlan{
		ID:            plan.ID,
		Name:          plan.Name,
		Count:         plan.Count,
		TargetStock:   plan.TargetStock,
		Quota:         plan.Quota,
		Group:         plan.Group,
		Slot:          plan.Slot,
		IntervalUnit:  plan.IntervalUnit,
		TokenNameSlug: plan.TokenNameSlug,
	}
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
	endpoint := mcyUploadEndpoint()
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
	return salecore.BoolInt(value)
}

func firstSaleCardText(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
