package combine

import (
	"fmt"
	"strings"
)

func shouldAttemptMergeRollbackOnError(createdID int, mergeCompleted, rollbackAttempted, deletionStarted bool, oldDeleted int) bool {
	return createdID != 0 && !mergeCompleted && !rollbackAttempted && (!deletionStarted || oldDeleted == 0)
}

func appendRollbackNote(err error, rb rollbackState) error {
	if err == nil {
		return nil
	}
	if !rb.attempted || rb.succeeded || rb.note == "" || strings.Contains(err.Error(), rb.note) {
		return err
	}
	return fmt.Errorf("%s %s", strings.TrimSpace(err.Error()), rb.note)
}
