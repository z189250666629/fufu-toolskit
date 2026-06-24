package activityapp

import (
	"context"
	"fmt"
	"fufu/rawconv"
	"strings"
)

const mcyCardGetEndpoint = "/plugin/virtual-card-ship/card/get"

// queryMCYUsableStock returns how many cards of (itemID, skuID) are currently
// usable (unsold) on the MCY shop — the inventory 补卡 tops up against.
//
// card/get supports a precise filter, but ONLY via `equal-<field>` keys (the
// same convention as the working equal-card purchase lookup); plain
// item_id/sku_id/status are silently ignored and return the global total. With
// equal-item_id/equal-sku_id/equal-status the response's data.total is the exact
// per-SKU usable count, so one query answers one plan — no full-list scan.
//
// Callers must NOT run these concurrently on one session: the shop rejects
// concurrent requests with a body-level 登录已过期.
func queryMCYUsableStock(ctx context.Context, itemID, skuID int) (int, error) {
	if !mcyConfigured() {
		return 0, fmt.Errorf("%w: MCY 未配置", ErrShopRequestFailed)
	}
	if err := ensureMCYCookie(ctx); err != nil {
		return 0, fmt.Errorf("%w: %v", ErrShopLoginFailed, err)
	}
	payload := map[string]any{
		"equal-item_id": itemID,
		"equal-sku_id":  skuID,
		"equal-status":  0, // 0 = unsold
		"page":          1,
		"limit":         1,
	}
	data, err := mcyCardGet(ctx, payload)
	if err != nil {
		return 0, err
	}
	return mcyCardGetTotal(data), nil
}

// mcyCardGet performs an encrypted card/get. It re-authenticates once for the
// shop's body-level 登录已过期 signal (HTTP 200 + code!=200); HTTP 401/403 is
// treated as a credential/configuration problem and surfaced to the admin.
func mcyCardGet(ctx context.Context, payload map[string]any) (map[string]any, error) {
	return mcyCardGetWithFailureMessage(ctx, payload, "MCY 库存查询失败")
}

func mcyCardGetWithFailureMessage(ctx context.Context, payload map[string]any, fallbackMessage string) (map[string]any, error) {
	staleCookie := getMCYCookie()
	data, err := mcyEncryptedPost(ctx, mcyCardGetEndpoint, payload)
	if err != nil {
		return nil, classifyShopRequestError(err)
	}
	if mcyPayloadOK(data) {
		return data, nil
	}
	if !mcyIsSessionExpired(data) {
		return nil, fmt.Errorf("%w: %s", ErrShopRequestFailed, mcyPayloadMessage(data, fallbackMessage))
	}
	if data, err = mcyRetryCardGetAfterRelogin(ctx, staleCookie, payload); err != nil {
		return nil, err
	}
	if !mcyPayloadOK(data) {
		return nil, fmt.Errorf("%w: %s", ErrShopRequestFailed, mcyPayloadMessage(data, fallbackMessage))
	}
	return data, nil
}

func mcyRetryCardGetAfterRelogin(ctx context.Context, staleCookie string, payload map[string]any) (map[string]any, error) {
	if err := refreshMCYCookie(ctx, staleCookie); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrShopLoginFailed, err)
	}
	data, err := mcyEncryptedPost(ctx, mcyCardGetEndpoint, payload)
	if err != nil {
		return nil, classifyShopRequestError(err)
	}
	return data, nil
}

// mcyCardGetTotal reads data.total — with an equal-* filter, the count of cards
// matching that filter (i.e. the per-SKU usable count).
func mcyCardGetTotal(data map[string]any) int {
	if d, ok := data["data"].(map[string]any); ok {
		if total, ok := d["total"]; ok {
			return rawconv.Int(total)
		}
	}
	return 0
}

// mcyIsSessionExpired reports whether a non-OK payload signals an expired login
// (the shop returns HTTP 200 + code!=200 + 登录已过期 instead of a 401).
func mcyIsSessionExpired(data map[string]any) bool {
	msg := mcyPayloadMessage(data, "")
	for _, marker := range []string{"登录已过期", "登录失效", "未登录", "请登录", "请先登录"} {
		if strings.Contains(msg, marker) {
			return true
		}
	}
	return false
}
