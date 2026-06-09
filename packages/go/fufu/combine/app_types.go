package combine

import (
	"database/sql"
	"net/http"
	"sync"

	"fufu/newapi"
	"fufu/tokens"
)

type App struct {
	config    Config
	apiURL    string
	apiToken  string
	userID    string
	quotaUnit int64
	client    *http.Client
	apiClient *newapi.Client
	tokenSvc  *tokens.Service
	db        *sql.DB
	passwords map[string]struct {
		Hash string
		Role Role
	}
	mu         sync.Mutex
	sessions   map[string]SessionInfo
	mergeJobs  map[string]MergeJob
	mergeLocks map[int]struct{}
	static     http.Handler
}

type contextKey string

const roleContextKey contextKey = "role"
