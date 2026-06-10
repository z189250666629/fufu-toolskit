package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleModelTestMasksUnexpectedErrors(t *testing.T) {
	oldRunModelTest := runModelTest
	t.Cleanup(func() { runModelTest = oldRunModelTest })
	runModelTest = func(ctx context.Context, siteName, model, group string) (map[string]any, error) {
		return nil, errors.New("internal failure sk-secret http://10.0.0.5/config")
	}
	req := httptest.NewRequest(http.MethodPost, "/api/newapi/model-status/test", strings.NewReader(`{"siteName":"site-a","model":"gpt-test"}`))
	w := httptest.NewRecorder()

	handleModelTest(w, req)

	body := strings.TrimSpace(w.Body.String())
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("code=%d body=%s", w.Code, body)
	}
	if body != `{"error":"模型测试失败，请稍后重试"}` {
		t.Fatalf("unexpected safe error body: %s", body)
	}
	for _, leaked := range []string{"sk-secret", "10.0.0.5", "internal failure"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("unexpected model-test error leaked %q in %s", leaked, body)
		}
	}
}

func TestHandleModelTestPassesRequestContext(t *testing.T) {
	oldRunModelTest := runModelTest
	t.Cleanup(func() { runModelTest = oldRunModelTest })
	runModelTest = func(ctx context.Context, siteName, model, group string) (map[string]any, error) {
		if err := ctx.Err(); !errors.Is(err, context.Canceled) {
			t.Fatalf("runModelTest should receive request context cancellation, got %v", err)
		}
		return map[string]any{"ok": true}, nil
	}
	req := httptest.NewRequest(http.MethodPost, "/api/newapi/model-status/test", strings.NewReader(`{"siteName":"site-a","model":"gpt-test"}`))
	ctx, cancel := context.WithCancel(req.Context())
	cancel()
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handleModelTest(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
}
