package combine

import (
	"strconv"
	"testing"
	"time"
)

func TestCleanMergeJobsKeepsRecentNonTerminalJobs(t *testing.T) {
	recent := time.Now().Add(-time.Minute).UnixMilli()
	old := time.Now().Add(-mergeJobTTL - time.Minute).UnixMilli()
	app := &App{mergeJobs: map[string]MergeJob{
		"queued":  {CreatedAt: recent, UpdatedAt: recent, Status: "queued", Role: RoleGuest},
		"running": {CreatedAt: recent, UpdatedAt: recent, Status: "deleting", Role: RoleGuest},
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

func TestCleanMergeJobsMarksStaleActiveJobsExpired(t *testing.T) {
	old := time.Now().Add(-mergeJobTTL - time.Minute).UnixMilli()
	app := &App{mergeJobs: map[string]MergeJob{}}
	for i := 0; i < maxActiveMergeJobs; i++ {
		app.mergeJobs["active-"+strconv.Itoa(i)] = MergeJob{CreatedAt: old, UpdatedAt: old, Status: "queued", Role: RoleGuest}
	}

	app.cleanMergeJobs()

	if got := app.activeMergeJobCount(); got != 0 {
		t.Fatalf("stale active jobs should not count against the active limit, got %d", got)
	}
	for id, job := range app.mergeJobs {
		if job.Status != "error" {
			t.Fatalf("%s stale active job status = %q, want error", id, job.Status)
		}
		if job.Error == "" {
			t.Fatalf("%s stale active job should record a safe timeout error: %#v", id, job)
		}
	}
	if ok := app.tryQueueMergeJob("new", buildQueuedMergeJobPatch(1, RoleGuest, "准备合并...")); !ok {
		t.Fatalf("stale active jobs should be released before enforcing the active limit: %#v", app.mergeJobs)
	}
}

func TestActiveMergeJobCountIgnoresTerminalJobs(t *testing.T) {
	app := &App{mergeJobs: map[string]MergeJob{
		"queued": {Status: "queued"},
		"done":   {Status: "done"},
		"error":  {Status: "error"},
	}}

	if got := app.activeMergeJobCount(); got != 1 {
		t.Fatalf("active job count = %d, want 1", got)
	}
}
