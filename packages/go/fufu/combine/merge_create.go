package combine

func buildNewMergeTokenBody(uniqueName string, target mergeTargetPlan, intervalUnit int) map[string]any {
	return buildTokenCreateBody(uniqueName, target.Quota, target.Group, intervalUnit)
}

func buildTokenCreateBody(uniqueName string, quota int64, group string, intervalUnit int) map[string]any {
	return map[string]any{
		"name":              uniqueName,
		"remain_quota":      quota,
		"unlimited_quota":   false,
		"expired_time":      -1,
		"group":             group,
		"interval_quota":    quota,
		"interval_time":     -1,
		"trigger_last_time": 0,
		"interval_unit":     intervalUnit,
	}
}

func buildMergeResult(newCard map[string]any, target mergeTargetPlan, fallbackIntervalUnit int, deleteResults []DeleteResult) MergeResult {
	return MergeResult{
		Success: true,
		NewCard: NewCardResult{
			Key:          ensureFullKey(getString(newCard, "key")),
			Name:         getString(newCard, "name"),
			RemainQuota:  int64OrDefault(toInt64(newCard["remain_quota"]), target.Quota),
			IntervalUnit: intOrDefault(toInt(newCard["interval_unit"]), fallbackIntervalUnit),
			Group:        stringOrDefault(getString(newCard, "group"), target.Group),
		},
		DeleteResults: deleteResults,
	}
}
