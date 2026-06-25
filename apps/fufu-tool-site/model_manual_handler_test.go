package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleModelTestPassesPreferredURLThroughContext(t *testing.T) {
	oldRunModelTest := runModelTest
	t.Cleanup(func() { runModelTest = oldRunModelTest })
	runModelTest = func(ctx context.Context, siteName, model, group string) (map[string]any, error) {
		if got := modelTestPreferredURLFromContext(ctx); got != "https://api-fast.example.test" {
			t.Fatalf("preferred URL = %q", got)
		}
		return map[string]any{"ok": true}, nil
	}
	req := httptest.NewRequest(http.MethodPost, "/api/newapi/model-status/test", strings.NewReader(`{"siteName":"site-a","model":"gpt-test","preferredUrl":" https://api-fast.example.test "}`))
	w := httptest.NewRecorder()

	handleModelTest(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
}
