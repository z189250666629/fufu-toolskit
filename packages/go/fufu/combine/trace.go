package combine

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"sort"
	"strings"
	"time"
)

func (a *App) createMergeTrace(ctx context.Context, jobID string, role Role, intervalUnit int) (int64, error) {
	if a.db == nil {
		return 0, nil
	}
	now := time.Now().UnixMilli()
	res, err := a.db.ExecContext(ctx, `
		INSERT INTO merge_records (job_id, role, status, requested_interval_unit, created_at, updated_at)
		VALUES (?, ?, 'started', ?, ?, ?)
	`, jobID, string(role), intervalUnit, now, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
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
	`, boolInt(succeeded), note, time.Now().UnixMilli(), mergeID); err != nil {
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
	`, status, errText, now, now, mergeID); err != nil {
		log.Printf("trace finish update failed: %v", err)
	}
}

func (a *App) upsertTraceToken(ctx context.Context, mergeID int64, kind string, token ResolvedToken) error {
	if a.db == nil || mergeID == 0 {
		return nil
	}
	key := ensureFullKey(token.Key)
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
	`, mergeID, kind, token.ID, key, keyHash(key), keyMask(key), token.Name, token.RemainQuota, token.UsedQuota, token.IntervalUnit, token.Group, token.Status, now, now)
	return err
}

func (a *App) setTraceTokenDeleteResult(ctx context.Context, mergeID int64, token ResolvedToken, ok bool, errText string) {
	if a.db == nil || mergeID == 0 {
		return
	}
	if _, err := a.db.ExecContext(ctx, `
		UPDATE merge_tokens
		SET delete_ok = ?, delete_error = ?, updated_at = ?
		WHERE merge_id = ? AND kind = 'source' AND key_hash = ?
	`, boolInt(ok), errText, time.Now().UnixMilli(), mergeID, keyHash(token.Key)); err != nil {
		log.Printf("trace token delete update failed: %v", err)
	}
}

func (a *App) traceResultsForKeys(ctx context.Context, rawKeys []string) ([]TraceResult, error) {
	if a.db == nil {
		return []TraceResult{}, nil
	}
	keys := normalizeKeys(rawKeys)
	if len(keys) == 0 {
		return []TraceResult{}, nil
	}
	hashSet := map[string]bool{}
	hashes := []string{}
	for _, key := range keys {
		hash := keyHash(key)
		if !hashSet[hash] {
			hashSet[hash] = true
			hashes = append(hashes, hash)
		}
	}

	seenHashes := map[string]bool{}
	for hash := range hashSet {
		seenHashes[hash] = true
	}
	seenMergeIDs := map[int64]bool{}
	mergeIDs := []int64{}
	frontier := hashes

	for len(frontier) > 0 && len(mergeIDs) < maxTraceRecords {
		ids, err := a.traceMergeIDsForHashes(ctx, frontier, maxTraceRecords-len(mergeIDs))
		if err != nil {
			return nil, err
		}
		newIDs := []int64{}
		for _, id := range ids {
			if seenMergeIDs[id] {
				continue
			}
			seenMergeIDs[id] = true
			mergeIDs = append(mergeIDs, id)
			newIDs = append(newIDs, id)
		}
		if len(newIDs) == 0 {
			break
		}

		relatedHashes, err := a.traceKeyHashesForMergeIDs(ctx, newIDs)
		if err != nil {
			return nil, err
		}
		next := []string{}
		for _, hash := range relatedHashes {
			if seenHashes[hash] {
				continue
			}
			seenHashes[hash] = true
			next = append(next, hash)
		}
		frontier = next
	}

	results := []TraceResult{}
	for _, mergeID := range mergeIDs {
		trace, err := a.loadTraceResult(ctx, mergeID, hashSet)
		if err != nil {
			return nil, err
		}
		results = append(results, trace)
	}
	sort.SliceStable(results, func(i, j int) bool {
		return results[i].CreatedAt < results[j].CreatedAt
	})
	return results, nil
}

func (a *App) traceMergeIDsForHashes(ctx context.Context, hashes []string, limit int) ([]int64, error) {
	if len(hashes) == 0 || limit <= 0 {
		return nil, nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(hashes)), ",")
	args := make([]any, 0, len(hashes)+1)
	for _, hash := range hashes {
		args = append(args, hash)
	}
	args = append(args, limit)
	rows, err := a.db.QueryContext(ctx, `
		SELECT DISTINCT r.id, r.created_at
		FROM merge_records r
		JOIN merge_tokens t ON t.merge_id = r.id
		WHERE t.key_hash IN (`+placeholders+`)
		ORDER BY r.created_at ASC
		LIMIT ?
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := []int64{}
	for rows.Next() {
		var id int64
		var createdAt int64
		if err := rows.Scan(&id, &createdAt); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (a *App) traceKeyHashesForMergeIDs(ctx context.Context, mergeIDs []int64) ([]string, error) {
	if len(mergeIDs) == 0 {
		return nil, nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(mergeIDs)), ",")
	args := make([]any, 0, len(mergeIDs))
	for _, id := range mergeIDs {
		args = append(args, id)
	}
	rows, err := a.db.QueryContext(ctx, `
		SELECT DISTINCT key_hash
		FROM merge_tokens
		WHERE merge_id IN (`+placeholders+`)
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	hashes := []string{}
	for rows.Next() {
		var hash string
		if err := rows.Scan(&hash); err != nil {
			return nil, err
		}
		if strings.TrimSpace(hash) != "" {
			hashes = append(hashes, hash)
		}
	}
	return hashes, rows.Err()
}

func (a *App) loadTraceResult(ctx context.Context, mergeID int64, queryHashes map[string]bool) (TraceResult, error) {
	var trace TraceResult
	var role string
	var completedAt sql.NullInt64
	err := a.db.QueryRowContext(ctx, `
		SELECT id, COALESCE(job_id, ''), role, status, created_at, updated_at, completed_at,
		       COALESCE(final_quota, 0), COALESCE(requested_interval_unit, 0),
		       COALESCE(final_name, ''), COALESCE(final_group, ''),
		       COALESCE(error, ''), COALESCE(rollback_note, '')
		FROM merge_records
		WHERE id = ?
	`, mergeID).Scan(&trace.MergeID, &trace.JobID, &role, &trace.Status, &trace.CreatedAt, &trace.UpdatedAt, &completedAt, &trace.FinalQuota, &trace.IntervalUnit, &trace.FinalName, &trace.FinalGroup, &trace.Error, &trace.RollbackNote)
	if err != nil {
		return TraceResult{}, err
	}
	trace.Role = Role(role)
	if completedAt.Valid {
		trace.CompletedAt = &completedAt.Int64
	}

	rows, err := a.db.QueryContext(ctx, `
		SELECT kind, COALESCE(token_id, 0), key_full, key_hash, key_mask,
		       COALESCE(name, ''), COALESCE(remain_quota, 0), COALESCE(used_quota, 0),
		       COALESCE(interval_unit, 0), COALESCE(group_name, ''), COALESCE(status, 0),
		       delete_ok, COALESCE(delete_error, '')
		FROM merge_tokens
		WHERE merge_id = ?
		ORDER BY CASE kind WHEN 'source' THEN 0 ELSE 1 END, id
	`, mergeID)
	if err != nil {
		return TraceResult{}, err
	}
	defer rows.Close()

	matchedSource := false
	matchedResult := false
	for rows.Next() {
		var kind string
		var token TraceToken
		var hash string
		var deleteOK sql.NullInt64
		if err := rows.Scan(&kind, &token.TokenID, &token.Key, &hash, &token.KeyMask, &token.Name, &token.RemainQuota, &token.UsedQuota, &token.IntervalUnit, &token.Group, &token.Status, &deleteOK, &token.DeleteError); err != nil {
			return TraceResult{}, err
		}
		token.KeyHash = hash
		if deleteOK.Valid {
			ok := deleteOK.Int64 == 1
			token.DeleteOK = &ok
		}
		if kind == "source" {
			trace.SourceKeys = append(trace.SourceKeys, token)
			matchedSource = matchedSource || queryHashes[hash]
		} else if kind == "result" {
			trace.ResultKey = &token
			matchedResult = matchedResult || queryHashes[hash]
		}
	}
	if err := rows.Err(); err != nil {
		return TraceResult{}, err
	}
	switch {
	case matchedSource && matchedResult:
		trace.Direction = "both"
	case matchedResult:
		trace.Direction = "result"
	case matchedSource:
		trace.Direction = "source"
	default:
		trace.Direction = "related"
	}
	return trace, nil
}

func (a *App) upsertGeneratedToken(ctx context.Context, token ResolvedToken) error {
	if a.db == nil || token.ID == 0 || strings.TrimSpace(token.Key) == "" {
		return nil
	}
	key := ensureFullKey(token.Key)
	rawJSON, _ := json.Marshal(token.Raw)
	now := time.Now().UnixMilli()
	_, err := a.db.ExecContext(ctx, `
		INSERT INTO generated_tokens (
			token_id, key_full, key_hash, key_mask, name, remain_quota, used_quota,
			interval_unit, group_name, status, raw_json, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(key_hash) DO UPDATE SET
			token_id = excluded.token_id,
			key_full = excluded.key_full,
			key_mask = excluded.key_mask,
			name = excluded.name,
			remain_quota = excluded.remain_quota,
			used_quota = excluded.used_quota,
			interval_unit = excluded.interval_unit,
			group_name = excluded.group_name,
			status = excluded.status,
			raw_json = excluded.raw_json,
			updated_at = excluded.updated_at
	`, token.ID, key, keyHash(key), keyMask(key), token.Name, token.RemainQuota, token.UsedQuota, token.IntervalUnit, token.Group, token.Status, string(rawJSON), now, now)
	return err
}

func (a *App) generatedTokenIDByKey(ctx context.Context, key string) (int, bool, error) {
	if a.db == nil {
		return 0, false, nil
	}
	var id int
	err := a.db.QueryRowContext(ctx, `SELECT token_id FROM generated_tokens WHERE key_hash = ?`, keyHash(key)).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return id, id > 0, nil
}

func (a *App) deleteGeneratedTokenCacheByID(ctx context.Context, tokenID int) {
	if a.db == nil || tokenID == 0 {
		return
	}
	if _, err := a.db.ExecContext(ctx, `DELETE FROM generated_tokens WHERE token_id = ?`, tokenID); err != nil {
		log.Printf("generated token cache delete failed: %v", err)
	}
}
