package main

import (
	"fmt"
	activityapp "fufu-act"
	"net/http"
	"os"
	"path/filepath"

	"fufu/webutil"
)

func initRuntime(wd string) error {
	rootDir = wd
	statusWebDir = filepath.Join(rootDir, "web", "status")
	combineWebDir = filepath.Join(rootDir, "web", "combine")
	uiDir = filepath.Join(rootDir, "ui-dist")
	navDir = firstExistingDir(filepath.Join(rootDir, "nav"), filepath.Clean(filepath.Join(rootDir, "..", "y2k-nav")))
	activityDir = firstExistingDir(filepath.Join(rootDir, "activity"), filepath.Clean(filepath.Join(rootDir, "..", "fufu-act")))
	closeCombineRuntime()
	resetAdminLoginLimiter()
	activityApp = nil
	if err := os.MkdirAll(filepath.Join(rootDir, "data"), 0755); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}
	unifiedConfig = newToolConfigStore(filepath.Join(rootDir, "data", toolConfigDBName))
	if err := unifiedConfig.Load(rootDir); err != nil {
		return fmt.Errorf("load unified admin config: %w", err)
	}
	setupCombine()
	var err error
	activityApp, err = activityapp.NewHandler(activityDir)
	if err != nil {
		return fmt.Errorf("initialize activity module: %w", err)
	}
	applyToolConfigSnapshot(unifiedConfig.Snapshot())
	return nil
}

func firstExistingDir(candidates ...string) string {
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}
	if len(candidates) == 0 {
		return ""
	}
	return candidates[0]
}

func shutdownRuntime() {
	closeCombineRuntime()
	_ = activityapp.Close()
	activityApp = nil
	if unifiedConfig != nil {
		_ = unifiedConfig.Close()
	}
	unifiedConfig = nil
}

func serve(port string, handler http.Handler) error {
	return newHTTPServer(port, handler).ListenAndServe()
}

func newHTTPServer(port string, handler http.Handler) *http.Server {
	return webutil.NewHTTPServer("0.0.0.0:"+port, handler)
}
