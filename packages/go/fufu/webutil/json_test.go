package webutil

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDecodeJSONDecodesStruct(t *testing.T) {
	var got struct {
		Name string `json:"name"`
	}

	if err := DecodeJSON(strings.NewReader(`{"name":"card"}`), &got); err != nil {
		t.Fatalf("DecodeJSON: %v", err)
	}
	if got.Name != "card" {
		t.Fatalf("name = %q", got.Name)
	}
}

func TestDecodeJSONUseNumberOption(t *testing.T) {
	var got map[string]any

	if err := DecodeJSON(strings.NewReader(`{"n":1234567890123456789}`), &got, WithUseNumber()); err != nil {
		t.Fatalf("DecodeJSON: %v", err)
	}
	if n, ok := got["n"].(json.Number); !ok || n.String() != "1234567890123456789" {
		t.Fatalf("number = %#v", got["n"])
	}
}

func TestDecodeJSONReturnsInvalidJSONError(t *testing.T) {
	var got map[string]any

	if err := DecodeJSON(strings.NewReader(`{"bad"`), &got); err == nil {
		t.Fatalf("expected invalid json error")
	}
}

func TestDecodeJSONRejectsTrailingJSONValue(t *testing.T) {
	var got map[string]any

	if err := DecodeJSON(strings.NewReader(`{"name":"card"} {}`), &got); err == nil {
		t.Fatal("expected trailing JSON value to be rejected")
	}
}

func TestDecodeJSONAllowsTrailingWhitespace(t *testing.T) {
	var got map[string]any

	if err := DecodeJSON(strings.NewReader("{\"name\":\"card\"}\n\t "), &got); err != nil {
		t.Fatalf("DecodeJSON: %v", err)
	}
	if got["name"] != "card" {
		t.Fatalf("name = %#v", got["name"])
	}
}
