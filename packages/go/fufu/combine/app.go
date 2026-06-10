package combine

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fufu/newapi"
	"fufu/tokens"
	"fufu/webutil"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

func LoadConfig(path string) (Config, error) { return loadConfig(path) }

func InitTraceDB(path string) (*sql.DB, error) { return initTraceDB(path) }

func NewApp(cfg Config, db *sql.DB) *App { return newApp(cfg, db) }

func loadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	if strings.TrimSpace(cfg.URL) == "" || strings.TrimSpace(cfg.Token) == "" || strings.TrimSpace(cfg.UserID) == "" {
		return Config{}, errors.New("combine config.json is invalid")
	}
	if cfg.QuotaUnit <= 0 {
		cfg.QuotaUnit = 500000
	}
	cfg.URL = strings.TrimRight(cfg.URL, "/")
	return cfg, nil
}

func newApp(cfg Config, db *sql.DB) *App {
	apiClient := newapi.NewClient(newapi.Site{Name: cfg.Name, URL: cfg.URL, Token: cfg.Token, UserID: cfg.UserID, QuotaUnit: cfg.QuotaUnit, Currency: "$", RechargeRatio: 1})
	return &App{
		config: cfg, quotaUnit: cfg.QuotaUnit,
		db:        db,
		apiClient: apiClient,
		tokenSvc:  tokens.NewService(apiClient),
		passwords: map[string]struct {
			Hash string
			Role Role
		}{
			"admin": {"6628b315d42878243a5f3d0638389c2cf69a0efa01346dda6b3c46ae313c9fe9", RoleAdmin},
			"user":  {"5708e5c4c00d86c91e085624253d96bdcf7b3b828243d81e72d883ca414b5d1d", RoleUser},
		},
		sessions: make(map[string]SessionInfo), authFailures: make(map[string]authFailureRecord), authFailureDelay: authFailureDelay, activeSearches: make(map[string]struct{}),
		mergeJobs: make(map[string]MergeJob), mergeLocks: make(map[int]struct{}),
		static: webutil.NewStaticHandler("public"),
	}
}

func initTraceDB(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(`PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON; PRAGMA busy_timeout=5000;`); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(traceSchemaSQL); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}
