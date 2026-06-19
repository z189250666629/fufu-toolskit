package main

import (
	"fmt"
	"fufu/combine"
	"net/http"
	"path/filepath"
)

func setupCombine() {
	app, err := buildCombineApp()
	replaceCombineRuntime(app, err)
}

func buildCombineApp() (http.Handler, error) {
	site, err := primarySiteForCombine()
	if err != nil {
		return nil, err
	}
	db, err := combine.InitTraceDB(filepath.Join(rootDir, "data", "combine-trace.db"))
	if err != nil {
		return nil, err
	}
	cfg := combine.Config{Name: site.Name, URL: site.URL, Token: site.Token, UserID: site.UserID, QuotaUnit: site.QuotaUnit}
	return combine.NewApp(cfg, db), nil
}

func replaceCombineRuntime(app http.Handler, configErr error) {
	combineRuntimeMu.Lock()
	old := combineApp
	combineApp = app
	combineConfigErr = configErr
	combineRuntimeMu.Unlock()
	if closer, ok := old.(interface{ Close() error }); ok {
		_ = closer.Close()
	}
}

func closeCombineRuntime() {
	replaceCombineRuntime(nil, nil)
}

func rebuildCombine() {
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
	combineRuntimeMu.RLock()
	defer combineRuntimeMu.RUnlock()
	if combineApp == nil {
		message := "combine is not configured"
		if combineConfigErr != nil {
			message = fmt.Sprintf("combine is not configured: %v", combineConfigErr)
		}
		writeJSONError(w, http.StatusServiceUnavailable, message)
		return
	}
	if validUnifiedAdminSession(r) {
		if handler, ok := combineApp.(trustedCombineHandler); ok {
			handler.ServeHTTPAsRole(w, r, combine.RoleAdmin)
			return
		}
	}
	combineApp.ServeHTTP(w, r)
}
