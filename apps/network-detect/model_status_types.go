package main

type PublicSite struct {
	Name           string  `json:"name"`
	Category       string  `json:"category,omitempty"`
	URL            string  `json:"url,omitempty"`
	DisplayURL     string  `json:"displayUrl"`
	UserID         string  `json:"userId"`
	Kind           string  `json:"kind,omitempty"`
	SkipUserHeader bool    `json:"skipUserHeader,omitempty"`
	QuotaUnit      int64   `json:"quotaUnit"`
	Currency       string  `json:"currency"`
	RechargeRatio  float64 `json:"rechargeRatio"`
}

type SiteStatus struct {
	Site          PublicSite `json:"site"`
	Groups        []string   `json:"groups"`
	Status        string     `json:"status"`
	RequestCount  int        `json:"requestCount"`
	SuccessCount  int        `json:"successCount"`
	FailureCount  int        `json:"failureCount"`
	SuccessRate   *float64   `json:"successRate"`
	LastSeenAt    int64      `json:"lastSeenAt"`
	LogError      string     `json:"logError,omitempty"`
	ChannelsError string     `json:"channelsError,omitempty"`
	PricingError  string     `json:"pricingError,omitempty"`
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
