package main

import (
	"fufu/webutil"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

func apiRoute(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/api/login":
		post(w, r, handleLogin)
	case "/api/spin":
		post(w, r, handleSpin)
	case "/api/scratch/start":
		post(w, r, handleScratchStart)
	case "/api/scratch/reveal":
		post(w, r, handleScratchReveal)
	case "/api/scratch/cashout":
		post(w, r, handleScratchCashout)
	case "/api/scratch/reset":
		post(w, r, handleScratchReset)
	case "/api/admin/stats":
		if r.Method != http.MethodGet {
			writeJSON(w, 405, map[string]string{"error": "Only GET"})
			return
		}
		handleAdminStats(w, r)
	case "/api/prizes":
		if r.Method != http.MethodGet {
			writeJSON(w, 405, map[string]string{"error": "Only GET"})
			return
		}
		handlePrizes(w, r)
	default:
		writeJSON(w, 404, map[string]string{"error": "Not found"})
	}
}

func post(w http.ResponseWriter, r *http.Request, fn func(http.ResponseWriter, *http.Request)) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "Only POST"})
		return
	}
	fn(w, r)
}

func staticRoute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeJSON(w, 405, map[string]string{"error": "Only GET"})
		return
	}
	p := r.URL.Path
	if p == "/" {
		p = "/index.html"
	}
	publicDir := filepath.Join(rootDir, "public")
	file, ok := webutil.SafePath(publicDir, p)
	if !ok {
		http.Error(w, "Forbidden", 403)
		return
	}
	if _, err := os.Stat(file); err != nil {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, file)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	webutil.WriteJSON(w, status, payload)
}

func readBody(r *http.Request, out any) error {
	return webutil.DecodeJSON(io.LimitReader(r.Body, 1<<20), out)
}

func readCardKeyRequest(r *http.Request, out any, cardKey func() string) (string, bool) {
	if readBody(r, out) != nil {
		return "", false
	}
	key := strings.TrimSpace(cardKey())
	return key, key != ""
}

func writeMissingCardKey(w http.ResponseWriter) {
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": "请输入卡密"})
}

func (e httpErr) Error() string { return e.Message }

func writeHTTPError(w http.ResponseWriter, err error) {
	if e, ok := err.(httpErr); ok {
		writeJSON(w, e.Status, map[string]string{"error": e.Message})
	} else {
		writeJSON(w, 500, map[string]string{"error": "服务器错误"})
	}
}

func withCardLock(key string, fn func() (any, error)) (any, error) {
	muIface, _ := cardLocks.LoadOrStore(key, &sync.Mutex{})
	mu := muIface.(*sync.Mutex)
	mu.Lock()
	defer mu.Unlock()
	return fn()
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
