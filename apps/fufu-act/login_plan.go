package activityapp

import (
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"
	"unicode"

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

func planLoginCardForSubscription(user subscriptionUpstreamUser, sub subscriptionSummary, planTitle string, cfg activity.Config, quotaUnit int64) (loginCardPlan, error) {
	quotaUnit = quotaUnitOrDefault(quotaUnit)
	intervalQuota := sub.AmountTotal
	if dollars, ok := subscriptionSaleCardDollarsFromPlanTitle(planTitle); ok {
		intervalQuota = int64(math.Round(dollars * float64(quotaUnit)))
	}
	result := activity.PlanLoginCard(activity.LoginCardPlanInput{
		CardKey:       fmt.Sprintf("subscription-%d", sub.ID),
		Name:          user.Username,
		Status:        subscriptionStatusForPlan(sub.Status),
		IntervalQuota: intervalQuota,
		CreatedTime:   sub.StartTime,
		Config:        cfg,
		QuotaUnit:     quotaUnit,
	})
	switch result.Rejection {
	case "":
	case activity.LoginCardDisabled, activity.LoginCardOutsideWindow:
		return loginCardPlan{}, httpErr{http.StatusForbidden, "该用户没有活动期内生效的有效订阅"}
	default:
		return loginCardPlan{}, httpErr{http.StatusForbidden, "该订阅额度未匹配活动次数卡档位"}
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

type subscriptionPlanTitleCandidate struct {
	Token   string
	Dollars float64
}

func subscriptionSaleCardDollarsFromPlanTitle(planTitle string) (float64, bool) {
	title := normalizeSubscriptionPlanTitleToken(planTitle)
	if title == "" {
		return 0, false
	}
	candidates := []subscriptionPlanTitleCandidate{}
	for _, plan := range saleCardPlanTemplates() {
		for _, token := range []string{plan.Name, plan.Remark, plan.ID, plan.TokenNameSlug} {
			token = normalizeSubscriptionPlanTitleToken(token)
			if token != "" {
				candidates = append(candidates, subscriptionPlanTitleCandidate{Token: token, Dollars: plan.Quota})
			}
		}
		for _, token := range subscriptionQuotaTitleAliases(plan.Quota) {
			token = normalizeSubscriptionPlanTitleToken(token)
			if token != "" {
				candidates = append(candidates, subscriptionPlanTitleCandidate{Token: token, Dollars: plan.Quota})
			}
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if len(candidates[i].Token) != len(candidates[j].Token) {
			return len(candidates[i].Token) > len(candidates[j].Token)
		}
		return candidates[i].Dollars > candidates[j].Dollars
	})
	for _, candidate := range candidates {
		if title == candidate.Token || strings.Contains(title, candidate.Token) {
			return candidate.Dollars, true
		}
	}
	return 0, false
}

func subscriptionQuotaTitleAliases(dollars float64) []string {
	quota := int(math.Round(dollars))
	if quota <= 0 || math.Abs(float64(quota)-dollars) > 0.001 {
		return nil
	}
	aliases := []string{
		fmt.Sprintf("%d次卡", quota),
		fmt.Sprintf("%d次", quota),
	}
	switch quota {
	case 55:
		aliases = append(aliases, "五十五次卡", "五十五次")
	case 100:
		aliases = append(aliases, "一百次卡", "一百次", "百次卡", "百次")
	case 150:
		aliases = append(aliases, "一百五十次卡", "一百五十次")
	case 300:
		aliases = append(aliases, "三百次卡", "三百次")
	case 500:
		aliases = append(aliases, "五百次卡", "五百次")
	case 1000:
		aliases = append(aliases, "一千次卡", "一千次", "千次卡", "千次")
	}
	return aliases
}

func normalizeSubscriptionPlanTitleToken(value string) string {
	return strings.Map(func(r rune) rune {
		if r >= '０' && r <= '９' {
			return r - '０' + '0'
		}
		if unicode.IsSpace(r) {
			return -1
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, strings.TrimSpace(value))
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
