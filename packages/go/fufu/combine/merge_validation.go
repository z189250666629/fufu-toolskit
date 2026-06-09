package combine

import (
	"errors"
	"fmt"
	"strings"
)

func validateExecuteMergeRequest(p ExecuteMergeParams, tokens []ResolvedToken) error {
	if p.Role == RoleGuest {
		if p.CustomQuota || p.TotalQuota != nil || strings.TrimSpace(p.Name) != "" {
			return errors.New("普通免登录合卡不支持指定额度或自定义命名")
		}
		if p.IntervalUnit != publicTargetUnit {
			return errors.New("普通免登录合卡只支持合成周不刷新卡")
		}
		elig := evaluatePublicMergeEligibility(tokens)
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
		if units[3] {
			allowed[8] = true
		}
		if units[8] {
			allowed[8] = true
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
