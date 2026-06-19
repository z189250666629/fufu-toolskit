package activityapp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"fufu/webutil"
	"io"
	"net/http"
	"time"
)

const maxMCYResponseBodyBytes int64 = 16 << 20

var mcyHTTPClient = &http.Client{Timeout: 15 * time.Second}

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
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, mcyHTTPError{status: resp.StatusCode}
	}
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
