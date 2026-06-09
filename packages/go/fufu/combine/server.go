package combine

import (
	"context"
	"net/http"
	"strings"
	"time"
)

func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		a.handleAPI(w, r)
		return
	}
	if r.URL.Path == "/" || strings.HasSuffix(r.URL.Path, ".html") {
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	}
	a.static.ServeHTTP(w, r)
}

func isPublicAPI(path string) bool {
	return path == "/api/auth" || path == "/api/search-keys" || path == "/api/public-merge" || strings.HasPrefix(path, "/api/merge-status/")
}

func (a *App) handleAPI(w http.ResponseWriter, r *http.Request) {
	if !isPublicAPI(r.URL.Path) {
		role, ok := a.authenticate(r)
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "未授权"})
			return
		}
		r = r.WithContext(context.WithValue(r.Context(), roleContextKey, role))
	}
	switch {
	case r.URL.Path == "/api/auth" && r.Method == http.MethodPost:
		a.handleAuth(w, r)
	case r.URL.Path == "/api/session" && r.Method == http.MethodGet:
		a.handleSession(w, r)
	case r.URL.Path == "/api/search-keys" && r.Method == http.MethodPost:
		a.handleSearchKeys(w, r)
	case r.URL.Path == "/api/merge" && r.Method == http.MethodPost:
		a.handleMerge(w, r)
	case r.URL.Path == "/api/public-merge" && r.Method == http.MethodPost:
		a.handlePublicMerge(w, r)
	case strings.HasPrefix(r.URL.Path, "/api/merge-status/") && r.Method == http.MethodGet:
		a.handleMergeStatus(w, r)
	case r.URL.Path == "/api/generate" && r.Method == http.MethodPost:
		a.handleGenerate(w, r)
	case strings.HasPrefix(r.URL.Path, "/api/token/") && r.Method == http.MethodDelete:
		a.handleDeleteToken(w, r)
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Not found"})
	}
}

func (a *App) authenticate(r *http.Request) (Role, bool) {
	token := r.Header.Get("X-Session-Token")
	if token == "" {
		return "", false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	s, ok := a.sessions[token]
	if !ok || s.Expiry.Before(time.Now()) {
		if ok {
			delete(a.sessions, token)
		}
		return "", false
	}
	return s.Role, true
}

func roleFromContext(ctx context.Context) Role {
	role, _ := ctx.Value(roleContextKey).(Role)
	return role
}
