package webutil

import (
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// IsInsideDir reports whether path resolves inside base.
func IsInsideDir(base, path string) bool {
	baseAbs, err := filepath.Abs(base)
	if err != nil {
		return false
	}
	pathAbs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(baseAbs, pathAbs)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".." && !filepath.IsAbs(rel))
}

// SafePath joins an HTTP URL path to root after cleaning it and rejects
// traversal outside root.
func SafePath(root, urlPath string) (string, bool) {
	clean := filepath.Clean("." + filepath.FromSlash(urlPath))
	file := filepath.Join(root, clean)
	if !IsInsideDir(root, file) {
		return "", false
	}
	return file, true
}

// IsPublicStaticPath reports whether an HTTP static asset path is safe to serve
// from bundled browser assets. It rejects dotfiles and test artifacts that may
// be present in source trees but should not be exposed by the production server.
func IsPublicStaticPath(urlPath string) bool {
	for _, segment := range strings.Split(strings.ReplaceAll(urlPath, "\\", "/"), "/") {
		if segment == "" {
			continue
		}
		if strings.HasPrefix(segment, ".") {
			return false
		}
		switch strings.ToLower(segment) {
		case "__tests__", "tests", "testdata", "coverage":
			return false
		}
	}
	base := strings.ToLower(filepath.Base(filepath.FromSlash(urlPath)))
	return !strings.Contains(base, ".test.") && !strings.Contains(base, ".spec.")
}

var (
	browserAttrRefPattern         = regexp.MustCompile(`(?i)\b(?:src|href)\s*=\s*["']([^"']+)["']`)
	browserJSImportPattern        = regexp.MustCompile(`\bimport\s+(?:[^"']*?\s+from\s+)?["']([^"']+)["']`)
	browserJSDynamicImportPattern = regexp.MustCompile(`\bimport\s*\(\s*["']([^"']+)["']\s*\)`)
	browserJSExportPattern        = regexp.MustCompile(`\bexport\s+[^"']*?\s+from\s+["']([^"']+)["']`)
	browserCSSImportPattern       = regexp.MustCompile(`(?i)@import\s+(?:url\(\s*)?["']?([^"')\s]+)["']?\s*\)?`)
)

// ReferencedBrowserAssetPaths returns the static browser assets reachable from
// the provided HTML/CSS/JS entrypoints. It builds a positive allowlist so source
// files that merely exist in the public directory are not exposed unless the app
// actually references them.
func ReferencedBrowserAssetPaths(root string, entrypoints []string) map[string]struct{} {
	allowed := map[string]struct{}{}
	queued := map[string]struct{}{}
	queue := []string{}
	enqueue := func(candidate string) {
		candidate = normalizeBrowserAssetPath(candidate)
		if candidate == "" {
			return
		}
		if !IsPublicStaticPath(candidate) {
			return
		}
		if _, ok := queued[candidate]; ok {
			return
		}
		queued[candidate] = struct{}{}
		queue = append(queue, candidate)
	}
	for _, entry := range entrypoints {
		enqueue(entry)
	}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		allowed[current] = struct{}{}
		file, ok := SafePath(root, current)
		if !ok {
			continue
		}
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		for _, ref := range browserAssetRefs(current, string(data)) {
			enqueue(ref)
		}
	}
	return allowed
}

func IsReferencedBrowserAsset(root, urlPath string, entrypoints []string) bool {
	normalized := normalizeBrowserAssetPath(urlPath)
	if normalized == "" {
		return false
	}
	_, ok := ReferencedBrowserAssetPaths(root, entrypoints)[normalized]
	return ok
}

func browserAssetRefs(fromPath, content string) []string {
	refs := []string{}
	for _, matches := range browserAttrRefPattern.FindAllStringSubmatch(content, -1) {
		if ref, ok := resolveBrowserAssetRef(fromPath, matches[1]); ok {
			refs = append(refs, ref)
		}
	}
	for _, matches := range browserJSImportPattern.FindAllStringSubmatch(content, -1) {
		if ref, ok := resolveBrowserAssetRef(fromPath, matches[1]); ok {
			refs = append(refs, ref)
		}
	}
	for _, matches := range browserJSDynamicImportPattern.FindAllStringSubmatch(content, -1) {
		if ref, ok := resolveBrowserAssetRef(fromPath, matches[1]); ok {
			refs = append(refs, ref)
		}
	}
	for _, matches := range browserJSExportPattern.FindAllStringSubmatch(content, -1) {
		if ref, ok := resolveBrowserAssetRef(fromPath, matches[1]); ok {
			refs = append(refs, ref)
		}
	}
	for _, matches := range browserCSSImportPattern.FindAllStringSubmatch(content, -1) {
		if ref, ok := resolveBrowserAssetRef(fromPath, matches[1]); ok {
			refs = append(refs, ref)
		}
	}
	return refs
}

func resolveBrowserAssetRef(fromPath, ref string) (string, bool) {
	ref = strings.TrimSpace(ref)
	if ref == "" || strings.HasPrefix(ref, "#") || strings.HasPrefix(ref, "//") {
		return "", false
	}
	if strings.Contains(ref, ":") {
		return "", false
	}
	if cut := strings.IndexAny(ref, "?#"); cut >= 0 {
		ref = ref[:cut]
	}
	if ref == "" {
		return "", false
	}
	if strings.HasPrefix(ref, "/") {
		return normalizeBrowserAssetPath(ref), true
	}
	base := path.Dir(normalizeBrowserAssetPath(fromPath))
	return normalizeBrowserAssetPath(path.Join(base, ref)), true
}

func normalizeBrowserAssetPath(urlPath string) string {
	urlPath = strings.TrimSpace(strings.ReplaceAll(urlPath, "\\", "/"))
	if urlPath == "" {
		return ""
	}
	if cut := strings.IndexAny(urlPath, "?#"); cut >= 0 {
		urlPath = urlPath[:cut]
	}
	if !strings.HasPrefix(urlPath, "/") {
		urlPath = "/" + urlPath
	}
	clean := path.Clean(urlPath)
	if clean == "." || clean == "/" {
		return "/index.html"
	}
	return clean
}

func NewHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}

// ResolvePort normalizes a service port string and rejects values that would
// otherwise fail later at listener startup.
func ResolvePort(value, defaultPort string) (string, error) {
	port := strings.TrimSpace(value)
	if port == "" {
		port = strings.TrimSpace(defaultPort)
	}
	number, err := strconv.Atoi(port)
	if err != nil {
		return "", fmt.Errorf("invalid port %q: must be a number between 1 and 65535", port)
	}
	if number < 1 || number > 65535 {
		return "", fmt.Errorf("invalid port %q: must be between 1 and 65535", port)
	}
	return port, nil
}

func NewStaticHandler(root string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			WriteStaticError(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		path := r.URL.Path
		if path == "/" {
			path = "/index.html"
		}
		if !IsPublicStaticPath(path) {
			WriteStaticNotFound(w, r)
			return
		}
		file, ok := SafePath(root, path)
		if !ok {
			WriteStaticError(w, "forbidden", http.StatusForbidden)
			return
		}
		info, err := os.Stat(file)
		if err != nil || info.IsDir() {
			WriteStaticNotFound(w, r)
			return
		}
		ServeFile(w, r, file, strings.HasSuffix(file, ".html"))
	})
}

func WriteStaticError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Cache-Control", "no-store")
	http.Error(w, message, status)
}

func WriteStaticNotFound(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	http.NotFound(w, r)
}

func WriteJSON(w http.ResponseWriter, status int, payload any) {
	body, err := json.Marshal(payload)
	if err != nil {
		status = http.StatusInternalServerError
		body = []byte(`{"error":"failed to encode response"}`)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func WriteJSONError(w http.ResponseWriter, status int, message string) {
	WriteJSON(w, status, map[string]string{"error": message})
}

func RequireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	return RequireMethodMessage(w, r, method, "Only "+method)
}

func RequireMethodMessage(w http.ResponseWriter, r *http.Request, method, message string) bool {
	if r.Method == method {
		return true
	}
	w.Header().Set("Allow", method)
	WriteJSONError(w, http.StatusMethodNotAllowed, message)
	return false
}

func ServeFile(w http.ResponseWriter, r *http.Request, file string, noStore bool) {
	data, err := os.ReadFile(file)
	if err != nil {
		WriteStaticError(w, "not found", http.StatusNotFound)
		return
	}
	if ct := mime.TypeByExtension(filepath.Ext(file)); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	if noStore || strings.HasSuffix(file, ".html") {
		w.Header().Set("Cache-Control", "no-store")
	} else {
		w.Header().Set("Cache-Control", "public, max-age=300")
	}
	if r.Method != http.MethodHead {
		_, _ = w.Write(data)
	}
}
