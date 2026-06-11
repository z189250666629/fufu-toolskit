package activityapp

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"
)

func testDecodeMCYRequest(t *testing.T, body io.Reader, secret string) map[string]any {
	t.Helper()
	if len(secret) < 16 {
		t.Fatalf("Secret header too short: %q", secret)
	}
	raw, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("read MCY body: %v", err)
	}
	decoded := testMCYDecrypt(t, string(raw), secret[:16])
	var payload map[string]any
	if err := json.Unmarshal([]byte(decoded), &payload); err != nil {
		t.Fatalf("decode MCY JSON %q: %v", decoded, err)
	}
	return payload
}

func testWriteEncryptedMCYResponse(t *testing.T, w http.ResponseWriter, payload map[string]any) {
	t.Helper()
	secret := "0123456789abcdef0123456789abcdef"
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal MCY response: %v", err)
	}
	w.Header().Set("Secret", secret)
	_, _ = w.Write([]byte(testMCYEncrypt(t, string(body), secret[:16])))
}

func testMCYEncrypt(t *testing.T, plaintext, key16 string) string {
	t.Helper()
	block, err := aes.NewCipher([]byte(key16))
	if err != nil {
		t.Fatalf("aes cipher: %v", err)
	}
	padded := testPKCS7Pad([]byte(plaintext), aes.BlockSize)
	out := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, []byte(key16)).CryptBlocks(out, padded)
	return base64.StdEncoding.EncodeToString(out)
}

func testMCYDecrypt(t *testing.T, ciphertext, key16 string) string {
	t.Helper()
	raw, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		t.Fatalf("base64 decode: %v", err)
	}
	block, err := aes.NewCipher([]byte(key16))
	if err != nil {
		t.Fatalf("aes cipher: %v", err)
	}
	if len(raw)%aes.BlockSize != 0 {
		t.Fatalf("ciphertext is not full blocks")
	}
	out := make([]byte, len(raw))
	cipher.NewCBCDecrypter(block, []byte(key16)).CryptBlocks(out, raw)
	out, err = testPKCS7Unpad(out, aes.BlockSize)
	if err != nil {
		t.Fatalf("pkcs7 unpad: %v", err)
	}
	return string(out)
}

func testPKCS7Pad(input []byte, blockSize int) []byte {
	padding := blockSize - len(input)%blockSize
	return append(input, bytes.Repeat([]byte{byte(padding)}, padding)...)
}

func testPKCS7Unpad(input []byte, blockSize int) ([]byte, error) {
	if len(input) == 0 || len(input)%blockSize != 0 {
		return nil, errors.New("invalid padding size")
	}
	padding := int(input[len(input)-1])
	if padding == 0 || padding > blockSize || padding > len(input) {
		return nil, errors.New("invalid padding")
	}
	for _, b := range input[len(input)-padding:] {
		if int(b) != padding {
			return nil, errors.New("invalid padding bytes")
		}
	}
	return input[:len(input)-padding], nil
}
