package activityapp

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"fufu/newapi"
	"fufu/rawconv"
	"fufu/tokens"
)

const rewardPlanTitlePrefix = "act-reward:"

var subscriptionRewardRetryAttempts = 2
var subscriptionRewardRetryDelay = 200 * time.Millisecond
var subscriptionRewardRequestTimeout = 5 * time.Second
var subscriptionRewardFallbackDuration = 30 * 24 * time.Hour
var subscriptionRewardBindRetryCooldown = 30 * time.Second

type subscriptionPlanSummary struct {
	ID            int64
	Title         string
	TotalAmount   int64
	DurationUnit  string
	DurationValue int64
	CustomSeconds int64
	Enabled       bool
}

type rewardPlanDuration struct {
	Unit          string
	Value         int64
	CustomSeconds int64
}

func addQuotaToSubscriptionReward(ctx context.Context, client *newapi.Client, quotaUnit int64, card Card, prizeDollars int) error {
	if client == nil {
		return fmt.Errorf("subscription runtime is not configured")
	}
	if !card.SubscriptionID.Valid || card.SubscriptionID.Int64 <= 0 {
		return fmt.Errorf("subscription id is missing")
	}
	if !card.UserID.Valid || card.UserID.Int64 <= 0 {
		return fmt.Errorf("subscription user id is missing")
	}
	service := tokens.NewService(client)
	if quotaUnit > 0 {
		service.QuotaUnit = quotaUnit
	}
	quota := service.DollarsToQuota(float64(prizeDollars))

	var lastErr error
	attempts := subscriptionRewardRetryAttempts
	if attempts <= 0 {
		attempts = 1
	}
	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 {
			if err := sleepWithContext(ctx, subscriptionRewardRetryDelay); err != nil {
				if lastErr != nil {
					return lastErr
				}
				return err
			}
		}
		if err := addQuotaToSubscriptionRewardOnce(ctx, client, card, quota); err != nil {
			if isPermanentCreditError(err) {
				return err
			}
			lastErr = err
			continue
		}
		return nil
	}
	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("subscription reward issuance failed")
}

func addQuotaToSubscriptionRewardOnce(ctx context.Context, client *newapi.Client, card Card, quota int64) (resultErr error) {
	userID := card.UserID.Int64
	subscriptionID := card.SubscriptionID.Int64

	if _, ok, err := lookupRewardIssuance(card.CardKey); err != nil {
		return err
	} else if ok {
		_ = releaseRewardPlanLease(card.CardKey)
		return nil
	}

	subs, err := loadSubscriptionSummariesForReward(ctx, client, userID)
	if err != nil {
		return err
	}
	plans, err := loadSubscriptionPlansForReward(ctx, client)
	if err != nil {
		return err
	}
	duration := rewardPlanDurationForReward(plans, subs, subscriptionID)

	lease, err := acquireOrResumeRewardPlanLease(card.CardKey)
	if err != nil {
		return err
	}
	leaseHeld := true
	defer func() {
		if leaseHeld && (resultErr == nil || isPermanentCreditError(resultErr)) {
			_ = releaseRewardPlanLease(card.CardKey)
		}
	}()

	plan, err := ensureRewardPoolPlanConfigured(ctx, client, plans, &lease, quota, duration)
	if err != nil {
		return err
	}
	if err := saveRewardPlanLeasePlanID(card.CardKey, plan.ID); err != nil {
		return err
	}
	lease.PlanID = plan.ID

	if rewardSub, ok := findNewlyBoundRewardSubscription(subs, plan.ID, subscriptionIDSet(lease.BaselineSubscriptionIDs)); ok {
		if err := saveRewardIssuance(card, plan.ID, rewardSub.ID, quota); err != nil {
			return err
		}
		return nil
	}

	now := time.Now().Unix()
	if lease.NextBindAt > now {
		rewardSub, observed, err := observeRewardSubscriptionAfterBind(ctx, client, userID, plan.ID, subscriptionIDSet(lease.BaselineSubscriptionIDs))
		if err != nil {
			return err
		}
		if observed {
			if err := saveRewardIssuance(card, plan.ID, rewardSub.ID, quota); err != nil {
				return err
			}
			return nil
		}
		return fmt.Errorf("奖励订阅发放确认中，请稍后重试")
	}

	beforeBind := activeUserSubscriptionIDsForPlan(subs, plan.ID)
	baselineIDs := subscriptionIDList(beforeBind)
	if err := prepareRewardPlanLeaseForBind(card, &lease, plan.ID, quota, duration, baselineIDs); err != nil {
		return err
	}

	if err := bindRewardSubscriptionPlan(ctx, client, userID, plan.ID); err != nil {
		if rewardSub, observed, lookupErr := observeRewardSubscriptionAfterBind(ctx, client, userID, plan.ID, beforeBind); observed {
			if saveErr := saveRewardIssuance(card, plan.ID, rewardSub.ID, quota); saveErr != nil {
				return saveErr
			}
			return nil
		} else if lookupErr != nil {
			return lookupErr
		}
		return err
	}

	rewardSub, observed, err := observeRewardSubscriptionAfterBind(ctx, client, userID, plan.ID, beforeBind)
	if err != nil {
		return err
	}
	if !observed {
		return fmt.Errorf("奖励订阅发放待确认，请稍后重试")
	}
	if err := saveRewardIssuance(card, plan.ID, rewardSub.ID, quota); err != nil {
		return err
	}
	return nil
}

func loadSubscriptionPlansForReward(ctx context.Context, client *newapi.Client) ([]subscriptionPlanSummary, error) {
	res, data, err := subscriptionRewardGet(ctx, client, "/api/subscription/admin/plans")
	if err != nil {
		return nil, err
	}
	if !res.OK() || !newapi.IsSuccess(data) {
		return nil, subscriptionRewardAPIError(res, data, "订阅套餐查询失败，请稍后再试")
	}
	items := newapi.PayloadItems(data, "data", "plans")
	out := make([]subscriptionPlanSummary, 0, len(items))
	for _, item := range items {
		raw := item
		if nested, ok := item["plan"].(map[string]any); ok {
			raw = nested
		}
		id := rawconv.Int64(raw["id"])
		if id <= 0 {
			continue
		}
		out = append(out, subscriptionPlanSummary{
			ID:            id,
			Title:         strings.TrimSpace(fmt.Sprint(raw["title"])),
			TotalAmount:   rawconv.Int64(raw["total_amount"]),
			DurationUnit:  strings.TrimSpace(fmt.Sprint(raw["duration_unit"])),
			DurationValue: rawconv.Int64(raw["duration_value"]),
			CustomSeconds: rawconv.Int64(raw["custom_seconds"]),
			Enabled:       rewardPlanEnabled(raw["enabled"]),
		})
	}
	return out, nil
}

func createRewardSubscriptionPlan(ctx context.Context, client *newapi.Client, title string, quota int64, duration rewardPlanDuration) (int64, error) {
	payload := buildRewardPlanPayload(title, quota, duration)
	res, data, err := subscriptionRewardRequest(ctx, client, http.MethodPost, "/api/subscription/admin/plans", payload)
	if err != nil {
		return 0, err
	}
	if !res.OK() || !newapi.IsSuccess(data) {
		return 0, subscriptionRewardAPIError(res, data, "奖励订阅套餐创建失败")
	}
	if id := subscriptionPlanIDFromPayload(data); id > 0 {
		return id, nil
	}
	return 0, newPermanentCreditError(fmt.Errorf("奖励订阅套餐创建响应异常"))
}

func updateRewardSubscriptionPlan(ctx context.Context, client *newapi.Client, planID int64, title string, quota int64, duration rewardPlanDuration) error {
	payload := buildRewardPlanPayload(title, quota, duration)
	res, data, err := subscriptionRewardRequest(ctx, client, http.MethodPut, fmt.Sprintf("/api/subscription/admin/plans/%d", planID), payload)
	if err != nil {
		return err
	}
	if !res.OK() || !newapi.IsSuccess(data) {
		return subscriptionRewardAPIError(res, data, "奖励订阅套餐配置失败")
	}
	return nil
}

func buildRewardPlanPayload(title string, quota int64, duration rewardPlanDuration) map[string]any {
	duration = normalizeRewardPlanDuration(duration)
	return map[string]any{
		"plan": map[string]any{
			"title":                      title,
			"subtitle":                   "活动中奖补发（自动生成）",
			"price_amount":               0,
			"currency":                   "USD",
			"duration_unit":              duration.Unit,
			"duration_value":             duration.Value,
			"custom_seconds":             duration.CustomSeconds,
			"enabled":                    false,
			"sort_order":                 0,
			"allow_balance_pay":          false,
			"max_purchase_per_user":      0,
			"total_amount":               quota,
			"upgrade_group":              "",
			"quota_reset_period":         "never",
			"quota_reset_custom_seconds": 0,
		},
	}
}

func bindRewardSubscriptionPlan(ctx context.Context, client *newapi.Client, userID, planID int64) error {
	res, data, err := subscriptionRewardRequest(ctx, client, http.MethodPost, fmt.Sprintf("/api/subscription/admin/users/%d/subscriptions", userID), map[string]any{
		"plan_id": planID,
	})
	if err != nil {
		return err
	}
	if !res.OK() || !newapi.IsSuccess(data) {
		return subscriptionRewardAPIError(res, data, "奖励订阅发放失败")
	}
	return nil
}

func loadSubscriptionSummariesForReward(ctx context.Context, client *newapi.Client, userID int64) ([]subscriptionSummary, error) {
	res, data, err := subscriptionRewardGet(ctx, client, fmt.Sprintf("/api/subscription/admin/users/%d/subscriptions", userID))
	if err != nil {
		return nil, err
	}
	if !res.OK() || !newapi.IsSuccess(data) {
		return nil, subscriptionRewardAPIError(res, data, "订阅查询失败，请稍后再试")
	}
	items := newapi.PayloadItems(data, "data", "subscriptions")
	out := make([]subscriptionSummary, 0, len(items))
	for _, item := range items {
		raw := item
		if nested, ok := item["subscription"].(map[string]any); ok {
			raw = nested
		}
		sub := subscriptionSummary{
			ID:          rawconv.Int64(raw["id"]),
			UserID:      rawconv.Int64(raw["user_id"]),
			PlanID:      rawconv.Int64(raw["plan_id"]),
			AmountTotal: rawconv.Int64(raw["amount_total"]),
			AmountUsed:  rawconv.Int64(raw["amount_used"]),
			StartTime:   rawconv.Int64(raw["start_time"]),
			EndTime:     rawconv.Int64(raw["end_time"]),
			Status:      strings.TrimSpace(fmt.Sprint(raw["status"])),
		}
		if sub.ID > 0 {
			out = append(out, sub)
		}
	}
	return out, nil
}

func ensureRewardPoolPlanConfigured(ctx context.Context, client *newapi.Client, plans []subscriptionPlanSummary, lease *rewardPlanLease, quota int64, duration rewardPlanDuration) (subscriptionPlanSummary, error) {
	duration = normalizeRewardPlanDuration(duration)
	title := rewardPlanPoolTitle(lease.Slot)
	plan, ok := findRewardPoolPlan(plans, lease, title)
	if !ok {
		planID, err := createRewardSubscriptionPlan(ctx, client, title, quota, duration)
		if err != nil {
			if recoveredID, lookupErr := findRewardPlanAfterCreateFailure(ctx, client, title); lookupErr == nil && recoveredID > 0 {
				planID = recoveredID
			} else {
				return subscriptionPlanSummary{}, err
			}
		}
		lease.PlanID = planID
		plan = subscriptionPlanSummary{ID: planID, Title: title}
		ok = true
	}
	if err := updateRewardSubscriptionPlan(ctx, client, plan.ID, title, quota, duration); err != nil {
		refreshed, lookupErr := loadSubscriptionPlansForReward(ctx, client)
		if lookupErr == nil {
			if refreshedPlan, ok := findRewardPoolPlan(refreshed, &rewardPlanLease{PlanID: plan.ID, Slot: lease.Slot}, title); ok && rewardPoolPlanMatches(refreshedPlan, lease.Slot, quota, duration) {
				return refreshedPlan, nil
			}
		}
		return subscriptionPlanSummary{}, err
	}
	return subscriptionPlanSummary{
		ID:            plan.ID,
		Title:         title,
		TotalAmount:   quota,
		DurationUnit:  duration.Unit,
		DurationValue: duration.Value,
		CustomSeconds: duration.CustomSeconds,
		Enabled:       false,
	}, nil
}

func findRewardPlanAfterCreateFailure(ctx context.Context, client *newapi.Client, title string) (int64, error) {
	plans, err := loadSubscriptionPlansForReward(ctx, client)
	if err != nil {
		return 0, err
	}
	return findSubscriptionPlanIDByTitle(plans, title), nil
}

func findSubscriptionPlanIDByTitle(plans []subscriptionPlanSummary, title string) int64 {
	title = strings.TrimSpace(title)
	for _, plan := range plans {
		if strings.TrimSpace(plan.Title) == title {
			return plan.ID
		}
	}
	return 0
}

func findSubscriptionPlanByID(plans []subscriptionPlanSummary, planID int64) (subscriptionPlanSummary, bool) {
	if planID <= 0 {
		return subscriptionPlanSummary{}, false
	}
	for _, plan := range plans {
		if plan.ID == planID {
			return plan, true
		}
	}
	return subscriptionPlanSummary{}, false
}

func findRewardPoolPlan(plans []subscriptionPlanSummary, lease *rewardPlanLease, title string) (subscriptionPlanSummary, bool) {
	if lease != nil && lease.PlanID > 0 {
		if plan, ok := findSubscriptionPlanByID(plans, lease.PlanID); ok {
			return plan, true
		}
	}
	if planID := findSubscriptionPlanIDByTitle(plans, title); planID > 0 {
		return findSubscriptionPlanByID(plans, planID)
	}
	return subscriptionPlanSummary{}, false
}

func rewardPoolPlanMatches(plan subscriptionPlanSummary, slot int, quota int64, duration rewardPlanDuration) bool {
	duration = normalizeRewardPlanDuration(duration)
	return strings.TrimSpace(plan.Title) == rewardPlanPoolTitle(slot) &&
		plan.TotalAmount == quota &&
		rewardPlanDurationMatches(plan, duration) &&
		!plan.Enabled
}

func activeUserSubscriptionIDsForPlan(subs []subscriptionSummary, planID int64) map[int64]struct{} {
	ids := map[int64]struct{}{}
	if planID <= 0 {
		return ids
	}
	for _, sub := range subs {
		if sub.PlanID != planID || !strings.EqualFold(strings.TrimSpace(sub.Status), "active") || sub.ID <= 0 {
			continue
		}
		ids[sub.ID] = struct{}{}
	}
	return ids
}

func observeRewardSubscriptionAfterBind(ctx context.Context, client *newapi.Client, userID, planID int64, before map[int64]struct{}) (subscriptionSummary, bool, error) {
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			if err := sleepWithContext(ctx, subscriptionRewardRetryDelay); err != nil {
				return subscriptionSummary{}, false, err
			}
		}
		subs, err := loadSubscriptionSummariesForReward(ctx, client, userID)
		if err != nil {
			lastErr = err
			continue
		}
		if rewardSub, ok := findNewlyBoundRewardSubscription(subs, planID, before); ok {
			return rewardSub, true, nil
		}
		lastErr = nil
	}
	return subscriptionSummary{}, false, lastErr
}

func findNewlyBoundRewardSubscription(subs []subscriptionSummary, planID int64, before map[int64]struct{}) (subscriptionSummary, bool) {
	var chosen subscriptionSummary
	found := false
	for _, sub := range subs {
		if sub.PlanID != planID || !strings.EqualFold(strings.TrimSpace(sub.Status), "active") || sub.ID <= 0 {
			continue
		}
		if _, seen := before[sub.ID]; seen {
			continue
		}
		if !found || sub.ID > chosen.ID {
			chosen = sub
			found = true
		}
	}
	return chosen, found
}

func rewardPlanDurationForReward(plans []subscriptionPlanSummary, subs []subscriptionSummary, subscriptionID int64) rewardPlanDuration {
	if plan, ok := rewardSourceSubscriptionPlan(plans, subs, subscriptionID); ok {
		return rewardPlanDurationFromPlan(plan)
	}
	return rewardPlanDurationFromSeconds(rewardPlanDurationSeconds(subs, subscriptionID))
}

func rewardSourceSubscriptionPlan(plans []subscriptionPlanSummary, subs []subscriptionSummary, subscriptionID int64) (subscriptionPlanSummary, bool) {
	var planID int64
	for _, sub := range subs {
		if sub.ID == subscriptionID {
			planID = sub.PlanID
			break
		}
	}
	if planID <= 0 {
		return subscriptionPlanSummary{}, false
	}
	for _, plan := range plans {
		if plan.ID == planID {
			return plan, true
		}
	}
	return subscriptionPlanSummary{}, false
}

func rewardPlanDurationFromPlan(plan subscriptionPlanSummary) rewardPlanDuration {
	switch strings.ToLower(strings.TrimSpace(plan.DurationUnit)) {
	case "year":
		if plan.DurationValue > 0 {
			return rewardPlanDuration{Unit: "month", Value: plan.DurationValue * 12}
		}
	case "month":
		if plan.DurationValue > 0 {
			return rewardPlanDuration{Unit: "month", Value: plan.DurationValue}
		}
	case "day":
		if plan.DurationValue > 0 {
			return rewardPlanDuration{Unit: "day", Value: plan.DurationValue}
		}
	case "hour":
		if plan.DurationValue > 0 && plan.DurationValue%24 == 0 {
			return rewardPlanDuration{Unit: "day", Value: plan.DurationValue / 24}
		}
	case "custom":
		if plan.CustomSeconds > 0 {
			return rewardPlanDurationFromSeconds(plan.CustomSeconds)
		}
	}
	return rewardPlanDurationFromSeconds(plan.CustomSeconds)
}

func rewardPlanDurationSeconds(subs []subscriptionSummary, subscriptionID int64) int64 {
	now := time.Now().Unix()
	for _, sub := range subs {
		if sub.ID != subscriptionID {
			continue
		}
		if sub.EndTime > now {
			return sub.EndTime - now
		}
		break
	}
	return int64(subscriptionRewardFallbackDuration / time.Second)
}

func rewardPlanDurationFromSeconds(seconds int64) rewardPlanDuration {
	if seconds <= 0 {
		seconds = int64(subscriptionRewardFallbackDuration / time.Second)
	}
	const daySeconds = int64(24 * time.Hour / time.Second)
	if seconds >= daySeconds && seconds%daySeconds == 0 {
		return rewardPlanDuration{
			Unit:  "day",
			Value: seconds / daySeconds,
		}
	}
	return rewardPlanDuration{
		Unit:          "custom",
		Value:         1,
		CustomSeconds: seconds,
	}
}

func normalizeRewardPlanDuration(duration rewardPlanDuration) rewardPlanDuration {
	switch strings.ToLower(strings.TrimSpace(duration.Unit)) {
	case "month":
		if duration.Value > 0 {
			return rewardPlanDuration{Unit: "month", Value: duration.Value}
		}
	case "day":
		if duration.Value > 0 {
			return rewardPlanDuration{Unit: "day", Value: duration.Value}
		}
	case "custom":
		if duration.CustomSeconds > 0 {
			return rewardPlanDuration{Unit: "custom", Value: 1, CustomSeconds: duration.CustomSeconds}
		}
	}
	return rewardPlanDurationFromSeconds(duration.CustomSeconds)
}

func rewardPlanDurationMatches(plan subscriptionPlanSummary, duration rewardPlanDuration) bool {
	planDuration := normalizeRewardPlanDuration(rewardPlanDuration{
		Unit:          plan.DurationUnit,
		Value:         plan.DurationValue,
		CustomSeconds: plan.CustomSeconds,
	})
	duration = normalizeRewardPlanDuration(duration)
	return planDuration.Unit == duration.Unit &&
		planDuration.Value == duration.Value &&
		planDuration.CustomSeconds == duration.CustomSeconds
}

func rewardPlanEnabled(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case string:
		x = strings.TrimSpace(strings.ToLower(x))
		return x == "1" || x == "true" || x == "yes"
	default:
		return rawconv.Int64(v) != 0
	}
}

func rewardPlanPoolTitle(slot int) string {
	return fmt.Sprintf("%spool:%d", rewardPlanTitlePrefix, slot)
}

func isRewardSubscriptionPlanTitle(title string) bool {
	title = strings.ToLower(strings.TrimSpace(title))
	return strings.HasPrefix(title, rewardPlanTitlePrefix) || strings.Contains(title, "reward")
}

func subscriptionIDSet(ids []int64) map[int64]struct{} {
	out := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id > 0 {
			out[id] = struct{}{}
		}
	}
	return out
}

func subscriptionIDList(ids map[int64]struct{}) []int64 {
	if len(ids) == 0 {
		return []int64{}
	}
	out := make([]int64, 0, len(ids))
	for id := range ids {
		out = append(out, id)
	}
	return out
}

func subscriptionPlanIDFromPayload(data map[string]any) int64 {
	if raw, ok := data["data"].(map[string]any); ok {
		if nested, ok := raw["plan"].(map[string]any); ok {
			raw = nested
		}
		if id := rawconv.Int64(raw["id"]); id > 0 {
			return id
		}
	}
	if id := rawconv.Int64(data["id"]); id > 0 {
		return id
	}
	return 0
}

func subscriptionRewardRequest(ctx context.Context, client *newapi.Client, method, endpoint string, body any) (newapi.Response, map[string]any, error) {
	reqCtx, cancel := subscriptionRewardContext(ctx)
	defer cancel()
	return client.Request(reqCtx, method, endpoint, body)
}

func subscriptionRewardGet(ctx context.Context, client *newapi.Client, endpoint string) (newapi.Response, map[string]any, error) {
	return subscriptionRewardRequest(ctx, client, http.MethodGet, endpoint, nil)
}

func subscriptionRewardContext(parent context.Context) (context.Context, context.CancelFunc) {
	if subscriptionRewardRequestTimeout <= 0 {
		return context.WithCancel(parent)
	}
	return context.WithTimeout(parent, subscriptionRewardRequestTimeout)
}

func subscriptionRewardAPIError(res newapi.Response, data map[string]any, fallback string) error {
	err := fmt.Errorf("%s", newapi.ErrorMessage(data, res.StatusCode, fallback))
	if res.StatusCode >= 400 && res.StatusCode < 500 && res.StatusCode != http.StatusRequestTimeout && res.StatusCode != http.StatusTooManyRequests {
		return newPermanentCreditError(err)
	}
	return err
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
