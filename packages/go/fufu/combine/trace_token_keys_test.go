package combine

import "testing"

func TestTraceTokenKeysNormalizesHashAndMask(t *testing.T) {
	parts := traceTokenKeys(ResolvedToken{Key: " abcdefghijkl "})

	if parts.full != "sk-abcdefghijkl" {
		t.Fatalf("full key = %q", parts.full)
	}
	if parts.hash != keyHash("sk-abcdefghijkl") {
		t.Fatalf("hash = %q", parts.hash)
	}
	if parts.mask != "sk-abcd…ijkl" {
		t.Fatalf("mask = %q", parts.mask)
	}
}
