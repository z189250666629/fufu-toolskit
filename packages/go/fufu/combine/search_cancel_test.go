package combine

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type combineCancelAfterSearchErrorTransport struct {
	slowStarted  chan struct{}
	slowCanceled chan struct{}
}

func (t *combineCancelAfterSearchErrorTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	token := req.URL.Query().Get("token")
	switch token {
	case "slow-combine-key":
		close(t.slowStarted)
		<-req.Context().Done()
		close(t.slowCanceled)
		return nil, req.Context().Err()
	case "fail-combine-key":
		select {
		case <-t.slowStarted:
		case <-time.After(time.Second):
			return nil, errors.New("slow combine search did not start")
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"success":false,"message":"boom"}`)),
			Request:    req,
		}, nil
	default:
		return nil, errors.New("unexpected combine token lookup: " + token)
	}
}

func TestSearchTokensConcurrentCancelsInFlightLookupsAfterFirstError(t *testing.T) {
	transport := &combineCancelAfterSearchErrorTransport{
		slowStarted:  make(chan struct{}),
		slowCanceled: make(chan struct{}),
	}
	app := NewApp(Config{URL: "https://newapi.example.test", Token: "x", UserID: "1"}, nil)
	app.apiClient.HTTPClient = &http.Client{Transport: transport}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err := app.searchTokensConcurrent(ctx, []string{"sk-slow-combine-key", "sk-fail-combine-key"})
	elapsed := time.Since(started)

	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("searchTokensConcurrent err = %v, want upstream failure", err)
	}
	select {
	case <-transport.slowCanceled:
	default:
		t.Fatal("slow in-flight combine lookup was not canceled after first error")
	}
	if elapsed > 300*time.Millisecond {
		t.Fatalf("searchTokensConcurrent waited for parent timeout instead of canceling peers after error: %s", elapsed)
	}
}
