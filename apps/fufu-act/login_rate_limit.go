package activityapp

import (
	"fufu/webutil"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	unknownLoginFailureLimit   = 5
	unknownLoginClientLimit    = 10
	unknownLoginFailureWindow  = 5 * time.Minute
	unknownLoginFailureLockout = time.Minute
)

var unknownLoginLimiter = newLoginUnknownRateLimiter()

type loginUnknownRateLimiter struct {
	mu             sync.Mutex
	cardFailures   map[string]loginUnknownFailureRecord
	clientFailures map[string]loginUnknownFailureRecord
}

type loginUnknownFailureRecord struct {
	Count        int
	FirstAttempt time.Time
	BlockedUntil time.Time
}

func newLoginUnknownRateLimiter() *loginUnknownRateLimiter {
	return &loginUnknownRateLimiter{
		cardFailures:   map[string]loginUnknownFailureRecord{},
		clientFailures: map[string]loginUnknownFailureRecord{},
	}
}

func (l *loginUnknownRateLimiter) allow(client, cardKey string, now time.Time) (time.Time, bool) {
	if l == nil {
		return time.Time{}, true
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	clientKey := unknownLoginClientKey(client)
	if blockedUntil, blocked := l.blockedUntil(l.clientFailures, clientKey, now); blocked {
		return blockedUntil, false
	}
	cardFailureKey := unknownLoginFailureKey(client, cardKey)
	if blockedUntil, blocked := l.blockedUntil(l.cardFailures, cardFailureKey, now); blocked {
		return blockedUntil, false
	}
	return time.Time{}, true
}

func (l *loginUnknownRateLimiter) recordUnknown(client, cardKey string, now time.Time) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.record(l.clientFailures, unknownLoginClientKey(client), now, unknownLoginClientLimit)
	l.record(l.cardFailures, unknownLoginFailureKey(client, cardKey), now, unknownLoginFailureLimit)
}

func (l *loginUnknownRateLimiter) clear(client, cardKey string) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.cardFailures, unknownLoginFailureKey(client, cardKey))
}

func (l *loginUnknownRateLimiter) blockedUntil(records map[string]loginUnknownFailureRecord, key string, now time.Time) (time.Time, bool) {
	rec := records[key]
	if !rec.BlockedUntil.IsZero() && now.Before(rec.BlockedUntil) {
		return rec.BlockedUntil, true
	}
	if !rec.FirstAttempt.IsZero() && now.Sub(rec.FirstAttempt) > unknownLoginFailureWindow {
		delete(records, key)
	}
	return time.Time{}, false
}

func (l *loginUnknownRateLimiter) record(records map[string]loginUnknownFailureRecord, key string, now time.Time, limit int) {
	rec := records[key]
	if rec.FirstAttempt.IsZero() || now.Sub(rec.FirstAttempt) > unknownLoginFailureWindow {
		rec = loginUnknownFailureRecord{FirstAttempt: now}
	}
	rec.Count++
	if rec.Count >= limit {
		rec.BlockedUntil = now.Add(unknownLoginFailureLockout)
	}
	records[key] = rec
}

func unknownLoginFailureKey(client, cardKey string) string {
	return unknownLoginClientKey(client) + "\x00" + strings.TrimSpace(cardKey)
}

func unknownLoginClientKey(client string) string {
	return strings.TrimSpace(client)
}

func loginClientIP(r *http.Request) string {
	return webutil.ClientIP(r)
}

func writeUnknownLoginRateLimited(w http.ResponseWriter, until time.Time) {
	retryAfter := int(time.Until(until).Seconds())
	if retryAfter < 1 {
		retryAfter = int(unknownLoginFailureLockout / time.Second)
	}
	w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
	writeJSONError(w, http.StatusTooManyRequests, "登录尝试过多，请稍后重试")
}
