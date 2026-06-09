package combine

func buildNewMergeTokenBody(uniqueName string, target mergeTargetPlan, intervalUnit int) map[string]any {
	return map[string]any{
		"name":              uniqueName,
		"remain_quota":      target.Quota,
		"unlimited_quota":   false,
		"expired_time":      -1,
		"group":             target.Group,
		"interval_quota":    target.Quota,
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
