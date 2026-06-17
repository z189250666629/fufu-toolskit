package combine

import (
	"fufu/combinecore"
)

type mergeTargetPlan struct {
	Quota int64
	Name  string
	Group string
}

func buildMergeTargetPlan(verified []ResolvedToken, quota *int64, name string, quotaUnit int64) (mergeTargetPlan, error) {
	plan, err := combinecore.BuildMergeTargetPlan(coreResolvedTokens(verified), quota, name, quotaUnit)
	if err != nil {
		return mergeTargetPlan{}, err
	}
	return mergeTargetPlan{Quota: plan.Quota, Name: plan.Name, Group: plan.Group}, nil
}
