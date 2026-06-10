package main

import (
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	unknownLoginFailureLimit   = 5
	unknownLoginFailureWindow  = 5 * time.Minute
	unknownLoginFailureLockout = time.Minute
)

var unknownLoginLimiter = newLoginUnknownRateLimiter()

type loginUnknownRateLimiter struct {
	mu       sync.Mutex
	failures map[string]loginUnknownFailureRecord
}

type loginUnknownFailureRecord struct {
	Count        int
	FirstAttempt time.Time
	BlockedUntil time.Time
}

func newLoginUnknownRateLimiter() *loginUnknownRateLimiter {
	return &loginUnknownRateLimiter{failures: map[string]loginUnknownFailureRecord{}}
}

func (l *loginUnknownRateLimiter) allow(client, cardKey string, now time.Time) (time.Time, bool) {
	if l == nil {
		return time.Time{}, true
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	key := unknownLoginFailureKey(client, cardKey)
	rec := l.failures[key]
	if !rec.BlockedUntil.IsZero() && now.Before(rec.BlockedUntil) {
		return rec.BlockedUntil, false
	}
	if !rec.FirstAttempt.IsZero() && now.Sub(rec.FirstAttempt) > unknownLoginFailureWindow {
		delete(l.failures, key)
	}
	return time.Time{}, true
}

func (l *loginUnknownRateLimiter) recordUnknown(client, cardKey string, now time.Time) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	key := unknownLoginFailureKey(client, cardKey)
	rec := l.failures[key]
	if rec.FirstAttempt.IsZero() || now.Sub(rec.FirstAttempt) > unknownLoginFailureWindow {
		rec = loginUnknownFailureRecord{FirstAttempt: now}
	}
	rec.Count++
	if rec.Count >= unknownLoginFailureLimit {
		rec.BlockedUntil = now.Add(unknownLoginFailureLockout)
	}
	l.failures[key] = rec
}

func (l *loginUnknownRateLimiter) clear(client, cardKey string) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.failures, unknownLoginFailureKey(client, cardKey))
}

func unknownLoginFailureKey(client, cardKey string) string {
	return strings.TrimSpace(client) + "\x00" + strings.TrimSpace(cardKey)
}

func loginClientIP(r *http.Request) string {
	for _, header := range []string{"Cf-Connecting-Ip", "X-Real-Ip", "X-Forwarded-For"} {
		value := strings.TrimSpace(r.Header.Get(header))
		if value != "" {
			return strings.TrimSpace(strings.Split(value, ",")[0])
		}
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil && host != "" {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}

func writeUnknownLoginRateLimited(w http.ResponseWriter, until time.Time) {
	retryAfter := int(time.Until(until).Seconds())
	if retryAfter < 1 {
		retryAfter = int(unknownLoginFailureLockout / time.Second)
	}
	w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
	writeJSONError(w, http.StatusTooManyRequests, "登录尝试过多，请稍后重试")
}
