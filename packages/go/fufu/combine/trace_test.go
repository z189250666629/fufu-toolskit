package combine

import (
	"context"
	"database/sql"
	"testing"
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
