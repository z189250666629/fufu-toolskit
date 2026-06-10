package combine

import (
	"testing"
	"time"
)

func TestCleanMergeJobsKeepsNonTerminalJobs(t *testing.T) {
	old := time.Now().Add(-mergeJobTTL - time.Minute).UnixMilli()
	app := &App{mergeJobs: map[string]MergeJob{
		"queued":  {CreatedAt: old, UpdatedAt: old, Status: "queued", Role: RoleGuest},
		"running": {CreatedAt: old, UpdatedAt: old, Status: "deleting", Role: RoleGuest},
		"done":    {CreatedAt: old, UpdatedAt: old, Status: "done", Role: RoleGuest},
		"error":   {CreatedAt: old, UpdatedAt: old, Status: "error", Role: RoleGuest},
	}}

	app.cleanMergeJobs()

	if _, ok := app.mergeJobs["queued"]; !ok {
		t.Fatalf("queued job should not be cleaned while non-terminal")
	}
	if _, ok := app.mergeJobs["running"]; !ok {
		t.Fatalf("running job should not be cleaned while non-terminal")
	}
	if _, ok := app.mergeJobs["done"]; ok {
		t.Fatalf("done job should be cleaned after TTL")
	}
	if _, ok := app.mergeJobs["error"]; ok {
		t.Fatalf("error job should be cleaned after TTL")
	}
}
