package activityapp

import (
	"context"
	"net/http"
	"strings"
)

func requireCurrentTokenActive(ctx context.Context, key string) error {
	if card, ok, err := lookupCard(key); err == nil && ok && isSubscriptionCard(card) {
		return nil
	} else if err != nil {
		return err
	}

	service, _ := snapshotTokenRuntime()
	if service == nil {
		return httpErr{http.StatusServiceUnavailable, "NewAPI 未配置"}
	}
	token, err := service.SearchTokenByKey(ctx, key)
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

func requireDragonBoatEligibleCard(ctx context.Context, key string) (Card, error) {
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
	if !isDragonBoatDollarTier(card.Dollars) {
		return Card{}, httpErr{http.StatusForbidden, "此卡密不参与端午捕粽活动"}
	}
	return card, nil
}

func isSubscriptionCard(card Card) bool {
	return card.SubscriptionID.Valid || strings.EqualFold(strings.TrimSpace(card.Source), "subscription")
}
