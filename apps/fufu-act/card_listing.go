package activityapp

import (
	"context"
	"errors"
	"fmt"
	"fufu/activity"
	"fufu/newapi"
	"fufu/salecore"
	"fufu/tokens"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	ErrSaleCardInvalidPlan      = salecore.ErrInvalidPlan
	ErrSaleCardGenerationFailed = errors.New("sale card generation failed")
	errSaleCardBatchUnsupported = errors.New("sale card batch create unsupported")
)

var saleCardNow = time.Now

// saleCardRestockMaxUploadPerJob is a production safety valve for automatic
// restock. When a SKU is far below target, do not create/upload the entire
// deficit in one scheduler run; smooth it over subsequent runs instead.
var saleCardRestockMaxUploadPerJob = 50

type SaleCardPlan = salecore.SaleCardPlan

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

type SaleCardTestKeyResult struct {
	PlanID    string   `json:"planId,omitempty"`
	PlanName  string   `json:"planName,omitempty"`
	Quota     float64  `json:"quota"`
	Game      string   `json:"game"`
	DrawCount int      `json:"drawCount"`
	Generated int      `json:"generated"`
	Keys      []string `json:"keys"`
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
		uploadCount = capSaleCardRestockUploadCount(max(0, plan.TargetStock-current))
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

func capSaleCardRestockUploadCount(count int) int {
	if count <= 0 {
		return 0
	}
	limit := saleCardRestockMaxUploadPerJob
	if limit <= 0 || count <= limit {
		return count
	}
	return limit
}

func generateSaleCardTestKeys(ctx context.Context, svc *tokens.Service, plan SaleCardPlan, count int, cfg activity.Config) (SaleCardTestKeyResult, error) {
	plan = normalizeSaleCardPlan(plan)
	if count <= 0 {
		count = 1
	}
	plan.Count = count
	plan.TargetStock = 0
	result := SaleCardTestKeyResult{
		PlanID:    plan.ID,
		PlanName:  plan.Name,
		Quota:     plan.Quota,
		Game:      cfg.GameForTier(plan.Quota),
		DrawCount: cfg.DrawCountForTier(plan.Quota),
		Keys:      []string{},
	}
	if err := salecore.ValidatePlan(saleCardTokenPlan(plan)); err != nil {
		return result, err
	}
	if svc == nil {
		return result, fmt.Errorf("%w: token service is not configured", ErrSaleCardGenerationFailed)
	}
	keys, err := createSaleCardTestTokenKeys(ctx, svc, plan, count)
	if err != nil {
		return result, err
	}
	result.Keys = keys
	result.Generated = len(keys)
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
	if count > 1 {
		keys, err := createSaleCardTokenKeysBatch(ctx, svc, plan, count)
		if err == nil {
			return keys, nil
		}
		if !errors.Is(err, errSaleCardBatchUnsupported) {
			return nil, err
		}
	}
	return createSaleCardTokenKeysIndividually(ctx, svc, plan, count)
}

func createSaleCardTokenKeysIndividually(ctx context.Context, svc *tokens.Service, plan SaleCardPlan, count int) ([]string, error) {
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

func createSaleCardTokenKeysBatch(ctx context.Context, svc *tokens.Service, plan SaleCardPlan, count int) ([]string, error) {
	name := saleCardTokenName(plan)
	createBody := buildSaleTokenCreateBody(name, svc.DollarsToQuota(plan.Quota), plan.Group, plan.IntervalUnit)
	res, data, err := svc.CreateTokens(ctx, count, createBody)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSaleCardGenerationFailed, err)
	}
	if !res.OK() {
		if saleCardBatchUnsupportedResponse(res, data) {
			return nil, fmt.Errorf("%w: %s", errSaleCardBatchUnsupported, res.BodyOr(http.StatusText(res.StatusCode)))
		}
		return nil, fmt.Errorf("%w: %s", ErrSaleCardGenerationFailed, res.BodyOr(http.StatusText(res.StatusCode)))
	}
	if !newapi.IsSuccess(data) {
		message := newapi.ErrorMessage(data, res.StatusCode, "NewAPI 批量创建卡密失败")
		if saleCardBatchUnsupportedMessage(message) {
			return nil, fmt.Errorf("%w: %s", errSaleCardBatchUnsupported, message)
		}
		return nil, fmt.Errorf("%w: %s", ErrSaleCardGenerationFailed, message)
	}
	if keys := extractCreatedSaleCardKeys(data); len(keys) == count {
		return keys, nil
	}

	searchSize := count * 2
	if searchSize < 10 {
		searchSize = 10
	}
	found, err := svc.SearchTokensByNamePrefix(ctx, name, searchSize)
	if err != nil {
		return nil, fmt.Errorf("%w: 批量创建后查找 token 失败: %v", ErrSaleCardGenerationFailed, err)
	}
	if len(found) < count {
		return nil, fmt.Errorf("%w: NewAPI 批量创建成功但只查回 %d/%d 个 token", ErrSaleCardGenerationFailed, len(found), count)
	}
	sort.Slice(found, func(i, j int) bool {
		if found[i].ID == found[j].ID {
			return found[i].Name < found[j].Name
		}
		return found[i].ID < found[j].ID
	})
	return saleCardTokenKeysFromCreatedTokens(ctx, svc, found[:count])
}

func extractCreatedSaleCardKeys(data map[string]any) []string {
	keys := []string{}
	for _, item := range tokens.DataList(data) {
		raw := strings.TrimSpace(fmt.Sprint(item["key"]))
		if raw == "" || raw == "<nil>" || tokens.IsMaskedKey(raw) {
			continue
		}
		keys = append(keys, tokens.EnsureFullKey(raw))
	}
	return keys
}

func saleCardTokenKeysFromCreatedTokens(ctx context.Context, svc *tokens.Service, created []tokens.Token) ([]string, error) {
	keys := make([]string, len(created))
	missing := []int{}
	missingPos := map[int]int{}
	for i, token := range created {
		key := strings.TrimSpace(token.Key)
		if key != "" && !tokens.IsMaskedKey(key) {
			keys[i] = tokens.EnsureFullKey(key)
			continue
		}
		if token.ID <= 0 {
			return nil, fmt.Errorf("%w: NewAPI 批量创建成功但第 %d 个 token 缺少 id/key", ErrSaleCardGenerationFailed, i+1)
		}
		missing = append(missing, token.ID)
		missingPos[token.ID] = i
	}
	if len(missing) > 0 {
		if resolved, err := svc.GetTokenKeysBatch(ctx, missing); err == nil {
			for id, key := range resolved {
				if pos, ok := missingPos[id]; ok {
					keys[pos] = strings.TrimSpace(key)
				}
			}
		}
		for _, id := range missing {
			pos := missingPos[id]
			if strings.TrimSpace(keys[pos]) != "" {
				continue
			}
			key, err := svc.ResolveTokenKey(ctx, id)
			if err != nil {
				return nil, fmt.Errorf("%w: 批量创建后读取 token %d key 失败: %v", ErrSaleCardGenerationFailed, id, err)
			}
			keys[pos] = key
		}
	}
	for i, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" || tokens.IsMaskedKey(key) {
			return nil, fmt.Errorf("%w: NewAPI 批量创建成功但第 %d 个 token key 为空或被隐藏", ErrSaleCardGenerationFailed, i+1)
		}
		keys[i] = tokens.EnsureFullKey(key)
	}
	return keys, nil
}

func saleCardBatchUnsupportedResponse(res newapi.Response, data map[string]any) bool {
	if res.StatusCode == http.StatusNotFound || res.StatusCode == http.StatusMethodNotAllowed {
		return true
	}
	return saleCardBatchUnsupportedMessage(newapi.ErrorMessage(data, res.StatusCode, ""))
}

func saleCardBatchUnsupportedMessage(message string) bool {
	message = strings.ToLower(strings.TrimSpace(message))
	return strings.Contains(message, "invalid url") ||
		strings.Contains(message, "not found") ||
		strings.Contains(message, "/api/token/tokens")
}

func createSaleCardTestTokenKeys(ctx context.Context, svc *tokens.Service, plan SaleCardPlan, count int) ([]string, error) {
	stamp := strconv.FormatInt(saleCardNow().UTC().Unix(), 36)
	keys := make([]string, 0, count)
	quota := svc.DollarsToQuota(plan.Quota)
	for i := 0; i < count; i++ {
		name, err := saleCardTestTokenName(plan.Quota, stamp, i, count)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrSaleCardInvalidPlan, err)
		}
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

func saleCardTestTokenName(quota float64, stamp string, index, total int) (string, error) {
	dollars := strconv.FormatFloat(quota, 'f', -1, 64)
	base := strings.TrimSpace(dollars) + "-act-test"
	if base == "-act-test" {
		return "", errors.New("测试卡额度无效")
	}
	suffix := strings.TrimSpace(stamp)
	if suffix == "" {
		suffix = strconv.FormatInt(time.Now().UTC().Unix(), 36)
	}
	if total > 1 {
		if index < 0 {
			index = 0
		}
		width := len(strconv.Itoa(total))
		if width < 2 {
			width = 2
		}
		suffix += "-" + fmt.Sprintf("%0*d", width, index+1)
	}
	name := base + "-" + suffix
	if len([]rune(name)) > salecore.MaxSaleTokenNameRunes {
		return "", errors.New("测试卡名称过长")
	}
	return name, nil
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
