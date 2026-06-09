package combine

func buildMergePhasePatch(status, stepText string, total int) MergeJobPatch {
	return MergeJobPatch{
		Status:   strp(status),
		StepText: strp(stepText),
		Total:    intp(total),
		Current:  intp(0),
	}
}
