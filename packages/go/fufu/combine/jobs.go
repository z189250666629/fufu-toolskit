package combine

import (
	"time"
)

func (a *App) cleanSessionsLocked(now time.Time) {
	for token, session := range a.sessions {
		if session.Expiry.Before(now) {
			delete(a.sessions, token)
		}
	}
}

func (a *App) cleanMergeJobs() {
	now := time.Now()
	a.mu.Lock()
	defer a.mu.Unlock()
	for id, job := range a.mergeJobs {
		base := time.UnixMilli(job.CreatedAt)
		if job.UpdatedAt != 0 {
			base = time.UnixMilli(job.UpdatedAt)
		}
		if base.Add(mergeJobTTL).Before(now) {
			delete(a.mergeJobs, id)
		}
	}
}

func (a *App) setMergeJob(jobID string, p MergeJobPatch) {
	nowMs := time.Now().UnixMilli()
	a.mu.Lock()
	defer a.mu.Unlock()
	job, ok := a.mergeJobs[jobID]
	if !ok {
		job = MergeJob{CreatedAt: nowMs, Status: "queued"}
	}
	job.UpdatedAt = nowMs
	if p.Status != nil {
		job.Status = *p.Status
	}
	if job.Status == "" {
		job.Status = "queued"
	}
	if p.StepText != nil {
		job.StepText = *p.StepText
	}
	if p.Current != nil {
		job.Current = p.Current
	}
	if p.Total != nil {
		job.Total = p.Total
	}
	if p.HasResult {
		job.Result = p.Result
	}
	if p.Error != nil {
		job.Error = *p.Error
	}
	if p.Role != nil {
		job.Role = *p.Role
	}
	if p.MergeID != nil {
		job.MergeID = *p.MergeID
	}
	a.mergeJobs[jobID] = job
}

func (a *App) getMergeJob(jobID string) (MergeJob, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	job, ok := a.mergeJobs[jobID]
	return job, ok
}

func (a *App) acquireMergeLock(ids []int) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, id := range ids {
		if _, ok := a.mergeLocks[id]; ok {
			return false
		}
	}
	for _, id := range ids {
		a.mergeLocks[id] = struct{}{}
	}
	return true
}

func (a *App) releaseMergeLock(ids []int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, id := range ids {
		delete(a.mergeLocks, id)
	}
}
