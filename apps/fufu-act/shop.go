package activityapp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"fufu/config"
	"fufu/rawconv"
	"fufu/webutil"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const maxMCYResponseBodyBytes int64 = 16 << 20

var mcyHTTPClient = &http.Client{Timeout: 15 * time.Second}
var mcyCookieMu sync.RWMutex
var mcyLoginMu sync.Mutex

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

const (
	mcyCardGetEndpoint   = "/plugin/virtual-card-ship/card/get"
	mcyStockScanPageSize = 200
	// mcyStockScanMaxPages caps the scan so a shop that never reports a usable
	// count can't make the refresh page through the entire (20k+) card history.
	mcyStockScanMaxPages = 400
)

// queryMCYUsableStock returns how many cards of (itemID, skuID) are currently
// usable (unsold) on the MCY shop — the inventory 补卡 tops up against.
//
// The shop's card/get endpoint IGNORES its item_id/sku_id/status filters: every
// query returns the global card list and the global data.total, so a single
// filtered query can't yield a per-SKU count. We instead scan the global list
// once and bucket the unsold cards by item:sku — see mcyUsableStockBySKU.
func queryMCYUsableStock(ctx context.Context, itemID, skuID int) (int, error) {
	counts, err := mcyUsableStockBySKU(ctx)
	if err != nil {
		return 0, err
	}
	return counts[skuStockKey(itemID, skuID)], nil
}

// mcyUsableStockBySKU scans the MCY card list (newest first) and counts unsold
// (status:0) cards per "item:sku", stopping as soon as the running total reaches
// the shop's global usable count (card_usable_count) — unsold cards cluster at
// the top, so this terminates long before the full history. Requests are
// SEQUENTIAL: the shop rejects concurrent calls on one session with 登录已过期.
func mcyUsableStockBySKU(ctx context.Context) (map[string]int, error) {
	if !mcyConfigured() {
		return nil, fmt.Errorf("%w: MCY 未配置", ErrShopRequestFailed)
	}
	if err := ensureMCYCookie(ctx); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrShopLoginFailed, err)
	}
	counts := map[string]int{}
	found, target := 0, -1
	for page := 1; page <= mcyStockScanMaxPages; page++ {
		data, err := mcyCardGetPage(ctx, page, mcyStockScanPageSize)
		if err != nil {
			return nil, err
		}
		if target < 0 {
			target = mcyGlobalUsableCount(data)
		}
		list := mcyCardList(data)
		if len(list) == 0 {
			break
		}
		found += tallyUsableBySKU(list, counts)
		if target >= 0 && found >= target {
			break
		}
		if len(list) < mcyStockScanPageSize {
			break
		}
	}
	return counts, nil
}

// mcyCardGetPage fetches one page of the card list over the encrypted protocol,
// re-authenticating once on either an HTTP auth error or a body-level 登录已过期
// (the shop signals an expired session with HTTP 200 + code!=200, not a 401).
func mcyCardGetPage(ctx context.Context, page, limit int) (map[string]any, error) {
	payload := map[string]any{"page": page, "limit": limit}
	staleCookie := getMCYCookie()
	data, err := mcyEncryptedPost(ctx, mcyCardGetEndpoint, payload)
	if err != nil {
		if !isMCYAuthError(err) {
			return nil, classifyShopRequestError(err)
		}
		if data, err = mcyRetryCardGetAfterRelogin(ctx, staleCookie, payload); err != nil {
			return nil, err
		}
	}
	if mcyPayloadOK(data) {
		return data, nil
	}
	if !mcyIsSessionExpired(data) {
		return nil, fmt.Errorf("%w: %s", ErrShopRequestFailed, mcyPayloadMessage(data, "MCY 库存查询失败"))
	}
	if data, err = mcyRetryCardGetAfterRelogin(ctx, staleCookie, payload); err != nil {
		return nil, err
	}
	if !mcyPayloadOK(data) {
		return nil, fmt.Errorf("%w: %s", ErrShopRequestFailed, mcyPayloadMessage(data, "MCY 库存查询失败"))
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

func skuStockKey(itemID, skuID int) string {
	return fmt.Sprintf("%d:%d", itemID, skuID)
}

// mcyGlobalUsableCount reads the shop-wide unsold count (card_usable_count) every
// card/get response carries; -1 means the field was absent (unknown).
func mcyGlobalUsableCount(data map[string]any) int {
	if v, ok := data["card_usable_count"]; ok {
		return rawconv.Int(v)
	}
	return -1
}

// mcyCardList returns a card/get page's card records (data.list).
func mcyCardList(data map[string]any) []any {
	if d, ok := data["data"].(map[string]any); ok {
		if list, ok := d["list"].([]any); ok {
			return list
		}
	}
	return nil
}

// tallyUsableBySKU adds each unsold (status:0) card on the page into counts keyed
// by item:sku and returns how many it added.
func tallyUsableBySKU(list []any, counts map[string]int) int {
	added := 0
	for _, item := range list {
		card, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if rawconv.Int(card["status"]) != 0 {
			continue
		}
		counts[skuStockKey(rawconv.Int(card["item_id"]), rawconv.Int(card["sku_id"]))]++
		added++
	}
	return added
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

func mcyConfig() (string, string, string, string) {
	c := getMCYRuntimeConfig()
	base := strings.TrimRight(firstNonEmpty(c.BaseURL, config.Env("MCY_BASE_URL"), config.Env("SHOP_BASE_URL")), "/")
	user := firstNonEmpty(c.Username, config.Env("MCY_USERNAME"), config.Env("SHOP_USERNAME"))
	pass := firstNonEmpty(c.Password, config.Env("MCY_PASSWORD"), config.Env("SHOP_PASSWORD"))
	login := firstNonEmpty(c.LoginEndpoint, config.Env("MCY_LOGIN_ENDPOINT"), "/admin/login")
	return base, user, pass, login
}

func getMCYCookie() string {
	mcyCookieMu.RLock()
	defer mcyCookieMu.RUnlock()
	return mcyCookie
}

func setMCYCookie(value string) {
	mcyCookieMu.Lock()
	mcyCookie = value
	mcyCookieMu.Unlock()
}

func ensureMCYCookie(ctx context.Context) error {
	if getMCYCookie() != "" {
		return nil
	}
	mcyLoginMu.Lock()
	defer mcyLoginMu.Unlock()
	if getMCYCookie() != "" {
		return nil
	}
	if err := mcyLogin(ctx); err != nil {
		return err
	}
	if getMCYCookie() == "" {
		return ErrShopLoginFailed
	}
	return nil
}

func refreshMCYCookie(ctx context.Context, staleCookie string) error {
	mcyLoginMu.Lock()
	defer mcyLoginMu.Unlock()
	if current := getMCYCookie(); current != "" && current != staleCookie {
		return nil
	}
	setMCYCookie("")
	if err := mcyLogin(ctx); err != nil {
		return err
	}
	if getMCYCookie() == "" {
		return ErrShopLoginFailed
	}
	return nil
}

type mcyHTTPError struct {
	status int
}

func (e mcyHTTPError) Error() string {
	return fmt.Sprintf("MCY HTTP %d", e.status)
}

func isMCYAuthError(err error) bool {
	var httpErr mcyHTTPError
	return errors.As(err, &httpErr) && (httpErr.status == http.StatusUnauthorized || httpErr.status == http.StatusForbidden)
}

func mcyLogin(ctx context.Context) error {
	base, user, pass, login := mcyConfig()
	if base == "" || user == "" || pass == "" {
		return fmt.Errorf("missing MCY config")
	}
	jsonErr := mcyLoginJSON(ctx, base, login, user, pass)
	if jsonErr == nil && getMCYCookie() != "" {
		return nil
	}
	if jsonErr != nil && !shouldTryEncryptedMCYLogin(jsonErr) {
		return jsonErr
	}
	var lastErr error
	if jsonErr != nil {
		lastErr = jsonErr
	}
	for _, endpoint := range encryptedMCYLoginEndpoints(login) {
		if err := mcyLoginEncrypted(ctx, base, endpoint, user, pass); err != nil {
			lastErr = err
			if !shouldTryEncryptedMCYLogin(err) {
				return err
			}
			continue
		}
		if getMCYCookie() != "" {
			return nil
		}
	}
	if lastErr != nil {
		return lastErr
	}
	return ErrShopLoginFailed
}

func mcyLoginJSON(ctx context.Context, base, login, user, pass string) error {
	body, _ := json.Marshal(map[string]string{"username": user, "password": pass})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(base, "/")+requestPath(login), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := mcyHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return mcyHTTPError{status: resp.StatusCode}
	}
	if setMCYCookieFromHTTPCookies(resp.Cookies(), "") {
		return nil
	}
	var data map[string]any
	if err := decodeMCYResponse(resp.Body, &data); err == nil {
		setMCYCookieFromHTTPCookies(nil, mcyLoginToken(data))
	}
	return nil
}

func mcyLoginEncrypted(ctx context.Context, base, endpoint, user, pass string) error {
	data, cookies, err := mcyEncryptedRequest(ctx, base, endpoint, map[string]any{"email": user, "password": pass}, "")
	if err != nil {
		return err
	}
	if !mcyPayloadOK(data) {
		return fmt.Errorf("%w: %s", ErrShopLoginFailed, mcyPayloadMessage(data, "MCY encrypted login failed"))
	}
	setMCYCookieFromHTTPCookies(cookies, mcyLoginToken(data))
	return nil
}

func encryptedMCYLoginEndpoints(login string) []string {
	login = requestPath(login)
	candidates := []string{}
	if login == "/admin/login" {
		candidates = append(candidates, "/admin", login)
	} else {
		candidates = append(candidates, login, "/admin")
	}
	seen := map[string]bool{}
	out := []string{}
	for _, item := range candidates {
		item = requestPath(item)
		if !seen[item] {
			seen[item] = true
			out = append(out, item)
		}
	}
	return out
}

func shouldTryEncryptedMCYLogin(err error) bool {
	if err == nil {
		return true
	}
	if errors.Is(err, ErrShopInvalidResponse) || errors.Is(err, ErrShopLoginFailed) {
		return true
	}
	var httpErr mcyHTTPError
	if errors.As(err, &httpErr) {
		switch httpErr.status {
		case http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusMethodNotAllowed, http.StatusUnsupportedMediaType:
			return true
		}
	}
	return false
}

func setMCYCookieFromHTTPCookies(cookies []*http.Cookie, token string) bool {
	parts := make([]string, 0, len(cookies)+1)
	for _, cookie := range cookies {
		if cookie.Name == "" {
			continue
		}
		parts = append(parts, cookie.Name+"="+cookie.Value)
	}
	if strings.TrimSpace(token) != "" {
		parts = append(parts, "manage_token="+url.QueryEscape(strings.TrimSpace(token)))
	}
	if len(parts) == 0 {
		return false
	}
	setMCYCookie(strings.Join(parts, "; "))
	return true
}

func mcyLoginToken(data map[string]any) string {
	if data == nil {
		return ""
	}
	if nested, ok := data["data"].(map[string]any); ok {
		if token := strings.TrimSpace(fmt.Sprint(nested["token"])); token != "" && token != "<nil>" {
			return token
		}
	}
	if token := strings.TrimSpace(fmt.Sprint(data["token"])); token != "" && token != "<nil>" {
		return token
	}
	return ""
}

func mcyPost(ctx context.Context, endpoint string, payload any) (map[string]any, error) {
	base, _, _, _ := mcyConfig()
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+endpoint, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cookie", getMCYCookie())
	resp, err := mcyHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var data map[string]any
	if err := decodeMCYResponse(resp.Body, &data); err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return data, mcyHTTPError{status: resp.StatusCode}
	}
	return data, nil
}

func decodeMCYResponse(r io.Reader, out any) error {
	limited := &io.LimitedReader{R: r, N: maxMCYResponseBodyBytes + 1}
	if err := webutil.DecodeJSON(limited, out, webutil.WithUseNumber()); err != nil {
		if limited.N <= 0 {
			return fmt.Errorf("%w: response body too large", ErrShopInvalidResponse)
		}
		return fmt.Errorf("%w: %v", ErrShopInvalidResponse, err)
	}
	if limited.N <= 0 {
		return fmt.Errorf("%w: response body too large", ErrShopInvalidResponse)
	}
	return nil
}
