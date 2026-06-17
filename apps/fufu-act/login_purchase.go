package activityapp

import (
	"context"
	"errors"
	"net/http"
	"time"

	"fufu/tokens"
)

func createLoginCard(ctx context.Context, key, client string, now time.Time) (Card, error) {
	t, err := lookupLoginToken(ctx, key, client, now)
	if err != nil {
		return Card{}, err
	}
	plan, err := prepareLoginCardPlan(ctx, key, t)
	if err != nil {
		return Card{}, err
	}
	if err := insertLoginCard(plan); err != nil {
		return Card{}, err
	}
	card, ok, err := lookupCard(key)
	if err != nil {
		return Card{}, err
	}
	if !ok {
		return Card{}, errors.New("inserted login card not found")
	}
	return card, nil
}

func prepareLoginCardPlan(ctx context.Context, key string, t *tokens.Token) (loginCardPlan, error) {
	shop := ShopPurchaseLookup{}
	if _, isActTest := parseActTestTokenName(t.Name); !isActTest {
		var err error
		shop, err = findShopPurchase(ctx, key)
		if err != nil {
			return loginCardPlan{}, httpErr{http.StatusBadGateway, "店铺查询失败，请稍后再试"}
		}
	}
	return planLoginCardForToken(key, t, shop, SnapshotRuntimeConfig(), loginTokenQuotaUnit())
}
