package main

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"fufu/webutil"
)

const defaultPort = "33148"

func main() {
	wd, _ := os.Getwd()
	port, err := resolvePort(os.Getenv("PORT"))
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
	return http.ListenAndServe("0.0.0.0:"+port, mux)
}

func resolvePort(value string) (string, error) {
	port := strings.TrimSpace(value)
	if port == "" {
		return defaultPort, nil
	}
	number, err := strconv.Atoi(port)
	if err != nil {
		return "", fmt.Errorf("must be a number between 1 and 65535")
	}
	if number < 1 || number > 65535 {
		return "", fmt.Errorf("must be between 1 and 65535")
	}
	return port, nil
}

func newStaticHandler(root string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		path := r.URL.Path
		if path == "/" {
			path = "/index.html"
		}
		file, ok := webutil.SafePath(root, path)
		if !ok {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if _, err := os.Stat(file); err != nil {
			http.NotFound(w, r)
			return
		}
		webutil.ServeFile(w, r, file, strings.HasSuffix(file, ".html"))
	})
}
