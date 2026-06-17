package mcycore

import "testing"

func TestSignatureValueIncludesZeroAndSkipsBlank(t *testing.T) {
	value, ok := SignatureValue(0)
	if !ok || value != "0" {
		t.Fatalf("SignatureValue(0) = (%q, %v)", value, ok)
	}
	if _, ok := SignatureValue("  "); ok {
		t.Fatal("blank string should be skipped")
	}
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	got, err := Encrypt(`{"ok":true}`, "0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	plain, err := Decrypt(got, "0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	if plain != `{"ok":true}` {
		t.Fatalf("plain = %q", plain)
	}
}

func TestPayloadHelpers(t *testing.T) {
	if !PayloadOK(map[string]any{"code": 200}) || PayloadOK(map[string]any{"success": false}) || PayloadOK(nil) {
		t.Fatalf("unexpected PayloadOK result")
	}
	if got := PayloadMessage(map[string]any{"msg": " bad "}, "fallback"); got != "bad" {
		t.Fatalf("PayloadMessage = %q", got)
	}
	if got := RequestPath("api/login"); got != "/api/login" {
		t.Fatalf("RequestPath = %q", got)
	}
}
