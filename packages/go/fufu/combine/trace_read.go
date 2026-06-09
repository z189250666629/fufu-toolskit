package combine

import (
	"context"
	"database/sql"
	"sort"
	"strings"
)

func (a *App) traceResultsForKeys(ctx context.Context, rawKeys []string) ([]TraceResult, error) {
	if a.db == nil {
		return []TraceResult{}, nil
	}
	hashes, hashSet := uniqueTraceKeyHashes(rawKeys)
	if len(hashes) == 0 {
		return []TraceResult{}, nil
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
	trace.Direction = traceDirectionFromMatches(matchedSource, matchedResult)
	return trace, nil
}
