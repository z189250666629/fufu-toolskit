package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"fufu/webutil"
)

const defaultPort = "33148"

func main() {
	wd, _ := os.Getwd()
	port, err := webutil.ResolvePort(os.Getenv("PORT"), defaultPort)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid PORT: %v\n", err)
		os.Exit(1)
	}
	if err := serve(wd, port); err != nil {
		fmt.Fprintf(os.Stderr, "server stopped: %v\n", err)
		os.Exit(1)
	}
}

func serve(root, port string) error {
	mux := http.NewServeMux()
	mux.Handle("/", newStaticHandler(root))
	fmt.Printf("y2k-nav Go static server listening on :%s\n", port)
	return newHTTPServer(port, mux).ListenAndServe()
}

func newHTTPServer(port string, handler http.Handler) *http.Server {
	return webutil.NewHTTPServer("0.0.0.0:"+port, handler)
}

func newStaticHandler(root string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			webutil.WriteStaticError(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		path := r.URL.Path
		if path == "/" {
			path = "/index.html"
		}
		file, ok := webutil.SafePath(root, path)
		if !ok {
			webutil.WriteStaticError(w, "forbidden", http.StatusForbidden)
			return
		}
		if !isPublicBrowserAsset(path) {
			webutil.WriteStaticNotFound(w, r)
			return
		}
		if _, err := os.Stat(file); err != nil {
			webutil.WriteStaticNotFound(w, r)
			return
		}
		webutil.ServeFile(w, r, file, strings.HasSuffix(file, ".html"))
	})
}

func isPublicBrowserAsset(path string) bool {
	switch path {
	case "/index.html", "/theme.mjs", "/latency.mjs":
		return true
	default:
		return false
	}
}
