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
var uiStaticEntryPoints = []string{"/index.html"}
var navStaticEntryPoints = []string{"/index.html"}

func serveStatic(w http.ResponseWriter, r *http.Request, path string) {
	if serveUIStatic(w, r, path) {
		return
	}
	if serveNavStatic(w, r, path) {
		return
	}
	if serveRootActivityStatic(w, r, path) {
		return
	}
	serveFrontendStatic(w, r, path)
}

func serveUIStatic(w http.ResponseWriter, r *http.Request, path string) bool {
	if !webutil.IsPublicStaticPath(path) {
		return false
	}
	if !webutil.IsReferencedBrowserAsset(uiDir, path, uiStaticEntryPoints) {
		return false
	}
	file, ok := webutil.SafePath(uiDir, path)
	if !ok {
		webutil.WriteStaticError(w, "Forbidden", http.StatusForbidden)
		return true
	}
	if _, err := os.Stat(file); err != nil {
		return false
	}
	serveFile(w, r, file, strings.HasSuffix(file, ".html"))
	return true
}

func serveFrontendStatic(w http.ResponseWriter, r *http.Request, path string) {
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

func serveNavStatic(w http.ResponseWriter, r *http.Request, path string) bool {
	if path == "/" {
		path = "/index.html"
	}
	if !webutil.IsPublicStaticPath(path) {
		return false
	}
	if !webutil.IsReferencedBrowserAsset(navDir, path, navStaticEntryPoints) {
		return false
	}
	file, ok := webutil.SafePath(navDir, path)
	if !ok {
		webutil.WriteStaticError(w, "Forbidden", http.StatusForbidden)
		return true
	}
	if _, err := os.Stat(file); err != nil {
		return false
	}
	serveFile(w, r, file, strings.HasSuffix(file, ".html"))
	return true
}

func serveRootActivityStatic(w http.ResponseWriter, r *http.Request, path string) bool {
	if activityApp == nil || !webutil.IsPublicStaticPath(path) {
		return false
	}
	publicDir := filepath.Join(activityDir, "public")
	if !webutil.IsReferencedBrowserAsset(publicDir, path, []string{"/index.html"}) {
		return false
	}
	cloned := r.Clone(r.Context())
	cloned.URL.Path = path
	activityApp.ServeHTTP(w, cloned)
	return true
}

func serveAdminShell(w http.ResponseWriter, r *http.Request) {
	serveUnifiedUIShell(w, r)
}

func serveUnifiedUIShell(w http.ResponseWriter, r *http.Request) {
	serveFile(w, r, filepath.Join(uiDir, "index.html"), true)
}

func serveActivityPage(w http.ResponseWriter, r *http.Request, path string) {
	stripped := strings.TrimPrefix(path, "/activity")
	if stripped == "" || stripped == "/" {
		stripped = "/"
	}
	serveActivityPageAs(w, r, stripped)
}

func serveActivityPageAs(w http.ResponseWriter, r *http.Request, path string) {
	if activityApp == nil {
		writeJSONError(w, 503, "activity is not configured")
		return
	}
	cloned := r.Clone(r.Context())
	cloned.URL.Path = path
	activityApp.ServeHTTP(w, cloned)
}

func isReferencedNetworkBrowserAsset(path string) bool {
	return webutil.IsReferencedBrowserAsset(frontendDir, path, frontendStaticEntryPoints) ||
		webutil.IsReferencedBrowserAsset(combineDir, path, combineStaticEntryPoints)
}

func serveFile(w http.ResponseWriter, r *http.Request, file string, noStore bool) {
	webutil.ServeFile(w, r, file, noStore)
}
