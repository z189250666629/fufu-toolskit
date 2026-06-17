package main

import (
	"fufu/combine"
	"net/http"
	"path/filepath"
)

func setupCombine() {
	site, err := primarySiteForCombine()
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

func rebuildCombine() {
	if closer, ok := combineApp.(interface{ Close() error }); ok {
		_ = closer.Close()
	}
	combineApp = nil
	combineConfigErr = nil
	setupCombine()
}

func isCombineAPI(path string) bool {
	return combine.IsAPIPath(path)
}

func combineAPIMethod(path string) (string, bool) {
	return combine.APIMethod(path)
}

type trustedCombineHandler interface {
	ServeHTTPAsRole(http.ResponseWriter, *http.Request, combine.Role)
}

func serveCombineAPI(w http.ResponseWriter, r *http.Request) {
	if validUnifiedAdminSession(r) {
		if handler, ok := combineApp.(trustedCombineHandler); ok {
			handler.ServeHTTPAsRole(w, r, combine.RoleAdmin)
			return
		}
	}
	combineApp.ServeHTTP(w, r)
}
