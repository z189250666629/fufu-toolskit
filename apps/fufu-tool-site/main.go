package main

import (
	"fmt"
	activityapp "fufu-act"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"fufu/modelcore"
	"fufu/webutil"
)

const (
	defaultPort                     = "8080"
	logTypeConsume                  = 2
	logTypeError                    = 5
	channelStatusEnabled            = 1
	modelStatusWindowSeconds        = 10 * 60
	modelStatusCacheTTL             = time.Duration(modelStatusWindowSeconds) * time.Second
	modelStatusForceRefreshCooldown = time.Minute
	modelTestCooldown               = time.Hour
	modelLogPageSize                = 100
	modelLogMaxRowsPerType          = 1000
)

var rootDir string
var frontendDir string
var combineDir string
var uiDir string
var navDir string
var adminDir string
var activityDir string
var combineApp http.Handler
var activityApp http.Handler
var combineConfigErr error
var combineRuntimeMu sync.RWMutex
var modelCache = struct {
	sync.Mutex
	Value             *ModelStatus
	Expires           time.Time
	Key               string
	ForceRefreshAfter time.Time
	Inflight          map[string]*modelStatusBuildCall
}{}
var testCooldowns sync.Map
var testClientCooldowns sync.Map
var testResults sync.Map

type apiResult struct {
	OK     bool
	Status int
	Error  string
	Data   map[string]any
}
type LogRow = modelcore.LogRow
type Channel = modelcore.Channel
type Pricing = modelcore.Pricing
type ModelCell = modelcore.ModelCell
type ModelRow = modelcore.ModelRow

type modelStatusBuildCall struct {
	done   chan struct{}
	status *ModelStatus
}

type testRecord struct {
	OK            bool   `json:"ok"`
	Status        string `json:"status"`
	Group         string `json:"group,omitempty"`
	Stream        bool   `json:"stream"`
	TestedAt      int64  `json:"testedAt"`
	Message       string `json:"message"`
	NextAllowedAt int64  `json:"nextAllowedAt"`
}

func main() {
	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to resolve working directory: %v\n", err)
		os.Exit(1)
	}
	if err := run(wd, os.Getenv("PORT")); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func run(wd, portValue string) error {
	port, err := webutil.ResolvePort(portValue, defaultPort)
	if err != nil {
		return fmt.Errorf("invalid PORT: %w", err)
	}
	if err := initRuntime(wd); err != nil {
		return fmt.Errorf("failed to initialize runtime: %w", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", route)
	activityapp.StartWorkers()
	fmt.Printf("fufu tool site listening on :%s\n", port)
	if err := serve(port, mux); err != nil {
		return fmt.Errorf("server stopped: %w", err)
	}
	return nil
}

func initRuntime(wd string) error {
	rootDir = wd
	frontendDir = filepath.Join(rootDir, "frontend")
	combineDir = filepath.Join(rootDir, "combine")
	uiDir = filepath.Join(rootDir, "ui-dist")
	navDir = firstExistingDir(filepath.Join(rootDir, "nav"), filepath.Clean(filepath.Join(rootDir, "..", "y2k-nav")))
	adminDir = filepath.Join(rootDir, "admin")
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

type httpError struct {
	Status        int
	Message       string
	NextAllowedAt int64
}
