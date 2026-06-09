package combine

import (
	"strings"
	"testing"
)

func TestFormatMergeDeletionFailureReturnsNilWithoutFailures(t *testing.T) {
	if err := formatMergeDeletionFailure(nil, 0, nil); err != nil {
		t.Fatalf("formatMergeDeletionFailure err = %v", err)
	}
}

func TestFormatMergeDeletionFailureReportsFullFailureWithRollback(t *testing.T) {
	rb := rollbackState{succeeded: true, note: "已回滚新卡"}
	err := formatMergeDeletionFailure([]string{"sk-source"}, 0, &rb)

	if err == nil || !strings.Contains(err.Error(), "旧卡删除失败：sk-source") || !strings.Contains(err.Error(), "未删除任何旧卡，已回滚新卡") {
		t.Fatalf("full rollback err = %v", err)
	}
}

func TestFormatMergeDeletionFailureReportsRollbackFailure(t *testing.T) {
	rb := rollbackState{succeeded: false, note: "新卡回滚失败（旧卡删除失败）：upstream down"}
	err := formatMergeDeletionFailure([]string{"sk-source"}, 0, &rb)

	if err == nil || !strings.Contains(err.Error(), "新卡回滚失败，请立即人工检查") || !strings.Contains(err.Error(), rb.note) {
		t.Fatalf("rollback failure err = %v", err)
	}
}

func TestFormatMergeDeletionFailureReportsPartialDeletion(t *testing.T) {
	err := formatMergeDeletionFailure([]string{"sk-a", "sk-b"}, 1, nil)

	if err == nil || !strings.Contains(err.Error(), "旧卡删除不完整：sk-a、sk-b") || !strings.Contains(err.Error(), "已保留新卡") {
		t.Fatalf("partial deletion err = %v", err)
	}
}
