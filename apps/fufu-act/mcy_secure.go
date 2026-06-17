package activityapp

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"fufu/mcycore"
	"fufu/webutil"
	"io"
	"net/http"
	"strings"
	"time"
)

func mcyEncryptedPost(ctx context.Context, endpoint string, payload map[string]any) (map[string]any, error) {
	base, _, _, _ := mcyConfig()
	data, _, err := mcyEncryptedRequest(ctx, base, endpoint, payload, getMCYCookie())
	return data, err
}

func mcyEncryptedRequest(ctx context.Context, base, endpoint string, payload map[string]any, cookie string) (map[string]any, []*http.Cookie, error) {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if base == "" {
		return nil, nil, fmt.Errorf("missing MCY config")
	}
	secret := newMCYSecret()
	key16 := secret[:16]
	bodyJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, nil, err
	}
	encryptedBody, err := mcyEncrypt(string(bodyJSON), key16)
	if err != nil {
		return nil, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+requestPath(endpoint), strings.NewReader(encryptedBody))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("Secret", secret)
	req.Header.Set("Signature", mcySignature(payload, secret))
	if strings.TrimSpace(cookie) != "" {
		req.Header.Set("Cookie", cookie)
	}
	resp, err := mcyHTTPClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	data, err := decodeMCYEncryptedResponse(resp)
	if err != nil {
		return data, resp.Cookies(), err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return data, resp.Cookies(), mcyHTTPError{status: resp.StatusCode}
	}
	return data, resp.Cookies(), nil
}

func decodeMCYEncryptedResponse(resp *http.Response) (map[string]any, error) {
	limited := &io.LimitedReader{R: resp.Body, N: maxMCYResponseBodyBytes + 1}
	raw, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if limited.N <= 0 {
		return nil, fmt.Errorf("%w: response body too large", ErrShopInvalidResponse)
	}
	if strings.TrimSpace(resp.Header.Get("Secret")) != "" {
		secret := strings.TrimSpace(resp.Header.Get("Secret"))
		if len(secret) < 16 {
			return nil, fmt.Errorf("%w: MCY response Secret header too short", ErrShopInvalidResponse)
		}
		plain, err := mcyDecrypt(strings.TrimSpace(string(raw)), secret[:16])
		if err != nil {
			return nil, fmt.Errorf("%w: decrypt MCY response: %v", ErrShopInvalidResponse, err)
		}
		raw = []byte(plain)
	}
	var data map[string]any
	if err := webutil.DecodeJSON(bytes.NewReader(raw), &data, webutil.WithUseNumber()); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrShopInvalidResponse, err)
	}
	return data, nil
}

func newMCYSecret() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err == nil {
		return hex.EncodeToString(buf)
	}
	sum := md5.Sum([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
	return hex.EncodeToString(sum[:])
}

func mcySignature(data map[string]any, secret string) string {
	return mcycore.Signature(data, secret)
}

func mcySignatureValue(value any) (string, bool) {
	return mcycore.SignatureValue(value)
}

func mcyEncrypt(plaintext, key16 string) (string, error) {
	return mcycore.Encrypt(plaintext, key16)
}

func mcyDecrypt(ciphertext, key16 string) (string, error) {
	return mcycore.Decrypt(ciphertext, key16)
}

func pkcs7Pad(input []byte, blockSize int) []byte {
	return mcycore.PKCS7Pad(input, blockSize)
}

func pkcs7Unpad(input []byte, blockSize int) ([]byte, error) {
	return mcycore.PKCS7Unpad(input, blockSize)
}

func mcyPayloadOK(data map[string]any) bool {
	return mcycore.PayloadOK(data)
}

func mcyPayloadMessage(data map[string]any, fallback string) string {
	return mcycore.PayloadMessage(data, fallback)
}

func requestPath(endpoint string) string {
	return mcycore.RequestPath(endpoint)
}
