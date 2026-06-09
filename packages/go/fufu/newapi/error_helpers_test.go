package newapi

import "testing"

func TestMessageFromPayloadReadsTopLevelAndNestedMessages(t *testing.T) {
	if got, ok := messageFromPayload(map[string]any{"message": " bad token "}); !ok || got != "bad token" {
		t.Fatalf("top-level message = %q/%v", got, ok)
	}
	if got, ok := messageFromPayload(map[string]any{"data": map[string]any{"error": "nested bad"}}); !ok || got != "nested bad" {
		t.Fatalf("nested error = %q/%v", got, ok)
	}
	if got, ok := messageFromPayload(map[string]any{"message": nil, "error": ""}); ok || got != "" {
		t.Fatalf("empty message = %q/%v", got, ok)
	}
}
