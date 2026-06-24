package activityapp

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var (
	ErrShopLoginFailed       = errors.New("shop login failed")
	ErrShopCredentialInvalid = errors.New("shop credential invalid")
	ErrShopRequestFailed     = errors.New("shop request failed")
	ErrShopInvalidResponse   = errors.New("shop invalid response")
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
	data, err := mcyPost(ctx, "/plugin/virtual-card-ship/card/get", shopPurchasePayload(cardKey))
	if err != nil {
		return lookup, classifyShopRequestError(err)
	}
	if !mcyPayloadOK(data) {
		if mcyIsSessionExpired(data) {
			if err := refreshMCYCookie(ctx, staleCookie); err != nil {
				return lookup, fmt.Errorf("%w: %v", ErrShopLoginFailed, err)
			}
			data, err = mcyPost(ctx, "/plugin/virtual-card-ship/card/get", shopPurchasePayload(cardKey))
			if err != nil {
				return lookup, classifyShopRequestError(err)
			}
			if !mcyPayloadOK(data) {
				return lookup, fmt.Errorf("%w: %s", ErrShopRequestFailed, mcyPayloadMessage(data, "MCY 卡密查询失败"))
			}
		} else {
			return lookup, fmt.Errorf("%w: %s", ErrShopRequestFailed, mcyPayloadMessage(data, "MCY 卡密查询失败"))
		}
	}
	if d, ok := data["data"].(map[string]any); ok {
		lookup.PurchaseTime = extractPurchaseTime(d)
		return lookup, nil
	}
	return lookup, ErrShopInvalidResponse
}

func shopPurchasePayload(cardKey string) map[string]any {
	return map[string]any{"equal-card": cardKey, "page": 1, "limit": 1}
}

func classifyShopRequestError(err error) error {
	if errors.Is(err, ErrShopInvalidResponse) {
		return err
	}
	if isMCYAuthError(err) {
		return mcyCredentialError()
	}
	return fmt.Errorf("%w: %v", ErrShopRequestFailed, err)
}

func mcyCredentialError() error {
	return fmt.Errorf("%w: %w: MCY 登录失败：请检查商城账号或密码", ErrShopLoginFailed, ErrShopCredentialInvalid)
}

func mcyMessageLooksCredentialInvalid(message string) bool {
	message = strings.ToLower(strings.TrimSpace(message))
	if message == "" {
		return false
	}
	for _, marker := range []string{"密码", "账号", "账户", "用户名", "认证", "unauthorized", "forbidden", "invalid credential"} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
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
