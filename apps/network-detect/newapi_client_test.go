package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"fufu/newapi"
)

func TestNewAPIGetReportsDecodeErrorWithUpstreamStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("page failed"))
	}))
	t.Cleanup(server.Close)

	result := newAPIGet(context.Background(), newapi.Site{URL: server.URL, Token: "token", UserID: "1"}, "/api/log/self", time.Second)

	if result.OK {
		t.Fatalf("result should fail: %#v", result)
	}
	if result.Status != http.StatusBadGateway {
		t.Fatalf("status = %d", result.Status)
	}
	if !strings.Contains(result.Error, "502") {
		t.Fatalf("error should include upstream status: %q", result.Error)
	}
}

func TestNewAPIGetMasksTransportErrors(t *testing.T) {
	result := newAPIGet(context.Background(), newapi.Site{URL: "http://internal.example.local/%zz", Token: "token", UserID: "1"}, "/api/log/self", time.Second)

	if result.OK {
		t.Fatalf("result should fail: %#v", result)
	}
	if result.Status != 0 {
		t.Fatalf("status = %d", result.Status)
	}
	if result.Error != "NewAPI 请求失败" {
		t.Fatalf("transport error should use safe public message, got %q", result.Error)
	}
	for _, leaked := range []string{"internal.example.local", "%zz", "parse", "invalid URL"} {
		if strings.Contains(result.Error, leaked) {
			t.Fatalf("transport error leaked %q in %q", leaked, result.Error)
		}
	}
}

func TestToInt64ParsesDecimalJSONNumber(t *testing.T) {
	if got := toInt64(json.Number("42.0")); got != 42 {
		t.Fatalf("decimal json.Number = %d", got)
	}
}

func TestToInt64ParsesDecimalString(t *testing.T) {
	if got := toInt64("42.0"); got != 42 {
		t.Fatalf("decimal string = %d", got)
	}
}
