package combine

import "fmt"

func evaluatePublicMergeEligibility(tokens []ResolvedToken) PublicMergeEligibility {
	reasons := []string{}
	if len(tokens) < 2 {
		reasons = append(reasons, "至少需要 2 张天卡才能合卡")
	}
	for _, t := range tokens {
		if t.Status != 1 {
			reasons = append(reasons, fmt.Sprintf("%s 已被禁用", displayKey(t.Key)))
		}
		if t.IntervalUnit != publicSourceUnit {
			reasons = append(reasons, fmt.Sprintf("%s 不是天卡", displayKey(t.Key)))
		}
		if t.UsedQuota > 0 {
			reasons = append(reasons, fmt.Sprintf("%s 已经使用过", displayKey(t.Key)))
		}
		if t.RemainQuota <= 0 {
			reasons = append(reasons, fmt.Sprintf("%s 没有剩余额度", displayKey(t.Key)))
		}
	}
	return PublicMergeEligibility{Eligible: len(reasons) == 0, Reasons: reasons}
}
