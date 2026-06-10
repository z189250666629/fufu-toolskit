package combine

import "net/http"

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
