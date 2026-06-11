package main

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"fufu/rawconv"
	"fufu/webutil"
	"io"
	"math"
	"net/http"
	"sort"
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
	keys := make([]string, 0, len(data))
	for key := range data {
		if key == "sign" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := []string{}
	for _, key := range keys {
		value, ok := mcySignatureValue(data[key])
		if !ok {
			continue
		}
		parts = append(parts, key+"="+value)
	}
	base := strings.Join(parts, "&") + "&key=" + secret
	sum := md5.Sum([]byte(base))
	return hex.EncodeToString(sum[:])
}

func mcySignatureValue(value any) (string, bool) {
	switch v := value.(type) {
	case nil:
		return "", false
	case string:
		v = strings.TrimSpace(v)
		if v == "" {
			return "", false
		}
		return v, true
	case json.Number:
		if strings.TrimSpace(v.String()) == "" {
			return "", false
		}
		return v.String(), true
	case float32:
		if math.IsNaN(float64(v)) {
			return "", false
		}
		return fmt.Sprint(v), true
	case float64:
		if math.IsNaN(v) {
			return "", false
		}
		return fmt.Sprint(v), true
	case []any, []string, map[string]any:
		return "", false
	default:
		return fmt.Sprint(v), true
	}
}

func mcyEncrypt(plaintext, key16 string) (string, error) {
	block, err := aes.NewCipher([]byte(key16))
	if err != nil {
		return "", err
	}
	padded := pkcs7Pad([]byte(plaintext), aes.BlockSize)
	out := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, []byte(key16)).CryptBlocks(out, padded)
	return base64.StdEncoding.EncodeToString(out), nil
}

func mcyDecrypt(ciphertext, key16 string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher([]byte(key16))
	if err != nil {
		return "", err
	}
	if len(raw) == 0 || len(raw)%aes.BlockSize != 0 {
		return "", errors.New("invalid ciphertext block size")
	}
	out := make([]byte, len(raw))
	cipher.NewCBCDecrypter(block, []byte(key16)).CryptBlocks(out, raw)
	out, err = pkcs7Unpad(out, aes.BlockSize)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func pkcs7Pad(input []byte, blockSize int) []byte {
	padding := blockSize - len(input)%blockSize
	return append(input, bytes.Repeat([]byte{byte(padding)}, padding)...)
}

func pkcs7Unpad(input []byte, blockSize int) ([]byte, error) {
	if len(input) == 0 || len(input)%blockSize != 0 {
		return nil, errors.New("invalid padding size")
	}
	padding := int(input[len(input)-1])
	if padding == 0 || padding > blockSize || padding > len(input) {
		return nil, errors.New("invalid padding")
	}
	for _, item := range input[len(input)-padding:] {
		if int(item) != padding {
			return nil, errors.New("invalid padding bytes")
		}
	}
	return input[:len(input)-padding], nil
}

func mcyPayloadOK(data map[string]any) bool {
	if data == nil {
		return false
	}
	if value, ok := data["code"]; ok {
		return rawconv.Int(value) == http.StatusOK
	}
	if value, ok := data["success"].(bool); ok {
		return value
	}
	return true
}

func mcyPayloadMessage(data map[string]any, fallback string) string {
	for _, key := range []string{"msg", "message", "error"} {
		if value, ok := data[key]; ok {
			text := strings.TrimSpace(fmt.Sprint(value))
			if text != "" && text != "<nil>" {
				return text
			}
		}
	}
	return fallback
}

func requestPath(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	if strings.HasPrefix(endpoint, "/") {
		return endpoint
	}
	return "/" + endpoint
}
