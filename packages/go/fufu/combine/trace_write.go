package combine

import (
	"context"
	"log"
	"time"
)

func (a *App) createMergeTrace(ctx context.Context, jobID string, role Role, intervalUnit int) (int64, error) {
	if a.db == nil {
		return 0, nil
	}
	now := time.Now()
	if err := a.pruneExpiredTraceRows(ctx, now); err != nil {
		log.Printf("trace retention cleanup failed: %v", err)
	}
	nowMs := now.UnixMilli()
	res, err := a.db.ExecContext(ctx, `
		INSERT INTO merge_records (job_id, role, status, requested_interval_unit, created_at, updated_at)
		VALUES (?, ?, 'started', ?, ?, ?)
	`, jobID, string(role), intervalUnit, nowMs, nowMs)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (a *App) pruneExpiredTraceRows(ctx context.Context, now time.Time) error {
	if a.db == nil {
		return nil
	}
	cutoff := now.Add(-traceRetention).UnixMilli()
	if _, err := a.db.ExecContext(ctx, `
		DELETE FROM merge_tokens
		WHERE merge_id IN (
			SELECT id FROM merge_records
			WHERE completed_at IS NOT NULL AND completed_at < ?
		)
	`, cutoff); err != nil {
		return err
	}
	if _, err := a.db.ExecContext(ctx, `
		DELETE FROM merge_records
		WHERE completed_at IS NOT NULL AND completed_at < ?
	`, cutoff); err != nil {
		return err
	}
	return nil
}

func (a *App) setTraceStatus(ctx context.Context, mergeID int64, status string) {
	if a.db == nil || mergeID == 0 {
		return
	}
	if _, err := a.db.ExecContext(ctx, `UPDATE merge_records SET status = ?, updated_at = ? WHERE id = ?`, status, time.Now().UnixMilli(), mergeID); err != nil {
		log.Printf("trace status update failed: %v", err)
	}
}

func (a *App) setTraceFinal(ctx context.Context, mergeID int64, quota int64, name, group string) {
	if a.db == nil || mergeID == 0 {
		return
	}
	if _, err := a.db.ExecContext(ctx, `
		UPDATE merge_records
		SET final_quota = ?, final_name = ?, final_group = ?, updated_at = ?
		WHERE id = ?
	`, quota, name, group, time.Now().UnixMilli(), mergeID); err != nil {
		log.Printf("trace final update failed: %v", err)
	}
}

func (a *App) setTraceCreatedCard(ctx context.Context, mergeID int64, cardID int) {
	if a.db == nil || mergeID == 0 {
		return
	}
	if _, err := a.db.ExecContext(ctx, `
		UPDATE merge_records SET created_card_id = ?, updated_at = ? WHERE id = ?
	`, cardID, time.Now().UnixMilli(), mergeID); err != nil {
		log.Printf("trace created card update failed: %v", err)
	}
}

func (a *App) setTraceRollback(ctx context.Context, mergeID int64, succeeded bool, note string) {
	if a.db == nil || mergeID == 0 {
		return
	}
	if _, err := a.db.ExecContext(ctx, `
		UPDATE merge_records
		SET rollback_attempted = 1, rollback_succeeded = ?, rollback_note = ?, updated_at = ?
		WHERE id = ?
	`, boolInt(succeeded), redactTraceDiagnostic(note), time.Now().UnixMilli(), mergeID); err != nil {
		log.Printf("trace rollback update failed: %v", err)
	}
}

func (a *App) setTraceDeleteStarted(ctx context.Context, mergeID int64) {
	if a.db == nil || mergeID == 0 {
		return
	}
	if _, err := a.db.ExecContext(ctx, `
		UPDATE merge_records SET delete_started = 1, updated_at = ? WHERE id = ?
	`, time.Now().UnixMilli(), mergeID); err != nil {
		log.Printf("trace delete start update failed: %v", err)
	}
}

func (a *App) setTraceDeletedCount(ctx context.Context, mergeID int64, count int) {
	if a.db == nil || mergeID == 0 {
		return
	}
	if _, err := a.db.ExecContext(ctx, `
		UPDATE merge_records SET old_cards_deleted_count = ?, updated_at = ? WHERE id = ?
	`, count, time.Now().UnixMilli(), mergeID); err != nil {
		log.Printf("trace deleted count update failed: %v", err)
	}
}

func (a *App) finishTrace(ctx context.Context, mergeID int64, status, errText string) {
	if a.db == nil || mergeID == 0 {
		return
	}
	now := time.Now().UnixMilli()
	if _, err := a.db.ExecContext(ctx, `
		UPDATE merge_records
		SET status = ?, error = ?, updated_at = ?, completed_at = ?
		WHERE id = ?
	`, status, redactTraceDiagnostic(errText), now, now, mergeID); err != nil {
		log.Printf("trace finish update failed: %v", err)
	}
}

func (a *App) upsertTraceToken(ctx context.Context, mergeID int64, kind string, token ResolvedToken) error {
	if a.db == nil || mergeID == 0 {
		return nil
	}
	key := traceTokenKeys(token)
	now := time.Now().UnixMilli()
	_, err := a.db.ExecContext(ctx, `
		INSERT INTO merge_tokens (
			merge_id, kind, token_id, key_full, key_hash, key_mask, name,
			remain_quota, used_quota, interval_unit, group_name, status, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(merge_id, kind, key_hash) DO UPDATE SET
			token_id = excluded.token_id,
			key_full = excluded.key_full,
			key_mask = excluded.key_mask,
			name = excluded.name,
			remain_quota = excluded.remain_quota,
			used_quota = excluded.used_quota,
			interval_unit = excluded.interval_unit,
			group_name = excluded.group_name,
			status = excluded.status,
			updated_at = excluded.updated_at
	`, mergeID, kind, token.ID, key.mask, key.hash, key.mask, token.Name, token.RemainQuota, token.UsedQuota, token.IntervalUnit, token.Group, token.Status, now, now)
	return err
}

func (a *App) setTraceTokenDeleteResult(ctx context.Context, mergeID int64, token ResolvedToken, ok bool, errText string) {
	if a.db == nil || mergeID == 0 {
		return
	}
	key := traceTokenKeys(token)
	if _, err := a.db.ExecContext(ctx, `
		UPDATE merge_tokens
		SET delete_ok = ?, delete_error = ?, updated_at = ?
		WHERE merge_id = ? AND kind = 'source' AND key_hash = ?
	`, boolInt(ok), redactTraceDiagnostic(errText), time.Now().UnixMilli(), mergeID, key.hash); err != nil {
		log.Printf("trace token delete update failed: %v", err)
	}
}
