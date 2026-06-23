package activityapp

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"

	"fufu/activity"
	"fufu/newapi"
	"fufu/rawconv"
)

func loginWithSubscriptionIdentity(r *http.Request, userID int64, username string) (Card, error) {
	site, client, _ := snapshotSubscriptionRuntime()
	if client == nil {
		return Card{}, httpErr{http.StatusServiceUnavailable, "活动订阅鉴权未配置"}
	}

	user, err := loadSubscriptionUser(r.Context(), client, userID)
	if err != nil {
		return Card{}, err
	}
	if user.ID != userID || user.Username != username {
		return Card{}, httpErr{http.StatusForbidden, "用户ID与用户名不匹配"}
	}

	subs, err := loadSubscriptionSummaries(r.Context(), client, userID)
	if err != nil {
		return Card{}, err
	}
	planTitles, err := loadSubscriptionPlanTitles(r.Context(), client)
	if err != nil {
		return Card{}, err
	}
	return selectSubscriptionLoginCard(user, subs, planTitles, quotaUnitOrDefault(site.QuotaUnit))
}

func loadSubscriptionUser(ctx context.Context, client *newapi.Client, userID int64) (subscriptionUpstreamUser, error) {
	res, data, err := client.Get(ctx, fmt.Sprintf("/api/user/%d", userID))
	if err != nil {
		return subscriptionUpstreamUser{}, httpErr{http.StatusBadGateway, "用户校验失败，请稍后再试"}
	}
	if !res.OK() {
		if res.StatusCode == http.StatusNotFound {
			return subscriptionUpstreamUser{}, httpErr{http.StatusNotFound, "用户不存在"}
		}
		return subscriptionUpstreamUser{}, httpErr{http.StatusBadGateway, "用户校验失败，请稍后再试"}
	}
	if !newapi.IsSuccess(data) {
		return subscriptionUpstreamUser{}, httpErr{http.StatusBadGateway, "用户校验失败，请稍后再试"}
	}
	raw, _ := data["data"].(map[string]any)
	id := rawconv.Int64(raw["id"])
	username := strings.TrimSpace(fmt.Sprint(raw["username"]))
	if id <= 0 || username == "" || username == "<nil>" {
		return subscriptionUpstreamUser{}, httpErr{http.StatusNotFound, "用户不存在"}
	}
	return subscriptionUpstreamUser{ID: id, Username: username}, nil
}

func loadSubscriptionSummaries(ctx context.Context, client *newapi.Client, userID int64) ([]subscriptionSummary, error) {
	res, data, err := client.Get(ctx, fmt.Sprintf("/api/subscription/admin/users/%d/subscriptions", userID))
	if err != nil {
		return nil, httpErr{http.StatusBadGateway, "订阅查询失败，请稍后再试"}
	}
	if !res.OK() || !newapi.IsSuccess(data) {
		return nil, httpErr{http.StatusBadGateway, "订阅查询失败，请稍后再试"}
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

func loadSubscriptionPlanTitles(ctx context.Context, client *newapi.Client) (map[int64]string, error) {
	res, data, err := client.Get(ctx, "/api/subscription/admin/plans")
	if err != nil {
		return nil, httpErr{http.StatusBadGateway, "订阅套餐查询失败，请稍后再试"}
	}
	if !res.OK() || !newapi.IsSuccess(data) {
		return nil, httpErr{http.StatusBadGateway, "订阅套餐查询失败，请稍后再试"}
	}
	items := newapi.PayloadItems(data, "data", "plans")
	out := make(map[int64]string, len(items))
	for _, item := range items {
		raw := item
		if nested, ok := item["plan"].(map[string]any); ok {
			raw = nested
		}
		id := rawconv.Int64(raw["id"])
		if id <= 0 {
			continue
		}
		out[id] = strings.TrimSpace(fmt.Sprint(raw["title"]))
	}
	return out, nil
}

func selectSubscriptionLoginCard(user subscriptionUpstreamUser, subs []subscriptionSummary, planTitles map[int64]string, quotaUnit int64) (Card, error) {
	cfg := activity.CloneConfig(SnapshotRuntimeConfig())
	var exhausted Card
	hasExhausted := false
	var createPlan loginCardPlan
	needsCreate := false
	var eligiblePlanErr error

	for _, sub := range subs {
		if !subscriptionQualifiesForActivity(sub, planTitles[sub.PlanID], user.ID, cfg) {
			continue
		}
		plan, err := planLoginCardForSubscription(user, sub, planTitles[sub.PlanID], cfg, quotaUnit)
		if err != nil {
			if eligiblePlanErr == nil {
				eligiblePlanErr = err
			}
			continue
		}
		card, ok, err := lookupCardBySubscriptionID(sub.ID)
		if err != nil {
			return Card{}, err
		}
		if ok {
			if card.TotalSpins-card.UsedSpins > 0 {
				return card, nil
			}
			if !hasExhausted {
				exhausted = card
				hasExhausted = true
			}
			continue
		}
		if !needsCreate {
			createPlan = plan
			needsCreate = true
		}
	}

	if needsCreate {
		return createSubscriptionCard(createPlan)
	}
	if hasExhausted {
		return exhausted, nil
	}
	if eligiblePlanErr != nil {
		return Card{}, eligiblePlanErr
	}
	return Card{}, httpErr{http.StatusForbidden, "该用户没有活动期内生效的有效订阅"}
}

func subscriptionQualifiesForActivity(sub subscriptionSummary, planTitle string, userID int64, cfg activity.Config) bool {
	if sub.ID <= 0 || sub.UserID != userID {
		return false
	}
	if isRewardSubscriptionPlanTitle(planTitle) {
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(sub.Status), "active") {
		return false
	}
	if sub.StartTime < cfg.StartTS || sub.StartTime > cfg.EndTS {
		return false
	}
	if sub.AmountTotal <= 0 {
		return false
	}
	return true
}

func createSubscriptionCard(plan loginCardPlan) (Card, error) {
	for attempt := 0; attempt < 5; attempt++ {
		plan.CardKey = generateSubscriptionCardKey()
		if err := insertLoginCard(plan); err != nil {
			if existing, ok, lookupErr := lookupCardBySubscriptionID(plan.SubscriptionID); lookupErr == nil && ok {
				return existing, nil
			}
			if attempt == 4 {
				return Card{}, err
			}
			continue
		}
		card, ok, err := lookupCardBySubscriptionID(plan.SubscriptionID)
		if err != nil {
			return Card{}, err
		}
		if !ok {
			return Card{}, httpErr{http.StatusInternalServerError, "服务器错误"}
		}
		return card, nil
	}
	return Card{}, httpErr{http.StatusInternalServerError, "服务器错误"}
}

func generateSubscriptionCardKey() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return "actsub-" + hex.EncodeToString(b[:])
}
