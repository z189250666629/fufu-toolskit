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
	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		port = defaultPort
	}
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		path := r.URL.Path
		if path == "/" {
			path = "/index.html"
		}
		file, ok := webutil.SafePath(wd, path)
		if !ok {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if _, err := os.Stat(file); err != nil {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, file)
	})
	fmt.Printf("y2k-nav Go static server listening on :%s\n", port)
	panic(http.ListenAndServe("0.0.0.0:"+port, nil))
}
