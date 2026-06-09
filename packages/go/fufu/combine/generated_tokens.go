package combine

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"time"
)

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
