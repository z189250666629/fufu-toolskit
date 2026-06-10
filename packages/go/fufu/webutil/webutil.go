package webutil

import (
	"encoding/json"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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
	WriteJSONError(w, http.StatusMethodNotAllowed, message)
	return false
}

func ServeFile(w http.ResponseWriter, r *http.Request, file string, noStore bool) {
	data, err := os.ReadFile(file)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
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
