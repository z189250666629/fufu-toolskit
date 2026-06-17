package activityapp

import (
	"context"
	"fmt"
	"time"

	"fufu/tokens"
)

var creditQuotaTimeout = 30 * time.Second

type creditQuotaAdapter interface {
	AddQuota(key string, prizeDollars int) error
}

type newAPICreditQuotaAdapter struct {
	service *tokens.Service
}

func (a newAPICreditQuotaAdapter) AddQuota(key string, prizeDollars int) error {
	if a.service == nil {
		return fmt.Errorf("token service is not configured")
	}
	ctx, cancel := creditQuotaContext()
	defer cancel()
	return a.service.AddQuota(ctx, key, int64(prizeDollars))
}

func creditQuotaContext() (context.Context, context.CancelFunc) {
	if creditQuotaTimeout <= 0 {
		return context.WithCancel(context.Background())
	}
	return context.WithTimeout(context.Background(), creditQuotaTimeout)
}
