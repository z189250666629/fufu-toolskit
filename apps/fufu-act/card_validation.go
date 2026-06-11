package activityapp

import (
	"context"
	"net/http"
)

func requireCurrentTokenActive(ctx context.Context, key string) error {
	if tokenSvc == nil {
		return httpErr{http.StatusServiceUnavailable, "NewAPI 未配置"}
	}
	token, err := tokenSvc.SearchTokenByKey(ctx, key)
	if err != nil {
		return err
	}
	if token == nil {
		return httpErr{http.StatusNotFound, "卡密不存在"}
	}
	if token.Status != 1 {
		return httpErr{http.StatusForbidden, "此卡密已被禁用，无法参与活动"}
	}
	return nil
}

func requireScratchEligibleCard(ctx context.Context, key string) (Card, error) {
	card, ok, err := lookupCard(key)
	if err != nil {
		return Card{}, err
	}
	if !ok {
		return Card{}, httpErr{http.StatusNotFound, "请先登录"}
	}
	if err := requireCurrentTokenActive(ctx, key); err != nil {
		return Card{}, err
	}
	if !isScratchDollarTier(card.Dollars) {
		return Card{}, httpErr{http.StatusForbidden, "此卡密不参与刮刮乐活动"}
	}
	return card, nil
}
