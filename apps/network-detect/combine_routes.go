package main

import (
	"fufu/combine"
	"fufu/config"
	"path/filepath"
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
	return combine.IsAPIPath(path)
}
