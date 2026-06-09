package combine

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
