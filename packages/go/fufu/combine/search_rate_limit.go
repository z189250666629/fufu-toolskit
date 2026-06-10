package combine

import (
	"net/http"
	"time"
)

func (a *App) tryBeginClientSearch(r *http.Request) (string, bool) {
	key := authClientKey(r)
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.activeSearches == nil {
		a.activeSearches = map[string]struct{}{}
	}
	if _, ok := a.activeSearches[key]; ok {
		return key, false
	}
	a.activeSearches[key] = struct{}{}
	return key, true
}

func (a *App) endClientSearch(key string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.activeSearches, key)
}

func (a *App) allowClientSearchRequest(r *http.Request, now time.Time) (time.Time, bool) {
	key := authClientKey(r)
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.searchRequests == nil {
		a.searchRequests = map[string]searchRequestRecord{}
	}
	pruneSearchRequestRecordsLocked(a.searchRequests, now)
	rec := a.searchRequests[key]
	if rec.WindowStart.IsZero() || !now.Before(rec.WindowStart.Add(searchRequestWindow)) {
		rec = searchRequestRecord{WindowStart: now}
	}
	if rec.Count >= maxSearchRequestsPerClientWindow {
		until := rec.WindowStart.Add(searchRequestWindow)
		a.searchRequests[key] = rec
		return until, false
	}
	rec.Count++
	a.searchRequests[key] = rec
	return time.Time{}, true
}

func pruneSearchRequestRecordsLocked(records map[string]searchRequestRecord, now time.Time) {
	for key, rec := range records {
		if rec.WindowStart.IsZero() || !now.Before(rec.WindowStart.Add(searchRequestWindow)) {
			delete(records, key)
		}
	}
}
