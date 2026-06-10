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
		if !base.Add(mergeJobTTL).Before(now) {
			continue
		}
		if isTerminalMergeJobStatus(job.Status) {
			delete(a.mergeJobs, id)
			continue
		}
		job.Status = "error"
		job.StepText = "合并超时"
		job.Error = "合并任务超时，请重试"
		job.UpdatedAt = now.UnixMilli()
		a.mergeJobs[id] = job
	}
}

func isTerminalMergeJobStatus(status string) bool {
	return status == "done" || status == "error"
}

func (a *App) activeMergeJobCount() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return activeMergeJobCountLocked(a.mergeJobs)
}

func activeMergeJobCountLocked(jobs map[string]MergeJob) int {
	count := 0
	for _, job := range jobs {
		if !isTerminalMergeJobStatus(job.Status) {
			count++
		}
	}
	return count
}

func (a *App) tryQueueMergeJob(jobID string, p MergeJobPatch) bool {
	nowMs := time.Now().UnixMilli()
	a.mu.Lock()
	defer a.mu.Unlock()
	if activeMergeJobCountLocked(a.mergeJobs) >= maxActiveMergeJobs {
		return false
	}
	job := MergeJob{CreatedAt: nowMs, Status: "queued"}
	applyMergeJobPatch(&job, p, nowMs)
	a.mergeJobs[jobID] = job
	return true
}

func (a *App) setMergeJob(jobID string, p MergeJobPatch) {
	nowMs := time.Now().UnixMilli()
	a.mu.Lock()
	defer a.mu.Unlock()
	job, ok := a.mergeJobs[jobID]
	if !ok {
		job = MergeJob{CreatedAt: nowMs, Status: "queued"}
	}
	applyMergeJobPatch(&job, p, nowMs)
	a.mergeJobs[jobID] = job
}

func applyMergeJobPatch(job *MergeJob, p MergeJobPatch, nowMs int64) {
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
