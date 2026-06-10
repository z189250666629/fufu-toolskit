package combine

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"
)

func newTraceTestApp(t *testing.T) *App {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(traceSchemaSQL); err != nil {
		t.Fatalf("init trace schema: %v", err)
	}
	return &App{db: db}
}

func TestLoadTraceResultDirectionAndDeleteState(t *testing.T) {
	ctx := context.Background()
	app := newTraceTestApp(t)
	mergeID, err := app.createMergeTrace(ctx, "job-trace", RoleUser, 60)
	if err != nil {
		t.Fatalf("createMergeTrace: %v", err)
	}
	source := ResolvedToken{ID: 1, Key: "sk-source", Name: "source", RemainQuota: 10, Status: 1}
	result := ResolvedToken{ID: 2, Key: "sk-result", Name: "result", RemainQuota: 20, Status: 1}
	if err := app.upsertTraceToken(ctx, mergeID, "source", source); err != nil {
		t.Fatalf("upsert source token: %v", err)
	}
	if err := app.upsertTraceToken(ctx, mergeID, "result", result); err != nil {
		t.Fatalf("upsert result token: %v", err)
	}
	app.setTraceTokenDeleteResult(ctx, mergeID, source, false, "blocked")

	sourceHash := keyHash(source.Key)
	resultHash := keyHash(result.Key)
	for _, tc := range []struct {
		name      string
		hashes    map[string]bool
		direction string
	}{
		{name: "source", hashes: map[string]bool{sourceHash: true}, direction: "source"},
		{name: "result", hashes: map[string]bool{resultHash: true}, direction: "result"},
		{name: "both", hashes: map[string]bool{sourceHash: true, resultHash: true}, direction: "both"},
		{name: "related", hashes: map[string]bool{}, direction: "related"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			trace, err := app.loadTraceResult(ctx, mergeID, tc.hashes)
			if err != nil {
				t.Fatalf("loadTraceResult: %v", err)
			}
			if trace.Direction != tc.direction {
				t.Fatalf("direction = %q, want %q", trace.Direction, tc.direction)
			}
			if len(trace.SourceKeys) != 1 || trace.ResultKey == nil {
				t.Fatalf("trace tokens not loaded: %#v", trace)
			}
			if trace.SourceKeys[0].DeleteOK == nil || *trace.SourceKeys[0].DeleteOK {
				t.Fatalf("delete ok state = %#v", trace.SourceKeys[0].DeleteOK)
			}
			if trace.SourceKeys[0].DeleteError != "blocked" {
				t.Fatalf("delete error = %q", trace.SourceKeys[0].DeleteError)
			}
		})
	}
}

func TestGeneratedTokenCacheRoundTrip(t *testing.T) {
	ctx := context.Background()
	app := newTraceTestApp(t)
	token := ResolvedToken{ID: 77, Key: "sk-generated", Name: "generated", RemainQuota: 100, Status: 1}
	if err := app.upsertGeneratedToken(ctx, token); err != nil {
		t.Fatalf("upsertGeneratedToken: %v", err)
	}
	id, ok, err := app.generatedTokenIDByKey(ctx, token.Key)
	if err != nil {
		t.Fatalf("generatedTokenIDByKey: %v", err)
	}
	if !ok || id != token.ID {
		t.Fatalf("generated token lookup = (%d, %v), want (%d, true)", id, ok, token.ID)
	}

	app.deleteGeneratedTokenCacheByID(ctx, token.ID)
	id, ok, err = app.generatedTokenIDByKey(ctx, token.Key)
	if err != nil {
		t.Fatalf("generatedTokenIDByKey after delete: %v", err)
	}
	if ok || id != 0 {
		t.Fatalf("generated token lookup after delete = (%d, %v), want (0, false)", id, ok)
	}
}

func TestUpsertTraceTokenDoesNotPersistFullKey(t *testing.T) {
	ctx := context.Background()
	app := newTraceTestApp(t)
	mergeID, err := app.createMergeTrace(ctx, "job-secure-trace", RoleUser, 60)
	if err != nil {
		t.Fatalf("createMergeTrace: %v", err)
	}
	token := ResolvedToken{ID: 101, Key: "sk-source-secret-1234567890", Name: "source", RemainQuota: 10, Status: 1}

	if err := app.upsertTraceToken(ctx, mergeID, "source", token); err != nil {
		t.Fatalf("upsertTraceToken: %v", err)
	}

	var keyFull, keyHashValue, keyMaskValue string
	if err := app.db.QueryRowContext(ctx, `SELECT key_full, key_hash, key_mask FROM merge_tokens WHERE merge_id = ?`, mergeID).Scan(&keyFull, &keyHashValue, &keyMaskValue); err != nil {
		t.Fatalf("query trace token: %v", err)
	}
	if keyHashValue != keyHash(token.Key) {
		t.Fatalf("key hash = %q, want %q", keyHashValue, keyHash(token.Key))
	}
	if keyFull == token.Key || strings.Contains(keyFull, "source-secret") || strings.Contains(keyFull, "1234567890") {
		t.Fatalf("merge_tokens.key_full persisted full secret: %q", keyFull)
	}
	if keyFull != keyMaskValue {
		t.Fatalf("legacy key_full column should store the mask, got key_full=%q key_mask=%q", keyFull, keyMaskValue)
	}
}

func TestGeneratedTokenCacheDoesNotPersistRawJSONOrFullKey(t *testing.T) {
	ctx := context.Background()
	app := newTraceTestApp(t)
	token := ResolvedToken{
		ID:           89,
		Key:          "sk-generated-secret-1234567890",
		Name:         "generated",
		RemainQuota:  100,
		IntervalUnit: 60,
		Group:        "vip",
		Status:       1,
		Raw: map[string]any{
			"key":  "sk-generated-secret-1234567890",
			"name": "generated",
		},
	}

	if err := app.upsertGeneratedToken(ctx, token); err != nil {
		t.Fatalf("upsertGeneratedToken: %v", err)
	}

	var keyFull string
	var rawJSON sql.NullString
	if err := app.db.QueryRowContext(ctx, `SELECT key_full, raw_json FROM generated_tokens WHERE token_id = ?`, token.ID).Scan(&keyFull, &rawJSON); err != nil {
		t.Fatalf("query generated token: %v", err)
	}
	for _, leaked := range []string{token.Key, "generated-secret", "1234567890"} {
		if strings.Contains(keyFull, leaked) {
			t.Fatalf("generated_tokens.key_full persisted full secret %q in %q", leaked, keyFull)
		}
		if rawJSON.Valid && strings.Contains(rawJSON.String, leaked) {
			t.Fatalf("generated_tokens.raw_json persisted full secret %q in %q", leaked, rawJSON.String)
		}
	}
}

func TestTraceDiagnosticFieldsRedactSecretsBeforePersisting(t *testing.T) {
	ctx := context.Background()
	app := newTraceTestApp(t)
	mergeID, err := app.createMergeTrace(ctx, "job-diagnostic-redaction", RoleUser, 60)
	if err != nil {
		t.Fatalf("createMergeTrace: %v", err)
	}
	source := ResolvedToken{ID: 222, Key: "sk-source-secret-1234567890", Name: "source", RemainQuota: 10, Status: 1}
	if err := app.upsertTraceToken(ctx, mergeID, "source", source); err != nil {
		t.Fatalf("upsertTraceToken: %v", err)
	}
	rawDiagnostic := "Get https://internal.example.local/api/token/search?keyword=&token=source-secret-1234567890&p=0: Authorization: Bearer upstream-secret-token failed for sk-source-secret-1234567890"

	app.finishTrace(ctx, mergeID, "error", rawDiagnostic)
	app.setTraceRollback(ctx, mergeID, false, rawDiagnostic)
	app.setTraceTokenDeleteResult(ctx, mergeID, source, false, rawDiagnostic)

	var traceError, rollbackNote, deleteError string
	if err := app.db.QueryRowContext(ctx, `SELECT error, rollback_note FROM merge_records WHERE id = ?`, mergeID).Scan(&traceError, &rollbackNote); err != nil {
		t.Fatalf("query merge diagnostics: %v", err)
	}
	if err := app.db.QueryRowContext(ctx, `SELECT delete_error FROM merge_tokens WHERE merge_id = ? AND kind = 'source'`, mergeID).Scan(&deleteError); err != nil {
		t.Fatalf("query token diagnostics: %v", err)
	}
	for name, value := range map[string]string{
		"error":         traceError,
		"rollback_note": rollbackNote,
		"delete_error":  deleteError,
	} {
		for _, leaked := range []string{"sk-source-secret-1234567890", "source-secret-1234567890", "internal.example.local", "upstream-secret-token"} {
			if strings.Contains(value, leaked) {
				t.Fatalf("%s persisted sensitive diagnostic %q in %q", name, leaked, value)
			}
		}
		if !strings.Contains(value, "sk-sour…7890") {
			t.Fatalf("%s should keep masked key context, got %q", name, value)
		}
	}
}

func TestCreateMergeTracePrunesExpiredTraceRows(t *testing.T) {
	ctx := context.Background()
	app := newTraceTestApp(t)
	oldMergeID, err := app.createMergeTrace(ctx, "job-old-trace", RoleUser, 60)
	if err != nil {
		t.Fatalf("create old trace: %v", err)
	}
	recentMergeID, err := app.createMergeTrace(ctx, "job-recent-trace", RoleUser, 60)
	if err != nil {
		t.Fatalf("create recent trace: %v", err)
	}
	oldToken := ResolvedToken{ID: 301, Key: "sk-old-trace-key-1234567890", Name: "old", Status: 1}
	recentToken := ResolvedToken{ID: 302, Key: "sk-recent-trace-key-1234567890", Name: "recent", Status: 1}
	if err := app.upsertTraceToken(ctx, oldMergeID, "source", oldToken); err != nil {
		t.Fatalf("upsert old token: %v", err)
	}
	if err := app.upsertTraceToken(ctx, recentMergeID, "source", recentToken); err != nil {
		t.Fatalf("upsert recent token: %v", err)
	}
	oldMs := time.Now().Add(-31 * 24 * time.Hour).UnixMilli()
	recentMs := time.Now().Add(-time.Hour).UnixMilli()
	if _, err := app.db.ExecContext(ctx, `UPDATE merge_records SET status = 'done', created_at = ?, updated_at = ?, completed_at = ? WHERE id = ?`, oldMs, oldMs, oldMs, oldMergeID); err != nil {
		t.Fatalf("age old trace: %v", err)
	}
	if _, err := app.db.ExecContext(ctx, `UPDATE merge_records SET status = 'done', created_at = ?, updated_at = ?, completed_at = ? WHERE id = ?`, recentMs, recentMs, recentMs, recentMergeID); err != nil {
		t.Fatalf("age recent trace: %v", err)
	}

	if _, err := app.createMergeTrace(ctx, "job-new-trace", RoleUser, 60); err != nil {
		t.Fatalf("create new trace: %v", err)
	}

	if got := countTraceRows(t, app, `SELECT COUNT(*) FROM merge_records WHERE id = ?`, oldMergeID); got != 0 {
		t.Fatalf("expired merge record count = %d, want 0", got)
	}
	if got := countTraceRows(t, app, `SELECT COUNT(*) FROM merge_tokens WHERE merge_id = ?`, oldMergeID); got != 0 {
		t.Fatalf("expired trace token count = %d, want 0", got)
	}
	if got := countTraceRows(t, app, `SELECT COUNT(*) FROM merge_records WHERE id = ?`, recentMergeID); got != 1 {
		t.Fatalf("recent merge record count = %d, want 1", got)
	}
	if got := countTraceRows(t, app, `SELECT COUNT(*) FROM merge_tokens WHERE merge_id = ?`, recentMergeID); got != 1 {
		t.Fatalf("recent trace token count = %d, want 1", got)
	}
}

func countTraceRows(t *testing.T, app *App, query string, args ...any) int {
	t.Helper()
	var count int
	if err := app.db.QueryRow(query, args...).Scan(&count); err != nil {
		t.Fatalf("count trace rows: %v", err)
	}
	return count
}

func TestGeneratedTokenCacheIgnoresUnserializableRawSnapshot(t *testing.T) {
	ctx := context.Background()
	app := newTraceTestApp(t)
	token := ResolvedToken{ID: 88, Key: "sk-bad-generated", Name: "bad", RemainQuota: 100, Status: 1, Raw: map[string]any{"bad": func() {}}}

	if err := app.upsertGeneratedToken(ctx, token); err != nil {
		t.Fatalf("raw snapshot should not block generated token cache: %v", err)
	}
	if id, ok, err := app.generatedTokenIDByKey(ctx, token.Key); err != nil || !ok || id != token.ID {
		t.Fatalf("generated token lookup with unserializable raw = (%d, %v, %v), want (%d, true, nil)", id, ok, err, token.ID)
	}
}
