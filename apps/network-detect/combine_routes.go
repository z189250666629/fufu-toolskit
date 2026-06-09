package main

import (
	"fufu/combine"
	"fufu/config"
	"path/filepath"
	"strings"
)

func setupCombine() {
	site, err := config.LoadPrimarySite(rootDir)
	if err != nil {
		combineConfigErr = err
		return
	}
	db, err := combine.InitTraceDB(filepath.Join(rootDir, "data", "combine-trace.db"))
	if err != nil {
		combineConfigErr = err
		return
	}
	cfg := combine.Config{Name: site.Name, URL: site.URL, Token: site.Token, UserID: site.UserID, QuotaUnit: site.QuotaUnit}
	combineApp = combine.NewApp(cfg, db)
}

func isCombineAPI(path string) bool {
	return path == "/api/auth" || path == "/api/session" || path == "/api/search-keys" || path == "/api/merge" || path == "/api/public-merge" || path == "/api/generate" || strings.HasPrefix(path, "/api/merge-status/") || strings.HasPrefix(path, "/api/token/")
}
