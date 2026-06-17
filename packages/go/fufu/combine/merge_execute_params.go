package combine

import "fufu/combinecore"

func buildExecuteMergeCardParams(p ExecuteMergeParams, onProgress func(MergeJobPatch)) MergeCardParams {
	validate := func(tokens []ResolvedToken) error {
		return validateExecuteMergeRequest(p, tokens)
	}
	plan := combinecore.BuildExecuteMergeCardPlan(coreExecuteMergeParams(p))
	return MergeCardParams{
		Keys:         plan.Keys,
		IntervalUnit: plan.IntervalUnit,
		Quota:        plan.Quota,
		Name:         plan.Name,
		Role:         p.Role,
		JobID:        plan.JobID,
		Validate:     validate,
		OnProgress:   onProgress,
	}
}
