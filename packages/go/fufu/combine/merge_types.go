package combine

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
	Client    string `json:"-"`
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
