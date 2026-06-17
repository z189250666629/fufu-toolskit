package combinecore

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

const (
	PublicSourceUnit = 3
	PublicTargetUnit = 8
)

type Role string

const (
	RoleAdmin Role = "admin"
	RoleUser  Role = "user"
	RoleGuest Role = "guest"
)

type ResolvedToken struct {
	ID           int
	Key          string
	DisplayKey   string
	RemainQuota  int64
	UsedQuota    int64
	IntervalUnit int
	Group        string
	Status       int
}

type PublicMergeEligibility struct {
	Eligible bool
	Reasons  []string
}

type ExecuteMergeParams struct {
	Keys         []string
	IntervalUnit int
	TotalQuota   *int64
	Name         string
	CustomQuota  bool
	Role         Role
	JobID        string
}

type MergeTargetPlan struct {
	Quota int64
	Name  string
	Group string
}

type MergeCardPlan struct {
	Keys         []string
	IntervalUnit int
	Quota        *int64
	Name         string
	Role         Role
	JobID        string
}

func ValidateExecuteMergeRequest(p ExecuteMergeParams, tokens []ResolvedToken) error {
	for _, token := range tokens {
		if token.ID <= 0 {
			return fmt.Errorf("Token ID 无效：%s", tokenLabel(token))
		}
	}
	if !IsAllowedMergeIntervalUnit(p.IntervalUnit) {
		return errors.New("卡类型无效")
	}
	if p.Role == RoleGuest {
		if p.CustomQuota || p.TotalQuota != nil || strings.TrimSpace(p.Name) != "" {
			return errors.New("普通免登录合卡不支持指定额度或自定义命名")
		}
		if p.IntervalUnit != PublicTargetUnit {
			return errors.New("普通免登录合卡只支持合成周不刷新卡")
		}
		elig := EvaluatePublicMergeEligibility(tokens)
		if !elig.Eligible {
			return fmt.Errorf("普通免登录合卡仅支持未使用天卡 → 周不刷新卡：%s", strings.Join(elig.Reasons, "；"))
		}
	}
	if p.Role == RoleUser && len(tokens) == 1 && p.IntervalUnit != tokens[0].IntervalUnit {
		return errors.New("单卡续卡只能保持原卡类型")
	}
	if p.Role == RoleUser && len(tokens) > 1 {
		units := map[int]bool{}
		for _, t := range tokens {
			units[t.IntervalUnit] = true
		}
		allowed := map[int]bool{}
		if units[PublicSourceUnit] {
			allowed[PublicTargetUnit] = true
		}
		if units[PublicTargetUnit] {
			allowed[PublicTargetUnit] = true
		}
		if units[9] {
			allowed[9] = true
		}
		if !allowed[p.IntervalUnit] {
			return errors.New("当前卡组合不允许生成该类型的卡")
		}
	}
	if p.Role != RoleAdmin && p.CustomQuota {
		return errors.New("无权指定额度")
	}
	return nil
}

func IsAllowedMergeIntervalUnit(unit int) bool {
	return unit == PublicSourceUnit || unit == PublicTargetUnit || unit == 9
}

func EvaluatePublicMergeEligibility(tokens []ResolvedToken) PublicMergeEligibility {
	reasons := []string{}
	if len(tokens) < 2 {
		reasons = append(reasons, "至少需要 2 张天卡才能合卡")
	}
	for _, t := range tokens {
		label := tokenLabel(t)
		if t.Status != 1 {
			reasons = append(reasons, fmt.Sprintf("%s 已被禁用", label))
		}
		if t.IntervalUnit != PublicSourceUnit {
			reasons = append(reasons, fmt.Sprintf("%s 不是天卡", label))
		}
		if t.UsedQuota > 0 {
			reasons = append(reasons, fmt.Sprintf("%s 已经使用过", label))
		}
		if t.RemainQuota <= 0 {
			reasons = append(reasons, fmt.Sprintf("%s 没有剩余额度", label))
		}
	}
	return PublicMergeEligibility{Eligible: len(reasons) == 0, Reasons: reasons}
}

func BuildMergeTargetPlan(verified []ResolvedToken, quota *int64, name string, quotaUnit int64) (MergeTargetPlan, error) {
	finalQuota := int64(0)
	for _, token := range verified {
		finalQuota += token.RemainQuota
	}
	if quota != nil {
		finalQuota = *quota
	}
	if finalQuota <= 0 {
		return MergeTargetPlan{}, errors.New("合并额度无效")
	}
	if quotaUnit <= 0 {
		return MergeTargetPlan{}, errors.New("额度单位无效")
	}

	finalName := strings.TrimSpace(name)
	if finalName == "" {
		finalName = strconv.FormatInt(int64(math.Round(float64(finalQuota)/float64(quotaUnit))), 10)
	}

	return MergeTargetPlan{
		Quota: finalQuota,
		Name:  finalName,
		Group: MajorityGroup(verified),
	}, nil
}

func BuildExecuteMergeCardPlan(p ExecuteMergeParams) MergeCardPlan {
	var quota *int64
	if p.Role == RoleAdmin && p.CustomQuota {
		quota = p.TotalQuota
	}
	name := ""
	if p.Role != RoleGuest {
		name = strings.TrimSpace(p.Name)
	}
	return MergeCardPlan{
		Keys:         p.Keys,
		IntervalUnit: p.IntervalUnit,
		Quota:        quota,
		Name:         name,
		Role:         p.Role,
		JobID:        p.JobID,
	}
}

func MajorityGroup(tokens []ResolvedToken) string {
	counts := map[string]int{}
	for _, t := range tokens {
		g := t.Group
		if g == "" {
			g = "mix"
		}
		counts[g]++
	}
	groups := make([]string, 0, len(counts))
	for g := range counts {
		groups = append(groups, g)
	}
	sort.Slice(groups, func(i, j int) bool {
		if counts[groups[i]] == counts[groups[j]] {
			return groups[i] < groups[j]
		}
		return counts[groups[i]] > counts[groups[j]]
	})
	if len(groups) > 0 {
		return groups[0]
	}
	return "mix"
}

func tokenLabel(token ResolvedToken) string {
	if strings.TrimSpace(token.DisplayKey) != "" {
		return strings.TrimSpace(token.DisplayKey)
	}
	return strings.TrimSpace(token.Key)
}
