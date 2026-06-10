package main

import (
	"errors"
	"fufu/config"
	"fufu/newapi"
	"fufu/webutil"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

const maxNetworkJSONBodyBytes int64 = 1 << 20

var errRequestBodyTooLarge = errors.New("request body too large")

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
		w.Header().Set("Allow", "GET, HEAD")
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
			writeJSONError(w, 503, "combine is not configured")
			return
		}
		combineApp.ServeHTTP(w, r)
		return
	}
	path := r.URL.Path
	route, ok := findNetworkAPIPath(path)
	if !ok {
		writeJSONError(w, 404, "API not found")
		return
	}
	if !requireMethod(w, r, route.Method) {
		return
	}

	switch path {
	case "/api/health":
		writeJSON(w, 200, map[string]any{"ok": true})
	case "/api/client":
		writeJSON(w, 200, map[string]any{"ip": clientIP(r), "serverTime": time.Now().UnixMilli(), "origin": r.Header.Get("Origin"), "userAgent": r.UserAgent()})
	case "/api/connectivity/targets":
		groups, errMsg := connectivityGroupsWithError()
		if errMsg != "" {
			writeJSONError(w, 500, errMsg)
			return
		}
		writeJSON(w, 200, map[string]any{"groups": groups})
	case "/api/newapi/sites":
		sites, msg := config.LoadManagedSites(rootDir)
		publicMsg := publicManagedSiteConfigError(msg)
		publics := []newapi.PublicSite{}
		for _, s := range sites {
			publics = append(publics, s.Public())
		}
		status := 200
		if msg != "" && len(sites) == 0 {
			status = 500
		}
		writeJSON(w, status, map[string]any{"configured": len(sites) > 0, "error": publicMsg, "sites": publics})
	case "/api/newapi/model-status":
		force := r.URL.Query().Get("refresh") == "1"
		status := getModelStatus(r.Context(), force)
		code := 200
		if status.ConfigError != "" && !status.Configured {
			code = 500
		}
		writeJSON(w, code, status)
	case "/api/newapi/overview":
		overview := buildOverview(r.Context(), r.URL.Query())
		writeJSON(w, 200, overview)
	case "/api/newapi/model-status/test":
		handleModelTest(w, r)
	default:
		writeJSONError(w, 404, "API not found")
	}
}

func requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	return webutil.RequireMethodMessage(w, r, method, "Only "+method+" is supported")
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	webutil.WriteJSON(w, status, payload)
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	webutil.WriteJSONError(w, status, message)
}

func readJSON(r *http.Request, out any) error {
	err := webutil.DecodeJSON(http.MaxBytesReader(nil, r.Body, maxNetworkJSONBodyBytes), out, webutil.WithUseNumber())
	if err == nil {
		return nil
	}
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		return errRequestBodyTooLarge
	}
	return err
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
