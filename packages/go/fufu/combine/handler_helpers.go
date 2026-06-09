package combine

import "strings"

func mergeStatusJobIDFromPath(path string) string {
	return strings.TrimPrefix(path, "/api/merge-status/")
}

func buildQueuedMergeJobPatch(total int, role Role, stepText string) MergeJobPatch {
	return MergeJobPatch{
		Status:   strp("queued"),
		StepText: strp(stepText),
		Current:  intp(0),
		Total:    intp(total),
		Role:     &role,
	}
}

func buildMergeAcceptedResponse(jobID string) map[string]any {
	return map[string]any{"ok": true, "jobId": jobID}
}
