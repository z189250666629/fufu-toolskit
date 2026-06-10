package combine

import (
	"strconv"
	"strings"
)

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

func publicMergeJobStatus(job MergeJob) MergeJob {
	out := job
	out.Error = safeMergeJobError(out.Error)
	out.Result = publicMergeJobResult(out.Result)
	return out
}

func safeMergeJobError(message string) string {
	if strings.TrimSpace(message) == "" {
		return ""
	}
	return "合并失败，请稍后重试"
}

func publicMergeJobResult(result any) any {
	switch v := result.(type) {
	case MergeResult:
		return publicMergeResult(v)
	case *MergeResult:
		if v == nil {
			return v
		}
		out := publicMergeResult(*v)
		return &out
	default:
		return result
	}
}

func publicMergeResult(result MergeResult) MergeResult {
	out := result
	if len(result.DeleteResults) > 0 {
		out.DeleteResults = make([]DeleteResult, len(result.DeleteResults))
		for i, deleteResult := range result.DeleteResults {
			out.DeleteResults[i] = deleteResult
			out.DeleteResults[i].Key = keyMask(deleteResult.Key)
		}
	}
	return out
}

func buildSearchKeysResponse(keys []string, found []ResolvedToken, missing []string, quotaUnit int64, elapsedMs int64, traceResults []TraceResult) map[string]any {
	elig := evaluatePublicMergeEligibility(found)
	publicTraces := publicTraceResults(traceResults)
	return map[string]any{
		"found": found, "missing": missing, "quotaUnit": quotaUnit, "searched": len(keys),
		"concurrency": min(searchConcurrency, len(keys)), "elapsedMs": elapsedMs,
		"publicMergeEligibility": map[string]any{"eligible": elig.Eligible, "reasons": elig.Reasons, "targetUnit": publicTargetUnit},
		"traceResults":           publicTraces,
	}
}

func publicTraceResults(results []TraceResult) []TraceResult {
	if len(results) == 0 {
		return results
	}
	out := make([]TraceResult, len(results))
	for i, result := range results {
		out[i] = result
		out[i].Error = publicTraceError(result.Error)
		out[i].RollbackNote = publicRollbackNote(result.RollbackNote)
		out[i].SourceKeys = publicTraceTokens(result.SourceKeys)
		if result.ResultKey != nil {
			token := publicTraceToken(*result.ResultKey)
			out[i].ResultKey = &token
		}
	}
	return out
}

func publicTraceTokens(tokens []TraceToken) []TraceToken {
	if len(tokens) == 0 {
		return tokens
	}
	out := make([]TraceToken, len(tokens))
	for i, token := range tokens {
		out[i] = publicTraceToken(token)
	}
	return out
}

func publicTraceToken(token TraceToken) TraceToken {
	if token.KeyMask != "" {
		token.Key = token.KeyMask
	} else {
		token.Key = keyMask(token.Key)
		token.KeyMask = token.Key
	}
	token.KeyHash = ""
	token.DeleteError = publicTraceDeleteError(token.DeleteError)
	return token
}

func publicTraceError(message string) string {
	if strings.TrimSpace(message) == "" {
		return ""
	}
	return "合并失败，请稍后重试"
}

func publicRollbackNote(message string) string {
	if strings.TrimSpace(message) == "" {
		return ""
	}
	return "回滚状态已记录"
}

func publicTraceDeleteError(message string) string {
	if strings.TrimSpace(message) == "" {
		return ""
	}
	return "删除失败，请稍后重试"
}

func canDeleteTokenRole(role Role) bool {
	return role == RoleAdmin || role == RoleUser
}

func deleteTokenIDFromPath(path string) (int, bool) {
	id, err := strconv.Atoi(strings.TrimPrefix(path, "/api/token/"))
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}
