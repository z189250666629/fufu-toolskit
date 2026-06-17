package mcycore

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"fufu/rawconv"
	"math"
	"sort"
	"strings"
)

func Signature(data map[string]any, secret string) string {
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
		value, ok := SignatureValue(data[key])
		if !ok {
			continue
		}
		parts = append(parts, key+"="+value)
	}
	base := strings.Join(parts, "&") + "&key=" + secret
	sum := md5.Sum([]byte(base))
	return hex.EncodeToString(sum[:])
}

func SignatureValue(value any) (string, bool) {
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

func Encrypt(plaintext, key16 string) (string, error) {
	block, err := aes.NewCipher([]byte(key16))
	if err != nil {
		return "", err
	}
	padded := PKCS7Pad([]byte(plaintext), aes.BlockSize)
	out := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, []byte(key16)).CryptBlocks(out, padded)
	return base64.StdEncoding.EncodeToString(out), nil
}

func Decrypt(ciphertext, key16 string) (string, error) {
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
	out, err = PKCS7Unpad(out, aes.BlockSize)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func PKCS7Pad(input []byte, blockSize int) []byte {
	padding := blockSize - len(input)%blockSize
	return append(input, bytes.Repeat([]byte{byte(padding)}, padding)...)
}

func PKCS7Unpad(input []byte, blockSize int) ([]byte, error) {
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

func PayloadOK(data map[string]any) bool {
	if data == nil {
		return false
	}
	if value, ok := data["code"]; ok {
		return rawconv.Int(value) == 200
	}
	if value, ok := data["success"].(bool); ok {
		return value
	}
	return true
}

func PayloadMessage(data map[string]any, fallback string) string {
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

func RequestPath(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	if strings.HasPrefix(endpoint, "/") {
		return endpoint
	}
	return "/" + endpoint
}
