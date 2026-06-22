package activityapp

import (
	"context"
	"fmt"
	"time"

	"fufu/newapi"
	"fufu/tokens"
)

var creditQuotaTimeout = 30 * time.Second

type creditQuotaAdapter interface {
	AddQuota(card Card, prizeDollars int) error
}

type newAPICreditQuotaAdapter struct {
	service               *tokens.Service
	subscriptionClient    *newapi.Client
	subscriptionQuotaUnit int64
}

func (a newAPICreditQuotaAdapter) AddQuota(card Card, prizeDollars int) error {
	ctx, cancel := creditQuotaContext()
	defer cancel()

	if isSubscriptionCard(card) {
		if !card.SubscriptionID.Valid || card.SubscriptionID.Int64 <= 0 {
			return fmt.Errorf("subscription id is missing")
		}
		if !card.UserID.Valid || card.UserID.Int64 <= 0 {
			return fmt.Errorf("subscription user id is missing")
		}
		if a.subscriptionClient == nil {
			return fmt.Errorf("subscription runtime is not configured")
		}
		return addQuotaToSubscriptionReward(ctx, a.subscriptionClient, a.subscriptionQuotaUnit, card, prizeDollars)
	}
	if a.service == nil {
		return fmt.Errorf("token service is not configured")
	}
	return a.service.AddQuota(ctx, card.CardKey, int64(prizeDollars))
}

func creditQuotaContext() (context.Context, context.CancelFunc) {
	if creditQuotaTimeout <= 0 {
		return context.WithCancel(context.Background())
	}
	return context.WithTimeout(context.Background(), creditQuotaTimeout)
}
