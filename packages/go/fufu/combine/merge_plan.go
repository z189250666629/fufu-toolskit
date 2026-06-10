package combine

import (
	"errors"
	"math"
	"strconv"
	"strings"
)

type mergeTargetPlan struct {
	Quota int64
	Name  string
	Group string
}

func buildMergeTargetPlan(verified []ResolvedToken, quota *int64, name string, quotaUnit int64) (mergeTargetPlan, error) {
	finalQuota := int64(0)
	for _, token := range verified {
		finalQuota += token.RemainQuota
	}
	if quota != nil {
		finalQuota = *quota
	}
	if finalQuota <= 0 {
		return mergeTargetPlan{}, errors.New("合并额度无效")
	}
	if quotaUnit <= 0 {
		return mergeTargetPlan{}, errors.New("额度单位无效")
	}

	finalName := strings.TrimSpace(name)
	if finalName == "" {
		finalName = strconv.FormatInt(int64(math.Round(float64(finalQuota)/float64(quotaUnit))), 10)
	}

	return mergeTargetPlan{
		Quota: finalQuota,
		Name:  finalName,
		Group: majorityGroup(verified),
	}, nil
}
