package activityapp

import (
	"fmt"
	"math"
	"net/http"
	"regexp"
	"sort"
	"strconv"
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
	Game             string
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
		Game:             plan.Game,
		Source:           plan.Source,
		PurchaseTime:     plan.PurchaseTime,
		PoolContribution: plan.PoolContribution,
	}, nil
}

func planLoginCardForSubscription(user subscriptionUpstreamUser, sub subscriptionSummary, planTitle string, cfg activity.Config, quotaUnit int64) (loginCardPlan, error) {
	quotaUnit = quotaUnitOrDefault(quotaUnit)
	intervalQuota := sub.AmountTotal
	if dollars, ok := subscriptionDollarsForPlan(sub, planTitle, cfg); ok {
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
		Game:             plan.Game,
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

func subscriptionDollarsForPlan(sub subscriptionSummary, planTitle string, cfg activity.Config) (float64, bool) {
	cfg = activity.NormalizeConfig(cfg)
	if dollars, ok := subscriptionMappedDollarsFromPlan(sub, planTitle, cfg.SubscriptionPlanMappings); ok {
		return dollars, true
	}
	return subscriptionSaleCardDollarsFromPlanTitle(planTitle)
}

func subscriptionMappedDollarsFromPlan(sub subscriptionSummary, planTitle string, mappings []activity.SubscriptionPlanMapping) (float64, bool) {
	title := normalizeSubscriptionPlanTitleToken(planTitle)
	for _, mapping := range mappings {
		if mapping.Dollars <= 0 {
			continue
		}
		if mapping.PlanID > 0 {
			if mapping.PlanID == sub.PlanID {
				return mapping.Dollars, true
			}
			continue
		}
		token := normalizeSubscriptionPlanTitleToken(mapping.Title)
		if token == "" || title == "" {
			continue
		}
		switch normalizeSubscriptionMappingMatch(mapping.Match) {
		case "exact":
			if title == token {
				return mapping.Dollars, true
			}
		default:
			if strings.Contains(title, token) {
				return mapping.Dollars, true
			}
		}
	}
	return 0, false
}

func normalizeSubscriptionMappingMatch(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "exact", "=", "eq", "equals", "full", "完全匹配":
		return "exact"
	default:
		return "contains"
	}
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
	candidates = append(candidates, subscriptionQuotaCandidatesFromPlanTitle(planTitle)...)
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

var subscriptionArabicQuotaPattern = regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?)(?:次(?:数)?卡?|times?|quota|credits?)`)

func subscriptionQuotaCandidatesFromPlanTitle(planTitle string) []subscriptionPlanTitleCandidate {
	title := normalizeSubscriptionPlanTitleToken(planTitle)
	if title == "" {
		return nil
	}
	candidates := []subscriptionPlanTitleCandidate{}
	for _, match := range subscriptionArabicQuotaPattern.FindAllStringSubmatch(title, -1) {
		if len(match) < 2 {
			continue
		}
		dollars, err := strconv.ParseFloat(match[1], 64)
		if err != nil || dollars <= 0 {
			continue
		}
		candidates = append(candidates, subscriptionPlanTitleCandidate{Token: match[0], Dollars: dollars})
	}
	candidates = append(candidates, subscriptionChineseQuotaCandidatesFromPlanTitle(title)...)
	return candidates
}

func subscriptionChineseQuotaCandidatesFromPlanTitle(title string) []subscriptionPlanTitleCandidate {
	runes := []rune(title)
	candidates := []subscriptionPlanTitleCandidate{}
	for idx, r := range runes {
		if r != '次' {
			continue
		}
		start := idx
		for start > 0 && isChineseQuotaNumeralRune(runes[start-1]) {
			start--
		}
		if start == idx {
			continue
		}
		value, ok := parseChineseQuotaNumeral(string(runes[start:idx]))
		if !ok || value <= 0 {
			continue
		}
		end := idx + 1
		if end < len(runes) && runes[end] == '卡' {
			end++
		}
		candidates = append(candidates, subscriptionPlanTitleCandidate{Token: string(runes[start:end]), Dollars: float64(value)})
	}
	return candidates
}

func isChineseQuotaNumeralRune(r rune) bool {
	switch r {
	case '零', '〇', '一', '二', '两', '三', '四', '五', '六', '七', '八', '九', '十', '百', '千', '万':
		return true
	default:
		return false
	}
}

func parseChineseQuotaNumeral(text string) (int, bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0, false
	}
	total := 0
	section := 0
	number := 0
	seen := false
	for _, r := range text {
		if value, ok := chineseQuotaDigitValue(r); ok {
			number = value
			seen = true
			continue
		}
		unit, ok := chineseQuotaUnitValue(r)
		if !ok {
			return 0, false
		}
		seen = true
		if unit == 10000 {
			section += number
			if section == 0 {
				section = 1
			}
			total += section * unit
			section = 0
			number = 0
			continue
		}
		if number == 0 {
			number = 1
		}
		section += number * unit
		number = 0
	}
	if !seen {
		return 0, false
	}
	result := total + section + number
	return result, result > 0
}

func chineseQuotaDigitValue(r rune) (int, bool) {
	switch r {
	case '零', '〇':
		return 0, true
	case '一':
		return 1, true
	case '二', '两':
		return 2, true
	case '三':
		return 3, true
	case '四':
		return 4, true
	case '五':
		return 5, true
	case '六':
		return 6, true
	case '七':
		return 7, true
	case '八':
		return 8, true
	case '九':
		return 9, true
	default:
		return 0, false
	}
}

func chineseQuotaUnitValue(r rune) (int, bool) {
	switch r {
	case '十':
		return 10, true
	case '百':
		return 100, true
	case '千':
		return 1000, true
	case '万':
		return 10000, true
	default:
		return 0, false
	}
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
