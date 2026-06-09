package combine

import "strings"

func buildExecuteMergeCardParams(p ExecuteMergeParams, onProgress func(MergeJobPatch)) MergeCardParams {
	validate := func(tokens []ResolvedToken) error {
		return validateExecuteMergeRequest(p, tokens)
	}
	var quota *int64
	if p.Role == RoleAdmin && p.CustomQuota {
		quota = p.TotalQuota
	}
	name := ""
	if p.Role != RoleGuest {
		name = strings.TrimSpace(p.Name)
	}
	return MergeCardParams{
		Keys:         p.Keys,
		IntervalUnit: p.IntervalUnit,
		Quota:        quota,
		Name:         name,
		Role:         p.Role,
		JobID:        p.JobID,
		Validate:     validate,
		OnProgress:   onProgress,
	}
}
