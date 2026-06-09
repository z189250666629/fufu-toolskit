package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"fufu/newapi"
)

const (
	defaultPort              = "8080"
	logTypeConsume           = 2
	logTypeError             = 5
	channelStatusEnabled     = 1
	modelStatusWindowSeconds = 10 * 60
	modelStatusCacheTTL      = time.Duration(modelStatusWindowSeconds) * time.Second
	modelTestCooldown        = time.Hour
	modelLogPageSize         = 100
	modelLogMaxRowsPerType   = 1000
)

var rootDir string
var frontendDir string
var combineDir string
var combineApp http.Handler
var combineConfigErr error
var modelCache = struct {
	sync.Mutex
	Value   *ModelStatus
	Expires time.Time
}{}
var testCooldowns sync.Map
var testResults sync.Map

type apiResult struct {
	OK     bool
	Status int
	Error  string
	Data   map[string]any
}
type LogRow struct {
	ModelName string
	TokenName string
	Group     string
	RequestID string
	Quota     int64
	CreatedAt int64
	Status    int
	Raw       map[string]any
}
type Channel struct {
	ID           int
	Name         string
	Status       int
	Models       []string
	Groups       []string
	ResponseTime int64
	Raw          map[string]any
}
type Pricing struct {
	Input    float64 `json:"input"`
	Output   float64 `json:"output"`
	Currency string  `json:"currency"`
}
type SiteStatus struct {
	Site          newapi.PublicSite `json:"site"`
	Groups        []string          `json:"groups"`
	Status        string            `json:"status"`
	RequestCount  int               `json:"requestCount"`
	SuccessCount  int               `json:"successCount"`
	FailureCount  int               `json:"failureCount"`
	SuccessRate   *float64          `json:"successRate"`
	LastSeenAt    int64             `json:"lastSeenAt"`
	LogError      string            `json:"logError,omitempty"`
	ChannelsError string            `json:"channelsError,omitempty"`
	PricingError  string            `json:"pricingError,omitempty"`
}
type ModelCell struct {
	Configured          bool                  `json:"configured"`
	SiteName            string                `json:"siteName"`
	Model               string                `json:"model"`
	Status              string                `json:"status"`
	RequestCount        int                   `json:"requestCount"`
	SuccessCount        int                   `json:"successCount"`
	FailureCount        int                   `json:"failureCount"`
	SuccessRate         *float64              `json:"successRate"`
	LastSuccessAt       int64                 `json:"lastSuccessAt"`
	LastFailureAt       int64                 `json:"lastFailureAt"`
	LastSeenAt          int64                 `json:"lastSeenAt"`
	EnabledChannelCount int                   `json:"enabledChannelCount"`
	TotalChannelCount   int                   `json:"totalChannelCount"`
	Groups              []string              `json:"groups"`
	GroupStats          map[string]*ModelCell `json:"groupStats,omitempty"`
	Pricing             *Pricing              `json:"pricing,omitempty"`
	ManualTest          any                   `json:"manualTest,omitempty"`
	NextTestAllowedAt   int64                 `json:"nextTestAllowedAt,omitempty"`
}
type ModelRow struct {
	Model            string                `json:"model"`
	Status           string                `json:"status"`
	OperationalSites int                   `json:"operationalSites"`
	ConfiguredSites  int                   `json:"configuredSites"`
	PerSite          map[string]*ModelCell `json:"perSite"`
}
type ModelStatus struct {
	Configured          bool           `json:"configured"`
	ConfigError         string         `json:"configError,omitempty"`
	GeneratedAt         int64          `json:"generatedAt"`
	ExpiresAt           int64          `json:"expiresAt"`
	WindowSeconds       int            `json:"windowSeconds"`
	RefreshEverySeconds int            `json:"refreshEverySeconds"`
	Sites               []SiteStatus   `json:"sites"`
	Models              []ModelRow     `json:"models"`
	Totals              map[string]int `json:"totals"`
}

type testRecord struct {
	OK            bool   `json:"ok"`
	Status        string `json:"status"`
	Stream        bool   `json:"stream"`
	TestedAt      int64  `json:"testedAt"`
	Message       string `json:"message"`
	NextAllowedAt int64  `json:"nextAllowedAt"`
}

func main() {
	wd, _ := os.Getwd()
	rootDir = wd
	frontendDir = filepath.Join(rootDir, "frontend")
	combineDir = filepath.Join(rootDir, "combine")
	_ = os.MkdirAll(filepath.Join(rootDir, "data"), 0755)
	setupCombine()
	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		port = defaultPort
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", route)
	fmt.Printf("network-detect Go backend listening on :%s\n", port)
	if err := http.ListenAndServe("0.0.0.0:"+port, mux); err != nil {
		panic(err)
	}
}

type httpError struct {
	Status        int
	Message       string
	NextAllowedAt int64
}
