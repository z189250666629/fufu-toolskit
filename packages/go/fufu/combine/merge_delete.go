package combine

import (
	"fmt"
	"strings"
)

type mergeDeleteOutcome struct {
	result       DeleteResult
	failureKey   string
	traceMessage string
}

func buildMergeDeleteOutcome(token ResolvedToken, ok bool, res APIResponse, delErr error) mergeDeleteOutcome {
	success := ok && delErr == nil
	out := mergeDeleteOutcome{
		result: DeleteResult{ID: token.ID, Key: token.Key, OK: success},
	}
	if success {
		return out
	}
	if delErr != nil {
		out.traceMessage = delErr.Error()
	} else {
		out.traceMessage = upstreamStatusMessage(res, "删除失败")
	}
	out.failureKey = displayKey(token.Key)
	return out
}

func formatMergeDeletionFailure(deleteFailures []string, oldDeleted int, rb *rollbackState) error {
	if len(deleteFailures) == 0 {
		return nil
	}
	failed := strings.Join(deleteFailures, "、")
	if oldDeleted > 0 {
		return fmt.Errorf("旧卡删除不完整：%s。已保留新卡以避免额度丢失，请立即人工清理剩余旧卡。", failed)
	}
	if rb != nil && rb.succeeded {
		return fmt.Errorf("旧卡删除失败：%s。未删除任何旧卡，已回滚新卡。", failed)
	}
	note := ""
	if rb != nil {
		note = rb.note
	}
	return fmt.Errorf("旧卡删除失败：%s。新卡回滚失败，请立即人工检查。%s", failed, note)
}
