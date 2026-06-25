package main

import (
	"fufu/webutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

var frontendStaticEntryPoints = []string{"/index.html"}
var combineStaticEntryPoints = []string{"/index.html"}

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
		path = "/index.html"
	}
	if !isReferencedNetworkBrowserAsset(path) {
		webutil.WriteStaticNotFound(w, r)
		return
	}
	serveFile(w, r, file, strings.HasSuffix(file, ".html"))
}

func isReferencedNetworkBrowserAsset(path string) bool {
	return webutil.IsReferencedBrowserAsset(frontendDir, path, frontendStaticEntryPoints) ||
		webutil.IsReferencedBrowserAsset(combineDir, path, combineStaticEntryPoints)
}

func serveFile(w http.ResponseWriter, r *http.Request, file string, noStore bool) {
	webutil.ServeFile(w, r, file, noStore)
}
