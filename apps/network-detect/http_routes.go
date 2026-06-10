package main

import (
	"fufu/config"
	"fufu/newapi"
	"fufu/webutil"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

func route(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if x := recover(); x != nil {
			writeJSONError(w, 500, "Internal server error")
		}
	}()
	path := r.URL.Path
	if strings.HasPrefix(path, "/api/") {
		handleAPI(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeJSONError(w, 405, "Only GET is supported")
		return
	}
	if path == "/combine" || path == "/combine/" {
		serveFile(w, r, filepath.Join(combineDir, "index.html"), true)
		return
	}
	serveStatic(w, r, path)
}

func handleAPI(w http.ResponseWriter, r *http.Request) {
	if isCombineAPI(r.URL.Path) {
		if combineApp == nil {
			writeJSONError(w, 503, "combine is not configured: "+errString(combineConfigErr))
			return
		}
		combineApp.ServeHTTP(w, r)
		return
	}
	path := r.URL.Path
	switch {
	case path == "/api/health":
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true})
	case path == "/api/client":
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		writeJSON(w, 200, map[string]any{"ip": clientIP(r), "serverTime": time.Now().UnixMilli(), "origin": r.Header.Get("Origin"), "userAgent": r.UserAgent()})
	case path == "/api/connectivity/targets":
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		writeJSON(w, 200, map[string]any{"groups": connectivityGroups()})
	case path == "/api/newapi/sites":
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		sites, msg := config.LoadManagedSites(rootDir)
		publics := []newapi.PublicSite{}
		for _, s := range sites {
			publics = append(publics, s.Public())
		}
		status := 200
		if msg != "" && len(sites) == 0 {
			status = 500
		}
		writeJSON(w, status, map[string]any{"configured": len(sites) > 0, "error": msg, "sites": publics})
	case path == "/api/newapi/model-status":
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		force := r.URL.Query().Get("refresh") == "1"
		status := getModelStatus(force)
		code := 200
		if status.ConfigError != "" && !status.Configured {
			code = 500
		}
		writeJSON(w, code, status)
	case path == "/api/newapi/overview":
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		overview := buildOverview(r.URL.Query())
		writeJSON(w, 200, overview)
	case path == "/api/newapi/model-status/test":
		if !requireMethod(w, r, http.MethodPost) {
			return
		}
		handleModelTest(w, r)
	default:
		writeJSONError(w, 404, "API not found")
	}
}

func requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	return webutil.RequireMethodMessage(w, r, method, "Only "+method+" is supported")
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	webutil.WriteJSON(w, status, payload)
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	webutil.WriteJSONError(w, status, message)
}

func readJSON(r *http.Request, out any) error {
	return webutil.DecodeJSON(http.MaxBytesReader(nil, r.Body, 1<<20), out, webutil.WithUseNumber())
}

func clientIP(r *http.Request) string {
	for _, h := range []string{"Cf-Connecting-Ip", "X-Real-Ip", "X-Forwarded-For"} {
		v := strings.TrimSpace(r.Header.Get(h))
		if v != "" {
			return strings.TrimSpace(strings.Split(v, ",")[0])
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}
