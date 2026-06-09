package combine

import (
	"database/sql"
	"net/http"
	"sync"
	"time"

	"fufu/auth"
	"fufu/newapi"
	"fufu/tokens"
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

type SearchTokenResult struct {
	Key   string
	Found *ResolvedToken
}

type APIResponse = newapi.Response
