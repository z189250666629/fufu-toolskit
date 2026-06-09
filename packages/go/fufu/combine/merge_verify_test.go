package combine

import (
	"strings"
	"testing"
)

func TestValidateVerifiedSourceTokenAcceptsMatchingEnabledToken(t *testing.T) {
	req := &ResolvedToken{ID: 1, Key: "sk-source"}
	verified := ResolvedToken{ID: 1, Key: "source", Status: 1}

	if err := validateVerifiedSourceToken(req, verified); err != nil {
		t.Fatalf("validateVerifiedSourceToken err = %v", err)
	}
}

func TestValidateVerifiedSourceTokenRejectsMissingRequest(t *testing.T) {
	err := validateVerifiedSourceToken(nil, ResolvedToken{ID: 7, Key: "sk-source", Status: 1})
	if err == nil || !strings.Contains(err.Error(), "Token 7 校验失败") {
		t.Fatalf("missing request err = %v", err)
	}
}

func TestValidateVerifiedSourceTokenRejectsKeyMismatch(t *testing.T) {
	req := &ResolvedToken{ID: 1, Key: "sk-source"}
	verified := ResolvedToken{ID: 1, Key: "sk-other", Status: 1}

	err := validateVerifiedSourceToken(req, verified)
	if err == nil || !strings.Contains(err.Error(), "sk-source 校验失败") {
		t.Fatalf("key mismatch err = %v", err)
	}
}

func TestValidateVerifiedSourceTokenRejectsDisabledToken(t *testing.T) {
	req := &ResolvedToken{ID: 1, Key: "sk-source"}
	verified := ResolvedToken{ID: 1, Key: "sk-source", Status: 2}

	err := validateVerifiedSourceToken(req, verified)
	if err == nil || !strings.Contains(err.Error(), "已被禁用") {
		t.Fatalf("disabled token err = %v", err)
	}
}
