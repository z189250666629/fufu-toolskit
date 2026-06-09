package combine

import (
	"errors"
	"strings"
	"testing"
)

func TestShouldAttemptMergeRollbackOnError(t *testing.T) {
	cases := []struct {
		name              string
		createdID         int
		mergeCompleted    bool
		rollbackAttempted bool
		deletionStarted   bool
		oldDeleted        int
		want              bool
	}{
		{name: "created before deletion", createdID: 10, want: true},
		{name: "created before deletion failure", createdID: 10, deletionStarted: true, oldDeleted: 0, want: true},
		{name: "nothing created", want: false},
		{name: "already completed", createdID: 10, mergeCompleted: true, want: false},
		{name: "already attempted", createdID: 10, rollbackAttempted: true, want: false},
		{name: "partial old deletion keeps new card", createdID: 10, deletionStarted: true, oldDeleted: 1, want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := shouldAttemptMergeRollbackOnError(tc.createdID, tc.mergeCompleted, tc.rollbackAttempted, tc.deletionStarted, tc.oldDeleted)
			if got != tc.want {
				t.Fatalf("shouldAttemptMergeRollbackOnError = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestAppendRollbackNotePreservesExistingMessage(t *testing.T) {
	err := appendRollbackNote(errors.New("新卡重命名失败"), rollbackState{attempted: true, succeeded: false, note: "回滚失败：upstream"})
	if err == nil || !strings.Contains(err.Error(), "新卡重命名失败 回滚失败：upstream") {
		t.Fatalf("append err = %v", err)
	}

	err = appendRollbackNote(errors.New("新卡重命名失败 回滚失败：upstream"), rollbackState{attempted: true, succeeded: false, note: "回滚失败：upstream"})
	if err == nil || strings.Count(err.Error(), "回滚失败：upstream") != 1 {
		t.Fatalf("duplicate note err = %v", err)
	}

	if got := appendRollbackNote(errors.New("原错误"), rollbackState{attempted: true, succeeded: true, note: "已回滚"}); got.Error() != "原错误" {
		t.Fatalf("successful rollback should not append note: %v", got)
	}
}
