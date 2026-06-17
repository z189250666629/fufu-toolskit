package activityapp

import (
	"net/http"

	"fufu/activity"
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
