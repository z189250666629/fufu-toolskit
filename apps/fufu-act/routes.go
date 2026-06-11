package activityapp

import (
	"errors"
	"fufu/webutil"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode"
)

const (
	maxCardKeyLength       = 256
	maxJSONBodyBytes int64 = 1 << 20
)

var publicStaticEntryPoints = []string{"/index.html", "/admin.html"}

var errInvalidCardKey = errors.New("invalid card key")
var errRequestBodyTooLarge = errors.New("request body too large")

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
		if !webutil.RequireMethod(w, r, http.MethodGet) {
			return
		}
		handleAdminStats(w, r)
	case "/api/admin/sale-cards/run":
		post(w, r, handleAdminSaleCardsRun)
	case "/api/admin/sale-cards/config":
		handleAdminSaleCardsConfig(w, r)
	case "/api/prizes":
		if !webutil.RequireMethod(w, r, http.MethodGet) {
			return
		}
		handlePrizes(w, r)
	default:
		writeJSONError(w, 404, "Not found")
	}
}

func post(w http.ResponseWriter, r *http.Request, fn func(http.ResponseWriter, *http.Request)) {
	if !webutil.RequireMethod(w, r, http.MethodPost) {
		return
	}
	fn(w, r)
}

func staticRoute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		writeJSONError(w, 405, "Only GET")
		return
	}
	p := r.URL.Path
	if p == "/" {
		p = "/index.html"
	}
	if !webutil.IsPublicStaticPath(p) {
		webutil.WriteStaticNotFound(w, r)
		return
	}
	publicDir := filepath.Join(rootDir, "public")
	file, ok := webutil.SafePath(publicDir, p)
	if !ok {
		webutil.WriteStaticError(w, "Forbidden", http.StatusForbidden)
		return
	}
	info, err := os.Stat(file)
	if err != nil {
		webutil.WriteStaticNotFound(w, r)
		return
	}
	if info.IsDir() {
		webutil.WriteStaticNotFound(w, r)
		return
	}
	if !webutil.IsReferencedBrowserAsset(publicDir, p, publicStaticEntryPoints) {
		webutil.WriteStaticNotFound(w, r)
		return
	}
	webutil.ServeFile(w, r, file, strings.HasSuffix(file, ".html"))
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	webutil.WriteJSON(w, status, payload)
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	webutil.WriteJSONError(w, status, message)
}

func readBody(r *http.Request, out any) error {
	limited := &io.LimitedReader{R: r.Body, N: maxJSONBodyBytes + 1}
	if err := webutil.DecodeJSON(limited, out); err != nil {
		if limited.N <= 0 {
			return errRequestBodyTooLarge
		}
		return err
	}
	if limited.N <= 0 {
		return errRequestBodyTooLarge
	}
	return nil
}

func readCardKeyRequest(r *http.Request, out any, cardKey func() string) (string, bool, error) {
	if err := readBody(r, out); err != nil {
		return "", false, err
	}
	key := strings.TrimSpace(cardKey())
	if key != "" && !isValidCardKey(key) {
		return "", false, errInvalidCardKey
	}
	return key, key != "", nil
}

func isValidCardKey(key string) bool {
	if len([]rune(key)) > maxCardKeyLength {
		return false
	}
	for _, r := range key {
		if unicode.IsControl(r) || unicode.IsSpace(r) {
			return false
		}
	}
	return true
}

func writeCardKeyRequestError(w http.ResponseWriter, err error) {
	if errors.Is(err, errRequestBodyTooLarge) {
		writeJSONError(w, http.StatusRequestEntityTooLarge, "请求体过大")
		return
	}
	if errors.Is(err, errInvalidCardKey) {
		writeJSONError(w, http.StatusBadRequest, "卡密格式错误")
		return
	}
	writeMalformedCardKeyRequest(w)
}

func writeMalformedCardKeyRequest(w http.ResponseWriter) {
	writeJSONError(w, http.StatusBadRequest, "请求格式错误")
}

func writeMissingCardKey(w http.ResponseWriter) {
	writeJSONError(w, http.StatusBadRequest, "请输入卡密")
}

func (e httpErr) Error() string { return e.Message }

func writeHTTPError(w http.ResponseWriter, err error) {
	if e, ok := err.(httpErr); ok {
		writeJSONError(w, e.Status, e.Message)
	} else {
		writeJSONError(w, 500, "服务器错误")
	}
}

type cardLockRegistry struct {
	mu    sync.Mutex
	locks map[string]*cardLockEntry
}

type cardLockEntry struct {
	mu   sync.Mutex
	refs int
}

func withCardLock(key string, fn func() (any, error)) (any, error) {
	entry := cardLocks.acquire(key)
	defer cardLocks.release(key, entry)
	return fn()
}

func (r *cardLockRegistry) acquire(key string) *cardLockEntry {
	r.mu.Lock()
	if r.locks == nil {
		r.locks = map[string]*cardLockEntry{}
	}
	entry := r.locks[key]
	if entry == nil {
		entry = &cardLockEntry{}
		r.locks[key] = entry
	}
	entry.refs++
	r.mu.Unlock()

	entry.mu.Lock()
	return entry
}

func (r *cardLockRegistry) release(key string, entry *cardLockEntry) {
	entry.mu.Unlock()

	r.mu.Lock()
	entry.refs--
	if entry.refs == 0 && r.locks[key] == entry {
		delete(r.locks, key)
	}
	r.mu.Unlock()
}

func (r *cardLockRegistry) has(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.locks[key]
	return ok
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
