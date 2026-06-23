package activityapp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

func mcyLogin(ctx context.Context) error {
	base, user, pass, login := mcyConfig()
	if base == "" || user == "" || pass == "" {
		return missingMCYConfigError()
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
	req.Header.Set("Connection", "close")
	req.Close = true
	resp, err := mcyHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return mcyCredentialError()
		}
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
		if isMCYAuthError(err) {
			return mcyCredentialError()
		}
		return err
	}
	if !mcyPayloadOK(data) {
		message := mcyPayloadMessage(data, "MCY encrypted login failed")
		if mcyMessageLooksCredentialInvalid(message) {
			return mcyCredentialError()
		}
		return fmt.Errorf("%w: %s", ErrShopLoginFailed, message)
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
	if errors.Is(err, ErrShopCredentialInvalid) {
		return false
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
