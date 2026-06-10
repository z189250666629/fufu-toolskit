package combine

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWriteJSONUsesSharedResponseContract(t *testing.T) {
	rec := httptest.NewRecorder()

	writeJSON(rec, http.StatusAccepted, map[string]any{"ok": true})

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q", got)
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "application/json") {
		t.Fatalf("Content-Type = %q", got)
	}
	if strings.TrimSpace(rec.Body.String()) != `{"ok":true}` {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestWriteJSONEncodeFailureReturnsStableError(t *testing.T) {
	rec := httptest.NewRecorder()

	writeJSON(rec, http.StatusAccepted, map[string]any{"bad": make(chan int)})

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rec.Code)
	}
	if strings.TrimSpace(rec.Body.String()) != `{"error":"failed to encode response"}` {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestWriteJSONErrorUsesSharedEnvelope(t *testing.T) {
	rec := httptest.NewRecorder()

	writeJSONError(rec, http.StatusForbidden, "无权操作")

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q", got)
	}
	if strings.TrimSpace(rec.Body.String()) != `{"error":"无权操作"}` {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestWriteBadJSONRequestUsesConsistentMessage(t *testing.T) {
	rec := httptest.NewRecorder()

	writeBadJSONRequest(rec)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rec.Code)
	}
	if strings.TrimSpace(rec.Body.String()) != `{"error":"请求格式错误"}` {
		t.Fatalf("body = %q", rec.Body.String())
	}
}
