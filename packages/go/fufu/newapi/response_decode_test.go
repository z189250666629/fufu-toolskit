package newapi

import (
	"encoding/json"
	"testing"
)

func TestDecodeResponsePayloadUsesJSONNumbers(t *testing.T) {
	data := decodeResponsePayload([]byte(`{"success":true,"quota":1234567890123456789}`))

	if data["success"] != true {
		t.Fatalf("success = %#v", data["success"])
	}
	if quota, ok := data["quota"].(json.Number); !ok || quota.String() != "1234567890123456789" {
		t.Fatalf("quota = %#v", data["quota"])
	}
}

func TestDecodeResponsePayloadToleratesEmptyAndInvalidBodies(t *testing.T) {
	if data := decodeResponsePayload([]byte("  ")); len(data) != 0 {
		t.Fatalf("empty payload = %#v", data)
	}
	if data := decodeResponsePayload([]byte("not-json")); len(data) != 0 {
		t.Fatalf("invalid payload = %#v", data)
	}
}
