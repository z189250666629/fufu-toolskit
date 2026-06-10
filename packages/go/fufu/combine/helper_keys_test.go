package combine

import (
	"reflect"
	"testing"

	tokenkeys "fufu/tokens"
)

func TestCombineNormalizeKeysMatchesSharedTokenNormalizationForPastedInput(t *testing.T) {
	raw := []string{
		" alpha123456789\nsk-beta123456789, gamma123456789 ",
		"\tsk-alpha123456789  ,\n delta123456789",
	}

	got := normalizeKeys(raw)
	want := tokenkeys.NormalizeKeys(raw)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalizeKeys() = %#v, want shared token normalization %#v", got, want)
	}
}

func TestCombineNormalizeKeysAcceptsFullWidthPunctuationAndQuotedJSONPaste(t *testing.T) {
	raw := []string{`["sk-alpha123456789"， "beta123456789"；"sk-gamma123456789"]`}
	got := normalizeKeys(raw)
	want := tokenkeys.NormalizeKeys(raw)
	if !reflect.DeepEqual(got, want) || len(got) != 3 {
		t.Fatalf("normalizeKeys() = %#v, want shared token normalization %#v", got, want)
	}
}
