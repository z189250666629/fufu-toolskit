package combine

func buildRunMergeJobParams(jobID string, p MergePayload, role Role) ExecuteMergeParams {
	return ExecuteMergeParams{
		Keys:         p.Keys,
		IntervalUnit: p.IntervalUnit,
		TotalQuota:   p.TotalQuota,
		Name:         p.Name,
		CustomQuota:  p.CustomQuota,
		Role:         role,
		JobID:        jobID,
	}
}
