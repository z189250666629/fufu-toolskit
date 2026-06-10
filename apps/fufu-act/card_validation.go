package main

import (
	"context"
	"net/http"
)

func requireCurrentTokenActive(ctx context.Context, key string) error {
	if tokenSvc == nil {
		return nil
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
