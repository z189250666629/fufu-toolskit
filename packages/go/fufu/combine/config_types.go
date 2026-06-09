package combine

import "time"

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
