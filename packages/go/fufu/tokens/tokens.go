package tokens

import "fufu/newapi"

const DefaultSearchConcurrency = 6

type Token struct {
	ID            int            `json:"id"`
	Key           string         `json:"key"`
	Name          string         `json:"name"`
	RemainQuota   int64          `json:"remain_quota"`
	UsedQuota     int64          `json:"used_quota"`
	IntervalUnit  int            `json:"interval_unit"`
	IntervalQuota int64          `json:"interval_quota,omitempty"`
	Group         string         `json:"group"`
	Status        int            `json:"status"`
	CreatedTime   int64          `json:"created_time,omitempty"`
	Raw           map[string]any `json:"-"`
}

type Service struct {
	Client      *newapi.Client
	QuotaUnit   int64
	Concurrency int
}

func NewService(client *newapi.Client) *Service {
	quotaUnit := newapi.DefaultQuotaUnit
	if client != nil && client.Site.QuotaUnit > 0 {
		quotaUnit = client.Site.QuotaUnit
	}
	return &Service{Client: client, QuotaUnit: quotaUnit, Concurrency: DefaultSearchConcurrency}
}
