package combine

import (
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (a *App) authBlockedUntil(r *http.Request, now time.Time) (time.Time, bool) {
	key := authClientKey(r)
	a.mu.Lock()
	defer a.mu.Unlock()
	rec, ok := a.authFailures[key]
	if !ok {
		return time.Time{}, false
	}
	if rec.BlockedUntil.After(now) {
		return rec.BlockedUntil, true
	}
	if !rec.BlockedUntil.IsZero() || now.Sub(rec.FirstAttempt) > authFailureWindow {
		delete(a.authFailures, key)
	}
	return time.Time{}, false
}

func (a *App) recordAuthFailure(r *http.Request, now time.Time) {
	key := authClientKey(r)
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.authFailures == nil {
		a.authFailures = map[string]authFailureRecord{}
	}
	rec := a.authFailures[key]
	if rec.FirstAttempt.IsZero() || now.Sub(rec.FirstAttempt) > authFailureWindow {
		rec = authFailureRecord{FirstAttempt: now}
	}
	rec.Count++
	if rec.Count >= authFailureLimit {
		rec.BlockedUntil = now.Add(authFailureLockout)
	}
	a.authFailures[key] = rec
}

func (a *App) clearAuthFailures(r *http.Request) {
	key := authClientKey(r)
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.authFailures, key)
}

func authClientKey(r *http.Request) string {
	for _, header := range []string{"Cf-Connecting-Ip", "X-Real-Ip", "X-Forwarded-For"} {
		if value := strings.TrimSpace(r.Header.Get(header)); value != "" {
			return strings.TrimSpace(strings.Split(value, ",")[0])
		}
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil && host != "" {
		return host
	}
	if strings.TrimSpace(r.RemoteAddr) != "" {
		return strings.TrimSpace(r.RemoteAddr)
	}
	return "unknown"
}

func retryAfterSeconds(until, now time.Time) string {
	seconds := int(until.Sub(now).Seconds())
	if seconds < 1 {
		seconds = 1
	}
	return strconv.Itoa(seconds)
}
