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

// queryMCYUsableStock returns how many cards of (itemID, skuID) are currently
// usable (status:0 — unsold) on the MCY shop. This is the real shop inventory
// 补卡 tops up against. It uses the encrypted card/get protocol (matching the
// fufu-shop tool and the card/add upload path); the per-SKU usable count is the
// data.total of the status:0-filtered query.
func queryMCYUsableStock(ctx context.Context, itemID, skuID int) (int, error) {
	if !mcyConfigured() {
		return 0, fmt.Errorf("%w: MCY 未配置", ErrShopRequestFailed)
	}
	if err := ensureMCYCookie(ctx); err != nil {
		return 0, fmt.Errorf("%w: %v", ErrShopLoginFailed, err)
	}
	payload := map[string]any{"item_id": itemID, "sku_id": skuID, "status": 0, "page": 1, "limit": 1}
	staleCookie := getMCYCookie()
	data, err := mcyEncryptedPost(ctx, "/plugin/virtual-card-ship/card/get", payload)
	if err != nil {
		if !isMCYAuthError(err) {
			return 0, classifyShopRequestError(err)
		}
		if err := refreshMCYCookie(ctx, staleCookie); err != nil {
			return 0, fmt.Errorf("%w: %v", ErrShopLoginFailed, err)
		}
		data, err = mcyEncryptedPost(ctx, "/plugin/virtual-card-ship/card/get", payload)
		if err != nil {
			return 0, classifyShopRequestError(err)
		}
	}
	if !mcyPayloadOK(data) {
		return 0, fmt.Errorf("%w: %s", ErrShopRequestFailed, mcyPayloadMessage(data, "MCY 库存查询失败"))
	}
	return mcyUsableCount(data), nil
}

// mcyUsableCount reads the per-SKU usable count from a card/get response. The
// status:0 filter makes data.total the count of unsold cards for that SKU.
func mcyUsableCount(data map[string]any) int {
	if d, ok := data["data"].(map[string]any); ok {
		if total, ok := d["total"]; ok {
			return rawconv.Int(total)
		}
	}
	return 0
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
