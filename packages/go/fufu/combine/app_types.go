package combine

import (
	"database/sql"
	"net/http"
	"sync"
	"time"

	"fufu/newapi"
	"fufu/tokens"
)

type App struct {
	config    Config
	quotaUnit int64
	apiClient *newapi.Client
	tokenSvc  *tokens.Service
	db        *sql.DB
	passwords map[string]struct {
		Hash string
		Role Role
	}
	mu               sync.Mutex
	sessions         map[string]SessionInfo
	authFailures     map[string]authFailureRecord
	authFailureDelay time.Duration
	activeSearches   map[string]struct{}
	searchRequests   map[string]searchRequestRecord
	mergeJobTimeout  time.Duration
	mergeJobs        map[string]MergeJob
	mergeLocks       map[int]struct{}
	static           http.Handler
}

type authFailureRecord struct {
	Count        int
	FirstAttempt time.Time
	BlockedUntil time.Time
}

type searchRequestRecord struct {
	Count       int
	WindowStart time.Time
}

type contextKey string

const roleContextKey contextKey = "role"
