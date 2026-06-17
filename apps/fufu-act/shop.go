package activityapp

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var (
	ErrShopLoginFailed     = errors.New("shop login failed")
	ErrShopRequestFailed   = errors.New("shop request failed")
	ErrShopInvalidResponse = errors.New("shop invalid response")
)

type ShopPurchaseLookup struct {
	Configured   bool
	PurchaseTime string
}

func findShopPurchase(ctx context.Context, cardKey string) (ShopPurchaseLookup, error) {
	lookup := ShopPurchaseLookup{}
	if !mcyConfigured() {
		return lookup, nil
	}
	lookup.Configured = true
	if err := ensureMCYCookie(ctx); err != nil {
		return lookup, fmt.Errorf("%w: %v", ErrShopLoginFailed, err)
	}
	staleCookie := getMCYCookie()
	data, err := mcyPost(ctx, "/plugin/virtual-card-ship/card/get", map[string]any{"equal-card": cardKey, "page": 1, "limit": 1})
	if err != nil {
		if !isMCYAuthError(err) {
			return lookup, classifyShopRequestError(err)
		}
		if err := refreshMCYCookie(ctx, staleCookie); err != nil {
			return lookup, fmt.Errorf("%w: %v", ErrShopLoginFailed, err)
		}
		data, err = mcyPost(ctx, "/plugin/virtual-card-ship/card/get", map[string]any{"equal-card": cardKey, "page": 1, "limit": 1})
		if err != nil {
			return lookup, classifyShopRequestError(err)
		}
	}
	if d, ok := data["data"].(map[string]any); ok {
		lookup.PurchaseTime = extractPurchaseTime(d)
		return lookup, nil
	}
	return lookup, ErrShopInvalidResponse
}

func classifyShopRequestError(err error) error {
	if errors.Is(err, ErrShopInvalidResponse) {
		return err
	}
	return fmt.Errorf("%w: %v", ErrShopRequestFailed, err)
}

func extractPurchaseTime(data map[string]any) string {
	arr, ok := data["list"].([]any)
	if !ok || len(arr) == 0 {
		return ""
	}
	obj, ok := arr[0].(map[string]any)
	if !ok {
		return ""
	}
	purchaseTime, ok := obj["purchase_time"]
	if !ok || purchaseTime == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(purchaseTime))
}
