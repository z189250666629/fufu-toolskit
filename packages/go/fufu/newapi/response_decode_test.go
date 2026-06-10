package newapi

import (
	"encoding/json"
	"testing"
)

func TestDecodeResponsePayloadUsesJSONNumbers(t *testing.T) {
	data, err := decodeResponsePayload([]byte(`{"success":true,"quota":1234567890123456789}`))
	if err != nil {
		t.Fatal(err)
	}

	if data["success"] != true {
		t.Fatalf("success = %#v", data["success"])
	}
	if quota, ok := data["quota"].(json.Number); !ok || quota.String() != "1234567890123456789" {
		t.Fatalf("quota = %#v", data["quota"])
	}
}

func TestDecodeResponsePayloadToleratesEmptyBody(t *testing.T) {
	data, err := decodeResponsePayload([]byte("  "))
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 0 {
		t.Fatalf("empty payload = %#v", data)
	}
}

func TestDecodeResponsePayloadReportsInvalidJSON(t *testing.T) {
	data, err := decodeResponsePayload([]byte("not-json"))
	if err == nil {
		t.Fatalf("expected invalid JSON error, got data=%#v", data)
	}
	if data != nil {
		t.Fatalf("invalid payload data = %#v", data)
	}
}

func TestDecodeResponsePayloadRejectsTrailingJSONValue(t *testing.T) {
	data, err := decodeResponsePayload([]byte(`{"success":true} {}`))
	if err == nil {
		t.Fatalf("expected trailing JSON error, got data=%#v", data)
	}
	if data != nil {
		t.Fatalf("trailing JSON payload = %#v", data)
	}
}

func TestDecodeResponsePayloadAllowsTrailingWhitespace(t *testing.T) {
	data, err := decodeResponsePayload([]byte("{\"success\":true}\n\t "))
	if err != nil {
		t.Fatal(err)
	}

	if data["success"] != true {
		t.Fatalf("payload with trailing whitespace = %#v", data)
	}
}
