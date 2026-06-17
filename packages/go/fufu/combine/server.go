package combine

import (
	"context"
	"fufu/webutil"
	"net/http"
	"strings"
	"time"
)

func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		a.handleAPI(w, r)
		return
	}
	a.static.ServeHTTP(w, r)
}

func (a *App) ServeHTTPAsRole(w http.ResponseWriter, r *http.Request, role Role) {
	if role == "" {
		a.ServeHTTP(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/") {
		a.handleAPIAsRole(w, r, role)
		return
	}
	a.static.ServeHTTP(w, r)
}

func isPublicAPI(path string) bool {
	route, ok := findAPIPath(path)
	return ok && route.Public
}

func (a *App) handleAPI(w http.ResponseWriter, r *http.Request) {
	a.handleAPIAsRole(w, r, "")
}

func (a *App) handleAPIAsRole(w http.ResponseWriter, r *http.Request, trustedRole Role) {
	route, ok := findAPIPath(r.URL.Path)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Not found"})
		return
	}
	if r.Method != route.Method {
		webutil.RequireMethodMessage(w, r, route.Method, "Only "+route.Method)
		return
	}
	if trustedRole != "" {
		r = r.WithContext(context.WithValue(r.Context(), roleContextKey, trustedRole))
	} else if !isPublicAPI(r.URL.Path) {
		role, ok := a.authenticate(r)
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "未授权"})
			return
		}
		r = r.WithContext(context.WithValue(r.Context(), roleContextKey, role))
	}
	switch {
	case r.URL.Path == "/api/auth":
		a.handleAuth(w, r)
	case r.URL.Path == "/api/session":
		a.handleSession(w, r)
	case r.URL.Path == "/api/search-keys":
		a.handleSearchKeys(w, r)
	case r.URL.Path == "/api/merge":
		a.handleMerge(w, r)
	case r.URL.Path == "/api/public-merge":
		a.handlePublicMerge(w, r)
	case strings.HasPrefix(r.URL.Path, "/api/merge-status/"):
		a.handleMergeStatus(w, r)
	case r.URL.Path == "/api/generate":
		a.handleGenerate(w, r)
	case strings.HasPrefix(r.URL.Path, "/api/token/"):
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
