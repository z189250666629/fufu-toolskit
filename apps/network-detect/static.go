package main

import (
	"fufu/webutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func serveStatic(w http.ResponseWriter, r *http.Request, path string) {
	if path == "/" {
		path = "/index.html"
	}
	if !webutil.IsPublicStaticPath(path) {
		webutil.WriteStaticNotFound(w, r)
		return
	}
	file, ok := webutil.SafePath(frontendDir, path)
	if !ok {
		webutil.WriteStaticError(w, "Forbidden", http.StatusForbidden)
		return
	}
	if _, err := os.Stat(file); err != nil {
		if filepath.Ext(path) != "" {
			webutil.WriteStaticNotFound(w, r)
			return
		}
		file = filepath.Join(frontendDir, "index.html")
	}
	serveFile(w, r, file, strings.HasSuffix(file, ".html"))
}

func serveFile(w http.ResponseWriter, r *http.Request, file string, noStore bool) {
	webutil.ServeFile(w, r, file, noStore)
}
