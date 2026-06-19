package main

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	adminLoginFailureLimit   = 5
	adminLoginFailureWindow  = 5 * time.Minute
	adminLoginFailureLockout = time.Minute
)

var adminLoginLimiter = newAdminLoginRateLimiter()

type adminLoginRateLimiter struct {
	mu       sync.Mutex
	failures map[string]adminLoginFailureRecord
}

type adminLoginFailureRecord struct {
	Count        int
	FirstAttempt time.Time
	BlockedUntil time.Time
}

func newAdminLoginRateLimiter() *adminLoginRateLimiter {
	return &adminLoginRateLimiter{failures: map[string]adminLoginFailureRecord{}}
}

func resetAdminLoginLimiter() {
	adminLoginLimiter = newAdminLoginRateLimiter()
}

func (l *adminLoginRateLimiter) allow(client string, now time.Time) (time.Time, bool) {
	if l == nil {
		return time.Time{}, true
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	key := adminLoginFailureKey(client)
	rec := l.failures[key]
	if !rec.BlockedUntil.IsZero() && now.Before(rec.BlockedUntil) {
		return rec.BlockedUntil, false
	}
	if !rec.FirstAttempt.IsZero() && now.Sub(rec.FirstAttempt) > adminLoginFailureWindow {
		delete(l.failures, key)
	}
	return time.Time{}, true
}

func (l *adminLoginRateLimiter) recordFailure(client string, now time.Time) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	key := adminLoginFailureKey(client)
	rec := l.failures[key]
	if rec.FirstAttempt.IsZero() || now.Sub(rec.FirstAttempt) > adminLoginFailureWindow {
		rec = adminLoginFailureRecord{FirstAttempt: now}
	}
	rec.Count++
	if rec.Count >= adminLoginFailureLimit {
		rec.BlockedUntil = now.Add(adminLoginFailureLockout)
	}
	l.failures[key] = rec
}

func (l *adminLoginRateLimiter) clear(client string) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.failures, adminLoginFailureKey(client))
}

func adminLoginFailureKey(client string) string {
	return strings.TrimSpace(client)
}

func writeAdminLoginRateLimited(w http.ResponseWriter, until time.Time) {
	retryAfter := int(time.Until(until).Seconds())
	if retryAfter < 1 {
		retryAfter = int(adminLoginFailureLockout / time.Second)
	}
	w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
	writeJSONError(w, http.StatusTooManyRequests, "管理员登录尝试过多，请稍后重试")
}
