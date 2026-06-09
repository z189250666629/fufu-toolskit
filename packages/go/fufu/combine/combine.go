package combine

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"fufu/auth"
	"fufu/newapi"
	"fufu/tokens"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

const (
	searchConcurrency = 6
	sessionTTL        = 4 * time.Hour
	mergeJobTTL       = 30 * time.Minute
	publicSourceUnit  = 3
	publicTargetUnit  = 8
	maxTraceRecords   = 50
)

type Role = auth.Role

const (
	RoleAdmin = auth.RoleAdmin
	RoleUser  = auth.RoleUser
	RoleGuest = auth.RoleGuest
)

type Config struct {
	Name      string `json:"name"`
	URL       string `json:"url"`
	Token     string `json:"token"`
	UserID    string `json:"userId"`
	QuotaUnit int64  `json:"quotaUnit"`
}

type SessionInfo struct {
	Expiry time.Time
	Role   Role
}

type MergeJob struct {
	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt,omitempty"`
	Status    string `json:"status"`
	StepText  string `json:"stepText,omitempty"`
	Current   *int   `json:"current,omitempty"`
	Total     *int   `json:"total,omitempty"`
	Result    any    `json:"result,omitempty"`
	Error     string `json:"error,omitempty"`
	Role      Role   `json:"role,omitempty"`
	MergeID   int64  `json:"mergeId,omitempty"`
}

type MergeJobPatch struct {
	Status    *string
	StepText  *string
	Current   *int
	Total     *int
	Result    any
	HasResult bool
	Error     *string
	Role      *Role
	MergeID   *int64
}

type ResolvedToken struct {
	ID           int            `json:"id"`
	Key          string         `json:"key"`
	Name         string         `json:"name"`
	RemainQuota  int64          `json:"remain_quota"`
	UsedQuota    int64          `json:"used_quota"`
	IntervalUnit int            `json:"interval_unit"`
	Group        string         `json:"group"`
	Status       int            `json:"status"`
	Raw          map[string]any `json:"-"`
}

type PublicMergeEligibility struct {
	Eligible bool     `json:"eligible"`
	Reasons  []string `json:"reasons"`
}

type DeleteResult struct {
	ID  int    `json:"id"`
	Key string `json:"key"`
	OK  bool   `json:"ok"`
}

type NewCardResult struct {
	Key          string `json:"key"`
	Name         string `json:"name"`
	RemainQuota  int64  `json:"remain_quota"`
	IntervalUnit int    `json:"interval_unit"`
	Group        string `json:"group"`
}

type MergeResult struct {
	Success       bool           `json:"success"`
	NewCard       NewCardResult  `json:"newCard"`
	DeleteResults []DeleteResult `json:"deleteResults"`
}

type TraceToken struct {
	TokenID      int    `json:"tokenId,omitempty"`
	Key          string `json:"key"`
	KeyHash      string `json:"keyHash,omitempty"`
	KeyMask      string `json:"keyMask"`
	Name         string `json:"name,omitempty"`
	RemainQuota  int64  `json:"remain_quota,omitempty"`
	UsedQuota    int64  `json:"used_quota,omitempty"`
	IntervalUnit int    `json:"interval_unit,omitempty"`
	Group        string `json:"group,omitempty"`
	Status       int    `json:"status,omitempty"`
	DeleteOK     *bool  `json:"deleteOk,omitempty"`
	DeleteError  string `json:"deleteError,omitempty"`
}

type TraceResult struct {
	MergeID      int64        `json:"mergeId"`
	JobID        string       `json:"jobId,omitempty"`
	Role         Role         `json:"role,omitempty"`
	Status       string       `json:"status"`
	Direction    string       `json:"direction"`
	CreatedAt    int64        `json:"createdAt"`
	UpdatedAt    int64        `json:"updatedAt"`
	CompletedAt  *int64       `json:"completedAt,omitempty"`
	FinalQuota   int64        `json:"finalQuota,omitempty"`
	IntervalUnit int          `json:"intervalUnit,omitempty"`
	FinalName    string       `json:"finalName,omitempty"`
	FinalGroup   string       `json:"finalGroup,omitempty"`
	Error        string       `json:"error,omitempty"`
	RollbackNote string       `json:"rollbackNote,omitempty"`
	SourceKeys   []TraceToken `json:"sourceKeys"`
	ResultKey    *TraceToken  `json:"resultKey,omitempty"`
}

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
	return &App{
		config: cfg, apiURL: cfg.URL, apiToken: cfg.Token, userID: cfg.UserID, quotaUnit: cfg.QuotaUnit,
		db:        db,
		client:    &http.Client{Timeout: 60 * time.Second},
		apiClient: newapi.NewClient(newapi.Site{Name: cfg.Name, URL: cfg.URL, Token: cfg.Token, UserID: cfg.UserID, QuotaUnit: cfg.QuotaUnit, Currency: "$", RechargeRatio: 1}),
		tokenSvc:  tokens.NewService(newapi.NewClient(newapi.Site{Name: cfg.Name, URL: cfg.URL, Token: cfg.Token, UserID: cfg.UserID, QuotaUnit: cfg.QuotaUnit, Currency: "$", RechargeRatio: 1})),
		passwords: map[string]struct {
			Hash string
			Role Role
		}{
			"admin": {"6628b315d42878243a5f3d0638389c2cf69a0efa01346dda6b3c46ae313c9fe9", RoleAdmin},
			"user":  {"5708e5c4c00d86c91e085624253d96bdcf7b3b828243d81e72d883ca414b5d1d", RoleUser},
		},
		sessions: make(map[string]SessionInfo), mergeJobs: make(map[string]MergeJob), mergeLocks: make(map[int]struct{}),
		static: http.FileServer(http.Dir("public")),
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

const traceSchemaSQL = `
CREATE TABLE IF NOT EXISTS merge_records (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  job_id TEXT UNIQUE,
  role TEXT NOT NULL,
  status TEXT NOT NULL,
  requested_interval_unit INTEGER,
  final_quota INTEGER,
  final_name TEXT,
  final_group TEXT,
  error TEXT,
  rollback_attempted INTEGER NOT NULL DEFAULT 0,
  rollback_succeeded INTEGER NOT NULL DEFAULT 0,
  rollback_note TEXT,
  delete_started INTEGER NOT NULL DEFAULT 0,
  old_cards_deleted_count INTEGER NOT NULL DEFAULT 0,
  created_card_id INTEGER,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  completed_at INTEGER
);
CREATE TABLE IF NOT EXISTS merge_tokens (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  merge_id INTEGER NOT NULL REFERENCES merge_records(id) ON DELETE CASCADE,
  kind TEXT NOT NULL CHECK (kind IN ('source', 'result')),
  token_id INTEGER,
  key_full TEXT NOT NULL,
  key_hash TEXT NOT NULL,
  key_mask TEXT NOT NULL,
  name TEXT,
  remain_quota INTEGER,
  used_quota INTEGER,
  interval_unit INTEGER,
  group_name TEXT,
  status INTEGER,
  delete_ok INTEGER,
  delete_error TEXT,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  UNIQUE (merge_id, kind, key_hash)
);
CREATE INDEX IF NOT EXISTS idx_merge_tokens_key_hash ON merge_tokens(key_hash);
CREATE INDEX IF NOT EXISTS idx_merge_tokens_merge_id ON merge_tokens(merge_id);
CREATE INDEX IF NOT EXISTS idx_merge_records_job_id ON merge_records(job_id);
CREATE TABLE IF NOT EXISTS generated_tokens (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  token_id INTEGER NOT NULL,
  key_full TEXT NOT NULL,
  key_hash TEXT NOT NULL UNIQUE,
  key_mask TEXT NOT NULL,
  name TEXT,
  remain_quota INTEGER,
  used_quota INTEGER,
  interval_unit INTEGER,
  group_name TEXT,
  status INTEGER,
  raw_json TEXT,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_generated_tokens_key_hash ON generated_tokens(key_hash);
`

func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		a.handleAPI(w, r)
		return
	}
	if r.URL.Path == "/" || strings.HasSuffix(r.URL.Path, ".html") {
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	}
	a.static.ServeHTTP(w, r)
}

func isPublicAPI(path string) bool {
	return path == "/api/auth" || path == "/api/search-keys" || path == "/api/public-merge" || strings.HasPrefix(path, "/api/merge-status/")
}

func (a *App) handleAPI(w http.ResponseWriter, r *http.Request) {
	if !isPublicAPI(r.URL.Path) {
		role, ok := a.authenticate(r)
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "未授权"})
			return
		}
		r = r.WithContext(context.WithValue(r.Context(), roleContextKey, role))
	}
	switch {
	case r.URL.Path == "/api/auth" && r.Method == http.MethodPost:
		a.handleAuth(w, r)
	case r.URL.Path == "/api/session" && r.Method == http.MethodGet:
		a.handleSession(w, r)
	case r.URL.Path == "/api/search-keys" && r.Method == http.MethodPost:
		a.handleSearchKeys(w, r)
	case r.URL.Path == "/api/merge" && r.Method == http.MethodPost:
		a.handleMerge(w, r)
	case r.URL.Path == "/api/public-merge" && r.Method == http.MethodPost:
		a.handlePublicMerge(w, r)
	case strings.HasPrefix(r.URL.Path, "/api/merge-status/") && r.Method == http.MethodGet:
		a.handleMergeStatus(w, r)
	case r.URL.Path == "/api/generate" && r.Method == http.MethodPost:
		a.handleGenerate(w, r)
	case strings.HasPrefix(r.URL.Path, "/api/token/") && r.Method == http.MethodDelete:
		a.handleDeleteToken(w, r)
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Not found"})
	}
}

func (a *App) authenticate(r *http.Request) (Role, bool) {
	token := r.Header.Get("X-Session-Token")
	if token == "" {
		return "", false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	s, ok := a.sessions[token]
	if !ok || s.Expiry.Before(time.Now()) {
		if ok {
			delete(a.sessions, token)
		}
		return "", false
	}
	return s.Role, true
}

func roleFromContext(ctx context.Context) Role {
	role, _ := ctx.Value(roleContextKey).(Role)
	return role
}

func (a *App) cleanSessionsLocked(now time.Time) {
	for token, session := range a.sessions {
		if session.Expiry.Before(now) {
			delete(a.sessions, token)
		}
	}
}

func (a *App) cleanMergeJobs() {
	now := time.Now()
	a.mu.Lock()
	defer a.mu.Unlock()
	for id, job := range a.mergeJobs {
		base := time.UnixMilli(job.CreatedAt)
		if job.UpdatedAt != 0 {
			base = time.UnixMilli(job.UpdatedAt)
		}
		if base.Add(mergeJobTTL).Before(now) {
			delete(a.mergeJobs, id)
		}
	}
}

func (a *App) setMergeJob(jobID string, p MergeJobPatch) {
	nowMs := time.Now().UnixMilli()
	a.mu.Lock()
	defer a.mu.Unlock()
	job, ok := a.mergeJobs[jobID]
	if !ok {
		job = MergeJob{CreatedAt: nowMs, Status: "queued"}
	}
	job.UpdatedAt = nowMs
	if p.Status != nil {
		job.Status = *p.Status
	}
	if job.Status == "" {
		job.Status = "queued"
	}
	if p.StepText != nil {
		job.StepText = *p.StepText
	}
	if p.Current != nil {
		job.Current = p.Current
	}
	if p.Total != nil {
		job.Total = p.Total
	}
	if p.HasResult {
		job.Result = p.Result
	}
	if p.Error != nil {
		job.Error = *p.Error
	}
	if p.Role != nil {
		job.Role = *p.Role
	}
	if p.MergeID != nil {
		job.MergeID = *p.MergeID
	}
	a.mergeJobs[jobID] = job
}

func (a *App) getMergeJob(jobID string) (MergeJob, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	job, ok := a.mergeJobs[jobID]
	return job, ok
}

func (a *App) acquireMergeLock(ids []int) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, id := range ids {
		if _, ok := a.mergeLocks[id]; ok {
			return false
		}
	}
	for _, id := range ids {
		a.mergeLocks[id] = struct{}{}
	}
	return true
}

func (a *App) releaseMergeLock(ids []int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, id := range ids {
		delete(a.mergeLocks, id)
	}
}

func (a *App) createMergeTrace(ctx context.Context, jobID string, role Role, intervalUnit int) (int64, error) {
	if a.db == nil {
		return 0, nil
	}
	now := time.Now().UnixMilli()
	res, err := a.db.ExecContext(ctx, `
		INSERT INTO merge_records (job_id, role, status, requested_interval_unit, created_at, updated_at)
		VALUES (?, ?, 'started', ?, ?, ?)
	`, jobID, string(role), intervalUnit, now, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (a *App) setTraceStatus(ctx context.Context, mergeID int64, status string) {
	if a.db == nil || mergeID == 0 {
		return
	}
	if _, err := a.db.ExecContext(ctx, `UPDATE merge_records SET status = ?, updated_at = ? WHERE id = ?`, status, time.Now().UnixMilli(), mergeID); err != nil {
		log.Printf("trace status update failed: %v", err)
	}
}

func (a *App) setTraceFinal(ctx context.Context, mergeID int64, quota int64, name, group string) {
	if a.db == nil || mergeID == 0 {
		return
	}
	if _, err := a.db.ExecContext(ctx, `
		UPDATE merge_records
		SET final_quota = ?, final_name = ?, final_group = ?, updated_at = ?
		WHERE id = ?
	`, quota, name, group, time.Now().UnixMilli(), mergeID); err != nil {
		log.Printf("trace final update failed: %v", err)
	}
}

func (a *App) setTraceCreatedCard(ctx context.Context, mergeID int64, cardID int) {
	if a.db == nil || mergeID == 0 {
		return
	}
	if _, err := a.db.ExecContext(ctx, `
		UPDATE merge_records SET created_card_id = ?, updated_at = ? WHERE id = ?
	`, cardID, time.Now().UnixMilli(), mergeID); err != nil {
		log.Printf("trace created card update failed: %v", err)
	}
}

func (a *App) setTraceRollback(ctx context.Context, mergeID int64, succeeded bool, note string) {
	if a.db == nil || mergeID == 0 {
		return
	}
	if _, err := a.db.ExecContext(ctx, `
		UPDATE merge_records
		SET rollback_attempted = 1, rollback_succeeded = ?, rollback_note = ?, updated_at = ?
		WHERE id = ?
	`, boolInt(succeeded), note, time.Now().UnixMilli(), mergeID); err != nil {
		log.Printf("trace rollback update failed: %v", err)
	}
}

func (a *App) setTraceDeleteStarted(ctx context.Context, mergeID int64) {
	if a.db == nil || mergeID == 0 {
		return
	}
	if _, err := a.db.ExecContext(ctx, `
		UPDATE merge_records SET delete_started = 1, updated_at = ? WHERE id = ?
	`, time.Now().UnixMilli(), mergeID); err != nil {
		log.Printf("trace delete start update failed: %v", err)
	}
}

func (a *App) setTraceDeletedCount(ctx context.Context, mergeID int64, count int) {
	if a.db == nil || mergeID == 0 {
		return
	}
	if _, err := a.db.ExecContext(ctx, `
		UPDATE merge_records SET old_cards_deleted_count = ?, updated_at = ? WHERE id = ?
	`, count, time.Now().UnixMilli(), mergeID); err != nil {
		log.Printf("trace deleted count update failed: %v", err)
	}
}

func (a *App) finishTrace(ctx context.Context, mergeID int64, status, errText string) {
	if a.db == nil || mergeID == 0 {
		return
	}
	now := time.Now().UnixMilli()
	if _, err := a.db.ExecContext(ctx, `
		UPDATE merge_records
		SET status = ?, error = ?, updated_at = ?, completed_at = ?
		WHERE id = ?
	`, status, errText, now, now, mergeID); err != nil {
		log.Printf("trace finish update failed: %v", err)
	}
}

func (a *App) upsertTraceToken(ctx context.Context, mergeID int64, kind string, token ResolvedToken) error {
	if a.db == nil || mergeID == 0 {
		return nil
	}
	key := ensureFullKey(token.Key)
	now := time.Now().UnixMilli()
	_, err := a.db.ExecContext(ctx, `
		INSERT INTO merge_tokens (
			merge_id, kind, token_id, key_full, key_hash, key_mask, name,
			remain_quota, used_quota, interval_unit, group_name, status, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(merge_id, kind, key_hash) DO UPDATE SET
			token_id = excluded.token_id,
			key_full = excluded.key_full,
			key_mask = excluded.key_mask,
			name = excluded.name,
			remain_quota = excluded.remain_quota,
			used_quota = excluded.used_quota,
			interval_unit = excluded.interval_unit,
			group_name = excluded.group_name,
			status = excluded.status,
			updated_at = excluded.updated_at
	`, mergeID, kind, token.ID, key, keyHash(key), keyMask(key), token.Name, token.RemainQuota, token.UsedQuota, token.IntervalUnit, token.Group, token.Status, now, now)
	return err
}

func (a *App) setTraceTokenDeleteResult(ctx context.Context, mergeID int64, token ResolvedToken, ok bool, errText string) {
	if a.db == nil || mergeID == 0 {
		return
	}
	if _, err := a.db.ExecContext(ctx, `
		UPDATE merge_tokens
		SET delete_ok = ?, delete_error = ?, updated_at = ?
		WHERE merge_id = ? AND kind = 'source' AND key_hash = ?
	`, boolInt(ok), errText, time.Now().UnixMilli(), mergeID, keyHash(token.Key)); err != nil {
		log.Printf("trace token delete update failed: %v", err)
	}
}

func (a *App) traceResultsForKeys(ctx context.Context, rawKeys []string) ([]TraceResult, error) {
	if a.db == nil {
		return []TraceResult{}, nil
	}
	keys := normalizeKeys(rawKeys)
	if len(keys) == 0 {
		return []TraceResult{}, nil
	}
	hashSet := map[string]bool{}
	hashes := []string{}
	for _, key := range keys {
		hash := keyHash(key)
		if !hashSet[hash] {
			hashSet[hash] = true
			hashes = append(hashes, hash)
		}
	}

	seenHashes := map[string]bool{}
	for hash := range hashSet {
		seenHashes[hash] = true
	}
	seenMergeIDs := map[int64]bool{}
	mergeIDs := []int64{}
	frontier := hashes

	for len(frontier) > 0 && len(mergeIDs) < maxTraceRecords {
		ids, err := a.traceMergeIDsForHashes(ctx, frontier, maxTraceRecords-len(mergeIDs))
		if err != nil {
			return nil, err
		}
		newIDs := []int64{}
		for _, id := range ids {
			if seenMergeIDs[id] {
				continue
			}
			seenMergeIDs[id] = true
			mergeIDs = append(mergeIDs, id)
			newIDs = append(newIDs, id)
		}
		if len(newIDs) == 0 {
			break
		}

		relatedHashes, err := a.traceKeyHashesForMergeIDs(ctx, newIDs)
		if err != nil {
			return nil, err
		}
		next := []string{}
		for _, hash := range relatedHashes {
			if seenHashes[hash] {
				continue
			}
			seenHashes[hash] = true
			next = append(next, hash)
		}
		frontier = next
	}

	results := []TraceResult{}
	for _, mergeID := range mergeIDs {
		trace, err := a.loadTraceResult(ctx, mergeID, hashSet)
		if err != nil {
			return nil, err
		}
		results = append(results, trace)
	}
	sort.SliceStable(results, func(i, j int) bool {
		return results[i].CreatedAt < results[j].CreatedAt
	})
	return results, nil
}

func (a *App) traceMergeIDsForHashes(ctx context.Context, hashes []string, limit int) ([]int64, error) {
	if len(hashes) == 0 || limit <= 0 {
		return nil, nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(hashes)), ",")
	args := make([]any, 0, len(hashes)+1)
	for _, hash := range hashes {
		args = append(args, hash)
	}
	args = append(args, limit)
	rows, err := a.db.QueryContext(ctx, `
		SELECT DISTINCT r.id, r.created_at
		FROM merge_records r
		JOIN merge_tokens t ON t.merge_id = r.id
		WHERE t.key_hash IN (`+placeholders+`)
		ORDER BY r.created_at ASC
		LIMIT ?
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := []int64{}
	for rows.Next() {
		var id int64
		var createdAt int64
		if err := rows.Scan(&id, &createdAt); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (a *App) traceKeyHashesForMergeIDs(ctx context.Context, mergeIDs []int64) ([]string, error) {
	if len(mergeIDs) == 0 {
		return nil, nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(mergeIDs)), ",")
	args := make([]any, 0, len(mergeIDs))
	for _, id := range mergeIDs {
		args = append(args, id)
	}
	rows, err := a.db.QueryContext(ctx, `
		SELECT DISTINCT key_hash
		FROM merge_tokens
		WHERE merge_id IN (`+placeholders+`)
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	hashes := []string{}
	for rows.Next() {
		var hash string
		if err := rows.Scan(&hash); err != nil {
			return nil, err
		}
		if strings.TrimSpace(hash) != "" {
			hashes = append(hashes, hash)
		}
	}
	return hashes, rows.Err()
}

func (a *App) loadTraceResult(ctx context.Context, mergeID int64, queryHashes map[string]bool) (TraceResult, error) {
	var trace TraceResult
	var role string
	var completedAt sql.NullInt64
	err := a.db.QueryRowContext(ctx, `
		SELECT id, COALESCE(job_id, ''), role, status, created_at, updated_at, completed_at,
		       COALESCE(final_quota, 0), COALESCE(requested_interval_unit, 0),
		       COALESCE(final_name, ''), COALESCE(final_group, ''),
		       COALESCE(error, ''), COALESCE(rollback_note, '')
		FROM merge_records
		WHERE id = ?
	`, mergeID).Scan(&trace.MergeID, &trace.JobID, &role, &trace.Status, &trace.CreatedAt, &trace.UpdatedAt, &completedAt, &trace.FinalQuota, &trace.IntervalUnit, &trace.FinalName, &trace.FinalGroup, &trace.Error, &trace.RollbackNote)
	if err != nil {
		return TraceResult{}, err
	}
	trace.Role = Role(role)
	if completedAt.Valid {
		trace.CompletedAt = &completedAt.Int64
	}

	rows, err := a.db.QueryContext(ctx, `
		SELECT kind, COALESCE(token_id, 0), key_full, key_hash, key_mask,
		       COALESCE(name, ''), COALESCE(remain_quota, 0), COALESCE(used_quota, 0),
		       COALESCE(interval_unit, 0), COALESCE(group_name, ''), COALESCE(status, 0),
		       delete_ok, COALESCE(delete_error, '')
		FROM merge_tokens
		WHERE merge_id = ?
		ORDER BY CASE kind WHEN 'source' THEN 0 ELSE 1 END, id
	`, mergeID)
	if err != nil {
		return TraceResult{}, err
	}
	defer rows.Close()

	matchedSource := false
	matchedResult := false
	for rows.Next() {
		var kind string
		var token TraceToken
		var hash string
		var deleteOK sql.NullInt64
		if err := rows.Scan(&kind, &token.TokenID, &token.Key, &hash, &token.KeyMask, &token.Name, &token.RemainQuota, &token.UsedQuota, &token.IntervalUnit, &token.Group, &token.Status, &deleteOK, &token.DeleteError); err != nil {
			return TraceResult{}, err
		}
		token.KeyHash = hash
		if deleteOK.Valid {
			ok := deleteOK.Int64 == 1
			token.DeleteOK = &ok
		}
		if kind == "source" {
			trace.SourceKeys = append(trace.SourceKeys, token)
			matchedSource = matchedSource || queryHashes[hash]
		} else if kind == "result" {
			trace.ResultKey = &token
			matchedResult = matchedResult || queryHashes[hash]
		}
	}
	if err := rows.Err(); err != nil {
		return TraceResult{}, err
	}
	switch {
	case matchedSource && matchedResult:
		trace.Direction = "both"
	case matchedResult:
		trace.Direction = "result"
	case matchedSource:
		trace.Direction = "source"
	default:
		trace.Direction = "related"
	}
	return trace, nil
}

func (a *App) upsertGeneratedToken(ctx context.Context, token ResolvedToken) error {
	if a.db == nil || token.ID == 0 || strings.TrimSpace(token.Key) == "" {
		return nil
	}
	key := ensureFullKey(token.Key)
	rawJSON, _ := json.Marshal(token.Raw)
	now := time.Now().UnixMilli()
	_, err := a.db.ExecContext(ctx, `
		INSERT INTO generated_tokens (
			token_id, key_full, key_hash, key_mask, name, remain_quota, used_quota,
			interval_unit, group_name, status, raw_json, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(key_hash) DO UPDATE SET
			token_id = excluded.token_id,
			key_full = excluded.key_full,
			key_mask = excluded.key_mask,
			name = excluded.name,
			remain_quota = excluded.remain_quota,
			used_quota = excluded.used_quota,
			interval_unit = excluded.interval_unit,
			group_name = excluded.group_name,
			status = excluded.status,
			raw_json = excluded.raw_json,
			updated_at = excluded.updated_at
	`, token.ID, key, keyHash(key), keyMask(key), token.Name, token.RemainQuota, token.UsedQuota, token.IntervalUnit, token.Group, token.Status, string(rawJSON), now, now)
	return err
}

func (a *App) generatedTokenIDByKey(ctx context.Context, key string) (int, bool, error) {
	if a.db == nil {
		return 0, false, nil
	}
	var id int
	err := a.db.QueryRowContext(ctx, `SELECT token_id FROM generated_tokens WHERE key_hash = ?`, keyHash(key)).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return id, id > 0, nil
}

func (a *App) deleteGeneratedTokenCacheByID(ctx context.Context, tokenID int) {
	if a.db == nil || tokenID == 0 {
		return
	}
	if _, err := a.db.ExecContext(ctx, `DELETE FROM generated_tokens WHERE token_id = ?`, tokenID); err != nil {
		log.Printf("generated token cache delete failed: %v", err)
	}
}

func (a *App) handleAuth(w http.ResponseWriter, r *http.Request) {
	var p struct {
		Password string `json:"password"`
	}
	if err := decodeJSON(r.Body, &p); err != nil {
		writeJSON(w, 400, map[string]string{"error": "请求格式错误"})
		return
	}
	hash := sha256Hex(p.Password)
	var matched Role
	for _, item := range a.passwords {
		if item.Hash == hash {
			matched = item.Role
			break
		}
	}
	if matched == "" {
		time.Sleep(time.Second)
		writeJSON(w, 401, map[string]string{"error": "密码错误"})
		return
	}
	token, err := randomHex(24)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "生成会话失败"})
		return
	}
	now := time.Now()
	a.mu.Lock()
	a.cleanSessionsLocked(now)
	a.sessions[token] = SessionInfo{Expiry: now.Add(sessionTTL), Role: matched}
	a.mu.Unlock()
	writeJSON(w, 200, map[string]any{"token": token, "role": matched})
}

func (a *App) handleSession(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"ok": true, "role": roleFromContext(r.Context())})
}

func (a *App) handleSearchKeys(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	var p struct {
		Keys []string `json:"keys"`
	}
	if err := decodeJSON(r.Body, &p); err != nil {
		writeJSON(w, 400, map[string]string{"error": "请求格式错误"})
		return
	}
	if len(p.Keys) == 0 {
		writeJSON(w, 400, map[string]string{"error": "No keys provided"})
		return
	}
	keys, found, missing, err := a.resolveTokensForSearch(r.Context(), p.Keys)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	traceResults, err := a.traceResultsForKeys(r.Context(), p.Keys)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	elig := evaluatePublicMergeEligibility(found)
	writeJSON(w, 200, map[string]any{
		"found": found, "missing": missing, "quotaUnit": a.quotaUnit, "searched": len(keys),
		"concurrency": min(searchConcurrency, len(keys)), "elapsedMs": time.Since(started).Milliseconds(),
		"publicMergeEligibility": map[string]any{"eligible": elig.Eligible, "reasons": elig.Reasons, "targetUnit": publicTargetUnit},
		"traceResults":           traceResults,
	})
}

func (a *App) handleMerge(w http.ResponseWriter, r *http.Request) {
	a.cleanMergeJobs()
	var p MergePayload
	if err := decodeJSON(r.Body, &p); err != nil {
		writeJSON(w, 400, map[string]string{"error": "请求格式错误"})
		return
	}
	role := roleFromContext(r.Context())
	jobID, err := randomHex(16)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "创建任务失败"})
		return
	}
	a.setMergeJob(jobID, MergeJobPatch{Status: strp("queued"), StepText: strp("准备合并..."), Current: intp(0), Total: intp(len(p.Keys)), Role: &role})
	go a.runMergeJob(jobID, p, role)
	writeJSON(w, 200, map[string]any{"ok": true, "jobId": jobID})
}

func (a *App) handlePublicMerge(w http.ResponseWriter, r *http.Request) {
	a.cleanMergeJobs()
	var p struct {
		Keys []string `json:"keys"`
	}
	if err := decodeJSON(r.Body, &p); err != nil {
		writeJSON(w, 400, map[string]string{"error": "请求格式错误"})
		return
	}
	jobID, err := randomHex(16)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "创建任务失败"})
		return
	}
	role := RoleGuest
	a.setMergeJob(jobID, MergeJobPatch{Status: strp("queued"), StepText: strp("准备普通合卡..."), Current: intp(0), Total: intp(len(p.Keys)), Role: &role})
	go a.runMergeJob(jobID, MergePayload{Keys: p.Keys, IntervalUnit: publicTargetUnit}, RoleGuest)
	writeJSON(w, 200, map[string]any{"ok": true, "jobId": jobID})
}

func (a *App) runMergeJob(jobID string, p MergePayload, role Role) {
	defer func() {
		if x := recover(); x != nil {
			a.setMergeJob(jobID, MergeJobPatch{Status: strp("error"), StepText: strp("合并失败"), Error: strp(fmt.Sprint(x))})
		}
	}()
	_, err := a.executeMerge(context.Background(), ExecuteMergeParams{Keys: p.Keys, IntervalUnit: p.IntervalUnit, TotalQuota: p.TotalQuota, Name: p.Name, CustomQuota: p.CustomQuota, Role: role, JobID: jobID})
	if err != nil {
		a.setMergeJob(jobID, MergeJobPatch{Status: strp("error"), StepText: strp("合并失败"), Error: strp(err.Error())})
	}
}

func (a *App) handleMergeStatus(w http.ResponseWriter, r *http.Request) {
	a.cleanMergeJobs()
	jobID := strings.TrimPrefix(r.URL.Path, "/api/merge-status/")
	job, ok := a.getMergeJob(jobID)
	if jobID == "" || !ok {
		writeJSON(w, 404, map[string]string{"error": "任务不存在或已过期"})
		return
	}
	writeJSON(w, 200, job)
}

func (a *App) handleGenerate(w http.ResponseWriter, r *http.Request) {
	if roleFromContext(r.Context()) != RoleAdmin {
		writeJSON(w, 403, map[string]string{"error": "无权操作"})
		return
	}
	var p struct {
		Count        int     `json:"count"`
		Quota        float64 `json:"quota"`
		IntervalUnit int     `json:"intervalUnit"`
		Group        string  `json:"group"`
	}
	if err := decodeJSON(r.Body, &p); err != nil {
		writeJSON(w, 400, map[string]string{"error": "请求格式错误"})
		return
	}
	if p.Count < 1 || p.Count > 100 || p.Quota <= 0 || p.IntervalUnit == 0 {
		writeJSON(w, 400, map[string]string{"error": "参数无效"})
		return
	}
	totalQuota := int64(math.Round(p.Quota * float64(a.quotaUnit)))
	group := strings.TrimSpace(p.Group)
	if group == "" {
		group = "mix"
	}
	keys := []string{}
	errs := []string{}
	for i := 0; i < p.Count; i++ {
		uniqueName := fmt.Sprintf("gen-%d-%s", time.Now().UnixMilli(), randomBase36(6))
		body := map[string]any{"name": uniqueName, "remain_quota": totalQuota, "unlimited_quota": false, "expired_time": -1, "group": group, "interval_quota": totalQuota, "interval_time": -1, "trigger_last_time": 0, "interval_unit": p.IntervalUnit}
		res, _, err := a.createToken(r.Context(), body)
		if err != nil {
			errs = append(errs, fmt.Sprintf("#%d: %s", i+1, err))
			continue
		}
		if !res.OK() {
			errs = append(errs, fmt.Sprintf("#%d: 创建失败", i+1))
			continue
		}
		token, err := a.searchTokenByName(r.Context(), uniqueName)
		if err != nil {
			errs = append(errs, fmt.Sprintf("#%d: %s", i+1, err))
			continue
		}
		if token == nil {
			errs = append(errs, fmt.Sprintf("#%d: 创建成功但未找到", i+1))
			continue
		}
		card := cloneMap(token.Raw)
		card["name"] = strconv.FormatFloat(p.Quota, 'f', -1, 64)
		if res, _, err := a.updateTokenRaw(r.Context(), card); err != nil {
			errs = append(errs, fmt.Sprintf("#%d: %s", i+1, err))
			continue
		} else if !res.OK() {
			errs = append(errs, fmt.Sprintf("#%d: 重命名失败", i+1))
			continue
		}
		tokenID := toInt(card["id"])
		verifiedToken, err := a.fetchVerifiedToken(r.Context(), tokenID)
		if err != nil {
			errs = append(errs, fmt.Sprintf("#%d: 创建成功但复查失败: %s", i+1, err))
			continue
		}
		if err := a.upsertGeneratedToken(r.Context(), verifiedToken); err != nil {
			log.Printf("generated token cache insert failed: %v", err)
		}
		keys = append(keys, verifiedToken.Key)
	}
	writeJSON(w, 200, map[string]any{"keys": keys, "errors": errs})
}

func (a *App) handleDeleteToken(w http.ResponseWriter, r *http.Request) {
	role := roleFromContext(r.Context())
	if role != RoleAdmin && role != RoleUser {
		writeJSON(w, 403, map[string]string{"error": "无权删除"})
		return
	}
	id, err := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/api/token/"))
	if err != nil || id <= 0 {
		writeJSON(w, 400, map[string]string{"error": "无效的 token ID"})
		return
	}
	ok, res, err := a.deleteToken(r.Context(), id)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if !ok {
		writeJSON(w, statusOrDefault(res.StatusCode, 500), map[string]string{"error": upstreamStatusMessage(res, "删除失败")})
		return
	}
	a.deleteGeneratedTokenCacheByID(r.Context(), id)
	writeJSON(w, 200, map[string]bool{"success": true})
}

type MergePayload struct {
	Keys         []string `json:"keys"`
	IntervalUnit int      `json:"intervalUnit"`
	TotalQuota   *int64   `json:"totalQuota"`
	Name         string   `json:"name"`
	CustomQuota  bool     `json:"customQuota"`
}
type ExecuteMergeParams struct {
	Keys         []string
	IntervalUnit int
	TotalQuota   *int64
	Name         string
	CustomQuota  bool
	Role         Role
	JobID        string
}
type MergeCardParams struct {
	Keys         []string
	IntervalUnit int
	Quota        *int64
	Name         string
	Role         Role
	JobID        string
	Validate     func([]ResolvedToken) error
	OnProgress   func(MergeJobPatch)
}
type rollbackState struct {
	attempted, succeeded bool
	note                 string
}

func (a *App) executeMerge(ctx context.Context, p ExecuteMergeParams) (MergeResult, error) {
	validate := func(tokens []ResolvedToken) error {
		if p.Role == RoleGuest {
			if p.CustomQuota || p.TotalQuota != nil || strings.TrimSpace(p.Name) != "" {
				return errors.New("普通免登录合卡不支持指定额度或自定义命名")
			}
			if p.IntervalUnit != publicTargetUnit {
				return errors.New("普通免登录合卡只支持合成周不刷新卡")
			}
			elig := evaluatePublicMergeEligibility(tokens)
			if !elig.Eligible {
				return fmt.Errorf("普通免登录合卡仅支持未使用天卡 → 周不刷新卡：%s", strings.Join(elig.Reasons, "；"))
			}
		}
		if p.Role == RoleUser && len(tokens) == 1 && p.IntervalUnit != tokens[0].IntervalUnit {
			return errors.New("单卡续卡只能保持原卡类型")
		}
		if p.Role == RoleUser && len(tokens) > 1 {
			units := map[int]bool{}
			for _, t := range tokens {
				units[t.IntervalUnit] = true
			}
			allowed := map[int]bool{}
			if units[3] {
				allowed[8] = true
			}
			if units[8] {
				allowed[8] = true
			}
			if units[9] {
				allowed[9] = true
			}
			if !allowed[p.IntervalUnit] {
				return errors.New("当前卡组合不允许生成该类型的卡")
			}
		}
		if p.Role != RoleAdmin && p.CustomQuota {
			return errors.New("无权指定额度")
		}
		return nil
	}
	var onProgress func(MergeJobPatch)
	if p.JobID != "" {
		onProgress = func(patch MergeJobPatch) { a.setMergeJob(p.JobID, patch) }
	}
	var quota *int64
	if p.Role == RoleAdmin && p.CustomQuota {
		quota = p.TotalQuota
	}
	name := ""
	if p.Role != RoleGuest {
		name = strings.TrimSpace(p.Name)
	}
	return a.mergeCards(ctx, MergeCardParams{Keys: p.Keys, IntervalUnit: p.IntervalUnit, Quota: quota, Name: name, Role: p.Role, JobID: p.JobID, Validate: validate, OnProgress: onProgress})
}

func (a *App) mergeCards(ctx context.Context, p MergeCardParams) (result MergeResult, err error) {
	update := p.OnProgress
	if update == nil {
		update = func(MergeJobPatch) {}
	}

	mergeID, err := a.createMergeTrace(ctx, p.JobID, p.Role, p.IntervalUnit)
	if err != nil {
		return MergeResult{}, err
	}
	if mergeID != 0 {
		update(MergeJobPatch{MergeID: &mergeID})
	}
	createdID := 0
	rollbackAttempted := false
	rollbackSucceeded := false
	rollbackNote := ""
	mergeCompleted := false
	deletionStarted := false
	oldDeleted := 0
	attemptRollback := func(reason string) rollbackState {
		if createdID == 0 || rollbackAttempted {
			return rollbackState{rollbackAttempted, rollbackSucceeded, rollbackNote}
		}
		rollbackAttempted = true
		update(MergeJobPatch{Status: strp("rollback"), StepText: strp("回滚新卡中..."), Total: intp(1), Current: intp(0)})
		a.setTraceStatus(context.Background(), mergeID, "rollback")
		ok, res, delErr := a.deleteToken(context.Background(), createdID)
		rollbackSucceeded = ok && delErr == nil
		if rollbackSucceeded {
			rollbackNote = fmt.Sprintf("已回滚新卡（%s）", reason)
		} else {
			msg := "未知错误"
			if delErr != nil {
				msg = delErr.Error()
			} else {
				msg = upstreamStatusMessage(res, "删除失败")
			}
			rollbackNote = fmt.Sprintf("新卡回滚失败（%s）：%s", reason, msg)
		}
		a.setTraceRollback(context.Background(), mergeID, rollbackSucceeded, rollbackNote)
		update(MergeJobPatch{Current: intp(1)})
		return rollbackState{rollbackAttempted, rollbackSucceeded, rollbackNote}
	}
	defer func() {
		if err != nil {
			if createdID != 0 && !mergeCompleted && !rollbackAttempted && (!deletionStarted || oldDeleted == 0) {
				attemptRollback("合并异常")
			}
			if rollbackAttempted && !rollbackSucceeded && rollbackNote != "" && !strings.Contains(err.Error(), rollbackNote) {
				err = fmt.Errorf("%s %s", strings.TrimSpace(err.Error()), rollbackNote)
			}
			a.finishTrace(context.Background(), mergeID, "error", err.Error())
			return
		}
		if mergeCompleted {
			a.finishTrace(context.Background(), mergeID, "done", "")
		}
	}()

	a.setTraceStatus(ctx, mergeID, "resolving")
	sourceTokens, err := a.resolveTokensStrict(ctx, p.Keys)
	if err != nil {
		return MergeResult{}, err
	}
	ids := uniqueIDs(sourceTokens)
	if len(ids) != len(sourceTokens) {
		return MergeResult{}, errors.New("存在重复的 key，请勿提交相同的卡密")
	}
	if !a.acquireMergeLock(ids) {
		return MergeResult{}, errors.New("这些卡正在合并中，请稍后再试")
	}
	defer a.releaseMergeLock(ids)
	verifiedQuota := int64(0)
	verified := []ResolvedToken{}
	update(MergeJobPatch{Status: strp("verifying"), StepText: strp("校验额度中..."), Total: intp(len(ids)), Current: intp(0)})
	a.setTraceStatus(ctx, mergeID, "verifying")
	for i, id := range ids {
		t, e := a.fetchVerifiedToken(ctx, id)
		if e != nil {
			return MergeResult{}, e
		}
		req := findResolvedByID(sourceTokens, id)
		if req == nil {
			return MergeResult{}, fmt.Errorf("Token %d 校验失败", id)
		}
		if strings.TrimPrefix(t.Key, "sk-") != strings.TrimPrefix(req.Key, "sk-") {
			return MergeResult{}, fmt.Errorf("%s 校验失败，请重试", displayKey(req.Key))
		}
		if t.Status != 1 {
			return MergeResult{}, fmt.Errorf("%s 已被禁用，无法参与合卡", displayKey(t.Key))
		}
		verified = append(verified, t)
		verifiedQuota += t.RemainQuota
		if e := a.upsertTraceToken(ctx, mergeID, "source", t); e != nil {
			return MergeResult{}, e
		}
		update(MergeJobPatch{Current: intp(i + 1)})
	}
	if p.Validate != nil {
		if e := p.Validate(verified); e != nil {
			return MergeResult{}, e
		}
	}
	finalQuota := verifiedQuota
	if p.Quota != nil {
		finalQuota = *p.Quota
	}
	if finalQuota <= 0 {
		return MergeResult{}, errors.New("合并额度无效")
	}
	finalName := strings.TrimSpace(p.Name)
	if finalName == "" {
		finalName = strconv.FormatInt(int64(math.Round(float64(finalQuota)/float64(a.quotaUnit))), 10)
	}
	finalGroup := majorityGroup(verified)
	uniqueName := fmt.Sprintf("merge-%d-%s", time.Now().UnixMilli(), randomBase36(6))
	a.setTraceFinal(ctx, mergeID, finalQuota, finalName, finalGroup)

	update(MergeJobPatch{Status: strp("creating"), StepText: strp("创建新卡中..."), Total: intp(1), Current: intp(0)})
	a.setTraceStatus(ctx, mergeID, "creating")
	body := map[string]any{"name": uniqueName, "remain_quota": finalQuota, "unlimited_quota": false, "expired_time": -1, "group": finalGroup, "interval_quota": finalQuota, "interval_time": -1, "trigger_last_time": 0, "interval_unit": p.IntervalUnit}
	res, _, e := a.createToken(ctx, body)
	if e != nil {
		return MergeResult{}, e
	}
	if !res.OK() {
		return MergeResult{}, errors.New(upstreamStatusMessage(res, "新卡创建失败"))
	}
	update(MergeJobPatch{Current: intp(1)})

	update(MergeJobPatch{Status: strp("renaming"), StepText: strp("整理新卡信息中..."), Total: intp(1), Current: intp(0)})
	a.setTraceStatus(ctx, mergeID, "renaming")
	token, e := a.searchTokenByName(ctx, uniqueName)
	if e != nil {
		return MergeResult{}, e
	}
	if token == nil || token.ID == 0 {
		return MergeResult{}, errors.New("新卡创建成功但未找到，请稍后人工检查")
	}
	newCard := cloneMap(token.Raw)
	createdID = toInt(newCard["id"])
	a.setTraceCreatedCard(ctx, mergeID, createdID)
	newCard["name"] = finalName
	res, _, e = a.updateTokenRaw(ctx, newCard)
	if e != nil || !res.OK() {
		rb := attemptRollback("重命名失败")
		if rb.succeeded {
			return MergeResult{}, errors.New("新卡重命名失败，已回滚")
		}
		return MergeResult{}, fmt.Errorf("新卡重命名失败，且回滚失败：%s", rb.note)
	}
	resultTraceToken := tokenFromRaw(newCard)
	if e := a.upsertTraceToken(ctx, mergeID, "result", resultTraceToken); e != nil {
		log.Printf("trace result token insert failed: %v", e)
	}
	update(MergeJobPatch{Current: intp(1)})

	update(MergeJobPatch{Status: strp("deleting"), StepText: strp("删卡中..."), Total: intp(len(verified)), Current: intp(0)})
	a.setTraceStatus(ctx, mergeID, "deleting")
	a.setTraceDeleteStarted(ctx, mergeID)
	deletionStarted = true
	deleteResults := []DeleteResult{}
	deleteFailures := []string{}
	for i, t := range verified {
		ok, res, delErr := a.deleteToken(ctx, t.ID)
		deleteResults = append(deleteResults, DeleteResult{ID: t.ID, Key: t.Key, OK: ok && delErr == nil})
		deleteMessage := ""
		if delErr != nil || !ok {
			if delErr != nil {
				deleteMessage = delErr.Error()
			} else {
				deleteMessage = upstreamStatusMessage(res, "删除失败")
			}
			deleteFailures = append(deleteFailures, displayKey(t.Key))
		} else {
			oldDeleted++
			a.setTraceDeletedCount(ctx, mergeID, oldDeleted)
		}
		a.setTraceTokenDeleteResult(ctx, mergeID, t, ok && delErr == nil, deleteMessage)
		update(MergeJobPatch{Current: intp(i + 1)})
	}
	if len(deleteFailures) > 0 {
		failed := strings.Join(deleteFailures, "、")
		if oldDeleted == 0 {
			rb := attemptRollback("旧卡删除失败")
			if rb.succeeded {
				return MergeResult{}, fmt.Errorf("旧卡删除失败：%s。未删除任何旧卡，已回滚新卡。", failed)
			}
			return MergeResult{}, fmt.Errorf("旧卡删除失败：%s。新卡回滚失败，请立即人工检查。%s", failed, rb.note)
		}
		return MergeResult{}, fmt.Errorf("旧卡删除不完整：%s。已保留新卡以避免额度丢失，请立即人工清理剩余旧卡。", failed)
	}

	result = MergeResult{Success: true, NewCard: NewCardResult{Key: ensureFullKey(getString(newCard, "key")), Name: getString(newCard, "name"), RemainQuota: int64OrDefault(toInt64(newCard["remain_quota"]), finalQuota), IntervalUnit: intOrDefault(toInt(newCard["interval_unit"]), p.IntervalUnit), Group: stringOrDefault(getString(newCard, "group"), finalGroup)}, DeleteResults: deleteResults}
	mergeCompleted = true
	update(MergeJobPatch{Status: strp("done"), StepText: strp("合并完成"), Result: result, HasResult: true, Total: intp(len(verified)), Current: intp(len(verified))})
	return result, nil
}

func (a *App) resolveTokensForSearch(ctx context.Context, raw []string) ([]string, []ResolvedToken, []string, error) {
	keys := normalizeKeys(raw)
	results, err := a.searchTokensConcurrent(ctx, keys)
	if err != nil {
		return keys, nil, nil, err
	}
	found := []ResolvedToken{}
	missing := []string{}
	for _, r := range results {
		if r.Found != nil {
			found = append(found, *r.Found)
		} else {
			missing = append(missing, r.Key)
		}
	}
	return keys, found, missing, nil
}

func (a *App) resolveTokensStrict(ctx context.Context, raw []string) ([]ResolvedToken, error) {
	keys := normalizeKeys(raw)
	if len(keys) == 0 {
		return nil, errors.New("No keys provided")
	}
	results, err := a.searchTokensConcurrent(ctx, keys)
	if err != nil {
		return nil, err
	}
	missing := []string{}
	found := make([]ResolvedToken, 0, len(keys))
	for _, r := range results {
		if r.Found == nil {
			missing = append(missing, r.Key)
		} else {
			found = append(found, *r.Found)
		}
	}
	if len(missing) > 0 {
		shown := []string{}
		for _, k := range missing {
			shown = append(shown, displayKey(k))
		}
		return nil, fmt.Errorf("未找到令牌: %s", strings.Join(shown, ", "))
	}
	return found, nil
}

type SearchTokenResult struct {
	Key   string
	Found *ResolvedToken
}

func (a *App) searchTokensConcurrent(ctx context.Context, keys []string) ([]SearchTokenResult, error) {
	results := make([]SearchTokenResult, len(keys))
	if len(keys) == 0 {
		return results, nil
	}
	jobs := make(chan int)
	errCh := make(chan error, 1)
	var wg sync.WaitGroup
	for i := 0; i < min(searchConcurrency, len(keys)); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				found, err := a.searchTokenByKey(ctx, keys[idx])
				if err != nil {
					select {
					case errCh <- err:
					default:
					}
					continue
				}
				results[idx] = SearchTokenResult{Key: keys[idx], Found: found}
			}
		}()
	}
	for i := range keys {
		select {
		case jobs <- i:
		case err := <-errCh:
			close(jobs)
			wg.Wait()
			return nil, err
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return nil, ctx.Err()
		}
	}
	close(jobs)
	wg.Wait()
	select {
	case err := <-errCh:
		return nil, err
	default:
		return results, nil
	}
}

func (a *App) searchTokenByKey(ctx context.Context, key string) (*ResolvedToken, error) {
	if a.tokenSvc == nil {
		a.tokenSvc = tokens.NewService(a.apiClient)
	}
	found, err := a.tokenSvc.SearchTokenByKey(ctx, key)
	if err != nil || found == nil {
		return nil, err
	}
	resolved := resolvedFromToken(*found)
	if id, ok, err := a.generatedTokenIDByKey(ctx, key); err != nil {
		return nil, err
	} else if ok && resolved.ID == 0 {
		token, err := a.fetchVerifiedToken(ctx, id)
		if err == nil {
			return &token, nil
		}
	}
	return &resolved, nil
}

func (a *App) fetchVerifiedToken(ctx context.Context, id int) (ResolvedToken, error) {
	if a.tokenSvc == nil {
		a.tokenSvc = tokens.NewService(a.apiClient)
	}
	token, err := a.tokenSvc.GetToken(ctx, id)
	if err != nil {
		return ResolvedToken{}, err
	}
	return resolvedFromToken(token), nil
}

type APIResponse = newapi.Response

func upstreamStatusMessage(r APIResponse, fallback string) string {
	if r.StatusCode > 0 {
		return fmt.Sprintf("%s（上游状态 %d）", fallback, r.StatusCode)
	}
	return fallback
}

func (a *App) deleteToken(ctx context.Context, id int) (bool, APIResponse, error) {
	if a.tokenSvc == nil {
		a.tokenSvc = tokens.NewService(a.apiClient)
	}
	return a.tokenSvc.DeleteToken(ctx, id)
}

func (a *App) createToken(ctx context.Context, body map[string]any) (APIResponse, map[string]any, error) {
	if a.tokenSvc == nil {
		a.tokenSvc = tokens.NewService(a.apiClient)
	}
	return a.tokenSvc.CreateToken(ctx, body)
}

func (a *App) updateTokenRaw(ctx context.Context, raw map[string]any) (APIResponse, map[string]any, error) {
	if a.tokenSvc == nil {
		a.tokenSvc = tokens.NewService(a.apiClient)
	}
	return a.tokenSvc.UpdateTokenRaw(ctx, raw)
}

func (a *App) searchTokenByName(ctx context.Context, name string) (*tokens.Token, error) {
	if a.tokenSvc == nil {
		a.tokenSvc = tokens.NewService(a.apiClient)
	}
	return a.tokenSvc.SearchTokenByName(ctx, name)
}

func (a *App) apiRequest(ctx context.Context, method, endpoint string, body any) (APIResponse, map[string]any, error) {
	if a.apiClient == nil {
		a.apiClient = newapi.NewClient(newapi.Site{Name: a.config.Name, URL: a.apiURL, Token: a.apiToken, UserID: a.userID, QuotaUnit: a.quotaUnit, Currency: "$", RechargeRatio: 1})
	}
	return a.apiClient.Request(ctx, method, endpoint, body)
}

func resolvedFromToken(t tokens.Token) ResolvedToken {
	return ResolvedToken{ID: t.ID, Key: t.Key, Name: t.Name, RemainQuota: t.RemainQuota, UsedQuota: t.UsedQuota, IntervalUnit: t.IntervalUnit, Group: t.Group, Status: t.Status, Raw: t.Raw}
}

func tokenFromRaw(raw map[string]any) ResolvedToken {
	if raw == nil {
		raw = map[string]any{}
	}
	return ResolvedToken{ID: toInt(raw["id"]), Key: ensureFullKey(getString(raw, "key")), Name: getString(raw, "name"), RemainQuota: toInt64(raw["remain_quota"]), UsedQuota: toInt64(raw["used_quota"]), IntervalUnit: toInt(raw["interval_unit"]), Group: stringOrDefault(getString(raw, "group"), "mix"), Status: intOrDefault(toIntDefault(raw["status"], 1), 1), Raw: raw}
}

func cloneMap(raw map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range raw {
		out[k] = v
	}
	return out
}

func ensureFullKey(key string) string {
	s := strings.TrimSpace(key)
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, "sk-") {
		return s
	}
	return "sk-" + s
}

func displayKey(key string) string {
	full := ensureFullKey(key)
	bare := strings.TrimPrefix(full, "sk-")
	r := []rune(bare)
	if len(r) <= 8 {
		return full
	}
	return "sk-" + string(r[:4]) + "…" + string(r[len(r)-4:])
}

func normalizeKeys(raw []string) []string {
	seen := map[string]bool{}
	keys := []string{}
	for _, item := range raw {
		key := ensureFullKey(strings.TrimSpace(item))
		if key == "" || key == "sk-" || seen[key] {
			continue
		}
		seen[key] = true
		keys = append(keys, key)
	}
	return keys
}

func majorityGroup(tokens []ResolvedToken) string {
	counts := map[string]int{}
	for _, t := range tokens {
		g := t.Group
		if g == "" {
			g = "mix"
		}
		counts[g]++
	}
	winner := "mix"
	max := 0
	for g, c := range counts {
		if c > max {
			winner = g
			max = c
		}
	}
	return winner
}

func evaluatePublicMergeEligibility(tokens []ResolvedToken) PublicMergeEligibility {
	reasons := []string{}
	if len(tokens) < 2 {
		reasons = append(reasons, "至少需要 2 张天卡才能合卡")
	}
	for _, t := range tokens {
		if t.Status != 1 {
			reasons = append(reasons, fmt.Sprintf("%s 已被禁用", displayKey(t.Key)))
		}
		if t.IntervalUnit != publicSourceUnit {
			reasons = append(reasons, fmt.Sprintf("%s 不是天卡", displayKey(t.Key)))
		}
		if t.UsedQuota > 0 {
			reasons = append(reasons, fmt.Sprintf("%s 已经使用过", displayKey(t.Key)))
		}
		if t.RemainQuota <= 0 {
			reasons = append(reasons, fmt.Sprintf("%s 没有剩余额度", displayKey(t.Key)))
		}
	}
	return PublicMergeEligibility{Eligible: len(reasons) == 0, Reasons: reasons}
}

func dataList(data map[string]any) []map[string]any {
	raw, ok := data["data"].([]any)
	if !ok {
		return nil
	}
	out := []map[string]any{}
	for _, item := range raw {
		if obj, ok := item.(map[string]any); ok {
			out = append(out, obj)
		}
	}
	return out
}

func findTokenByName(data map[string]any, name string) map[string]any {
	for _, item := range dataList(data) {
		if getString(item, "name") == name {
			return item
		}
	}
	return nil
}
func findResolvedByID(tokens []ResolvedToken, id int) *ResolvedToken {
	for i := range tokens {
		if tokens[i].ID == id {
			return &tokens[i]
		}
	}
	return nil
}
func uniqueIDs(tokens []ResolvedToken) []int {
	seen := map[int]bool{}
	ids := []int{}
	for _, t := range tokens {
		if !seen[t.ID] {
			seen[t.ID] = true
			ids = append(ids, t.ID)
		}
	}
	return ids
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("write json: %v", err)
	}
}
func decodeJSON(r io.Reader, out any) error {
	dec := json.NewDecoder(r)
	dec.UseNumber()
	return dec.Decode(out)
}
func sha256Hex(v string) string { sum := sha256.Sum256([]byte(v)); return hex.EncodeToString(sum[:]) }
func keyHash(key string) string { return sha256Hex(ensureFullKey(key)) }
func keyMask(key string) string { return displayKey(key) }
func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
func randomBase36(n int) string {
	const alphabet = "0123456789abcdefghijklmnopqrstuvwxyz"
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	for i, v := range b {
		b[i] = alphabet[int(v)%len(alphabet)]
	}
	return string(b)
}

func getString(obj map[string]any, key string) string {
	if obj == nil || obj[key] == nil {
		return ""
	}
	switch v := obj[key].(type) {
	case string:
		return v
	case json.Number:
		return v.String()
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprint(v)
	}
}

func toInt(v any) int { return int(toInt64(v)) }
func toIntDefault(v any, fallback int) int {
	if v == nil {
		return fallback
	}
	return toInt(v)
}
func toInt64(v any) int64 {
	switch x := v.(type) {
	case nil:
		return 0
	case int:
		return int64(x)
	case int64:
		return x
	case int32:
		return int64(x)
	case float64:
		return int64(x)
	case float32:
		return int64(x)
	case json.Number:
		if i, err := x.Int64(); err == nil {
			return i
		}
		if f, err := x.Float64(); err == nil {
			return int64(f)
		}
	case string:
		s := strings.TrimSpace(x)
		if i, err := strconv.ParseInt(s, 10, 64); err == nil {
			return i
		}
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return int64(f)
		}
	}
	return 0
}
func intOrDefault(v, fallback int) int {
	if v == 0 {
		return fallback
	}
	return v
}
func int64OrDefault(v, fallback int64) int64 {
	if v == 0 {
		return fallback
	}
	return v
}
func stringOrDefault(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}
func strp(v string) *string { return &v }
func intp(v int) *int       { return &v }
func statusOrDefault(status, fallback int) int {
	if status >= 100 && status <= 599 {
		return status
	}
	return fallback
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
