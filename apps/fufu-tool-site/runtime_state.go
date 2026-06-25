package main

import (
	"net/http"
	"sync"
	"time"

	"fufu/modelcore"
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
var statusWebDir string
var combineWebDir string
var uiDir string
var navDir string
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

type httpError struct {
	Status        int
	Message       string
	NextAllowedAt int64
}
