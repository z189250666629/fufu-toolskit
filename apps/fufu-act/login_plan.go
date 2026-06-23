package activityapp

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"fufu/activity"
	"fufu/newapi"
	"fufu/poolfundcore"
	"fufu/tokens"
)

type loginCardPlan struct {
	CardKey          string
	CardName         string
	Dollars          float64
	TotalSpins       int
	Source           string
	PurchaseTime     string
	PoolContribution poolfundcore.ContributionResult
	SubscriptionID   int64
	UserID           int64
	Username         string
}

type subscriptionUpstreamUser struct {
	ID       int64
	Username string
}

type subscriptionSummary struct {
	ID          int64
	UserID      int64
	PlanID      int64
	AmountTotal int64
	AmountUsed  int64
	StartTime   int64
	EndTime     int64
	Status      string
}

func planLoginCardForToken(key string, t *tokens.Token, shop ShopPurchaseLookup, cfg activity.Config, quotaUnit int64) (loginCardPlan, error) {
	if t == nil {
		return loginCardPlan{}, httpErr{http.StatusNotFound, "卡密不存在"}
	}

	result := activity.PlanLoginCard(activity.LoginCardPlanInput{
		CardKey:          key,
		Name:             t.Name,
		Status:           t.Status,
		IntervalQuota:    t.IntervalQuota,
		CreatedTime:      t.CreatedTime,
		ShopPurchaseTime: shop.PurchaseTime,
		Config:           cfg,
		QuotaUnit:        quotaUnit,
	})
	switch result.Rejection {
	case "":
	case activity.LoginCardDisabled:
		return loginCardPlan{}, httpErr{http.StatusForbidden, "此卡密已被禁用，无法参与活动"}
	case activity.LoginCardOutsideWindow:
		return loginCardPlan{}, httpErr{http.StatusForbidden, "此卡密不在活动期间内，不参与活动"}
	default:
		return loginCardPlan{}, httpErr{http.StatusForbidden, "此卡密额度不参与活动"}
	}
	plan := result.Plan
	return loginCardPlan{
		CardKey:          plan.CardKey,
		CardName:         plan.CardName,
		Dollars:          plan.Dollars,
		TotalSpins:       plan.TotalDraws,
		Source:           plan.Source,
		PurchaseTime:     plan.PurchaseTime,
		PoolContribution: plan.PoolContribution,
	}, nil
}

func planLoginCardForSubscription(user subscriptionUpstreamUser, sub subscriptionSummary, cfg activity.Config, quotaUnit int64) (loginCardPlan, error) {
	quotaUnit = quotaUnitOrDefault(quotaUnit)
	result := activity.PlanLoginCard(activity.LoginCardPlanInput{
		CardKey:       fmt.Sprintf("subscription-%d", sub.ID),
		Name:          user.Username,
		Status:        subscriptionStatusForPlan(sub.Status),
		IntervalQuota: sub.AmountTotal,
		CreatedTime:   sub.StartTime,
		Config:        cfg,
		QuotaUnit:     quotaUnit,
	})
	switch result.Rejection {
	case "":
	case activity.LoginCardDisabled, activity.LoginCardOutsideWindow:
		return loginCardPlan{}, httpErr{http.StatusForbidden, "该用户没有活动期内生效的有效订阅"}
	default:
		return fallbackSubscriptionLoginCardPlan(user, sub, cfg, quotaUnit)
	}
	plan := result.Plan
	return loginCardPlan{
		CardName:         firstNonEmpty(user.Username, fmt.Sprintf("user-%d", user.ID)),
		Dollars:          plan.Dollars,
		TotalSpins:       plan.TotalDraws,
		Source:           "subscription",
		PurchaseTime:     formatUnixText(sub.StartTime),
		PoolContribution: plan.PoolContribution,
		SubscriptionID:   sub.ID,
		UserID:           user.ID,
		Username:         user.Username,
	}, nil
}

func fallbackSubscriptionLoginCardPlan(user subscriptionUpstreamUser, sub subscriptionSummary, cfg activity.Config, quotaUnit int64) (loginCardPlan, error) {
	dollars := activity.DollarsTier(sub.AmountTotal, quotaUnit)
	if dollars <= 0 {
		return loginCardPlan{}, httpErr{http.StatusForbidden, "该用户没有可参与活动的订阅额度"}
	}
	totalDraws := cfg.DrawCountForTier(dollars)
	if totalDraws <= 0 {
		totalDraws = 1
	}
	contribution, _ := activity.DynamicPoolContributionForTier(cfg, dollars)
	return loginCardPlan{
		CardName:         firstNonEmpty(user.Username, fmt.Sprintf("user-%d", user.ID)),
		Dollars:          dollars,
		TotalSpins:       totalDraws,
		Source:           "subscription",
		PurchaseTime:     formatUnixText(sub.StartTime),
		PoolContribution: contribution,
		SubscriptionID:   sub.ID,
		UserID:           user.ID,
		Username:         user.Username,
	}, nil
}

func subscriptionStatusForPlan(status string) int {
	if strings.EqualFold(strings.TrimSpace(status), "active") {
		return 1
	}
	return 0
}

func formatUnixText(ts int64) string {
	if ts <= 0 {
		return ""
	}
	return time.Unix(ts, 0).Local().Format("2006-01-02 15:04:05")
}

func quotaUnitOrDefault(value int64) int64 {
	if value > 0 {
		return value
	}
	return newapi.DefaultQuotaUnit
}

// isScratchDollarTier reports whether a card of the given dollar tier plays the
// scratch game; the scratch tiers are admin-configurable.
func isScratchDollarTier(dollars float64) bool {
	return SnapshotRuntimeConfig().IsScratchTier(dollars)
}

func parseActTestTokenName(name string) (float64, bool) {
	return activity.ActTestDollarsFromName(name)
}

func isTestCardName(name string) bool {
	return activity.IsTestCardName(name)
}

func isTestCardSegment(value string) bool {
	return activity.IsTestCardSegment(value)
}
