package activityapp

import (
	"context"
	"net/http"
	"time"

	"fufu/activity"
	"fufu/newapi"
	"fufu/tokens"
)

type loginRateLimitError struct {
	Until time.Time
}

func (e loginRateLimitError) Error() string {
	return "unknown login rate limited"
}

func lookupLoginToken(ctx context.Context, key, client string, now time.Time) (*tokens.Token, error) {
	if tokenSvc == nil {
		return nil, httpErr{http.StatusServiceUnavailable, "NewAPI 未配置"}
	}
	if blockedUntil, allowed := unknownLoginLimiter.allow(client, key, now); !allowed {
		return nil, loginRateLimitError{Until: blockedUntil}
	}
	t, err := tokenSvc.SearchTokenByKey(ctx, key)
	if err != nil {
		return nil, err
	}
	if t == nil {
		unknownLoginLimiter.recordUnknown(client, key, now)
		return nil, httpErr{http.StatusNotFound, "卡密不存在"}
	}
	unknownLoginLimiter.clear(client, key)
	return t, nil
}

func loginTokenQuotaUnit() int64 {
	unit := int64(newapi.DefaultQuotaUnit)
	if tokenSvc != nil && tokenSvc.QuotaUnit > 0 {
		unit = tokenSvc.QuotaUnit
	}
	return unit
}

func dollarsTier(q int64) float64 {
	return activity.DollarsTier(q, loginTokenQuotaUnit())
}
