package tokens

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"fufu/newapi"
)

type cancelAfterSearchErrorTransport struct {
	slowStarted  chan struct{}
	slowCanceled chan struct{}
}

func (t *cancelAfterSearchErrorTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	token := req.URL.Query().Get("token")
	switch token {
	case "slow-search-key":
		close(t.slowStarted)
		<-req.Context().Done()
		close(t.slowCanceled)
		return nil, req.Context().Err()
	case "fail-search-key":
		select {
		case <-t.slowStarted:
		case <-time.After(time.Second):
			return nil, errors.New("slow search did not start")
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"success":false,"message":"boom"}`)),
			Request:    req,
		}, nil
	default:
		return nil, errors.New("unexpected token lookup: " + token)
	}
}

func TestBatchSearchCancelsInFlightLookupsAfterFirstError(t *testing.T) {
	transport := &cancelAfterSearchErrorTransport{
		slowStarted:  make(chan struct{}),
		slowCanceled: make(chan struct{}),
	}
	client := newapi.NewClient(newapi.Site{URL: "https://newapi.example.test", Token: "x", UserID: "1"})
	client.HTTPClient = &http.Client{Transport: transport}
	svc := NewService(client)
	svc.Concurrency = 2

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, _, _, err := svc.BatchSearch(ctx, []string{"slow-search-key", "fail-search-key"})
	elapsed := time.Since(started)

	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("BatchSearch err = %v, want upstream failure", err)
	}
	select {
	case <-transport.slowCanceled:
	default:
		t.Fatal("slow in-flight lookup was not canceled after first error")
	}
	if elapsed > 300*time.Millisecond {
		t.Fatalf("BatchSearch waited for parent timeout instead of canceling peers after error: %s", elapsed)
	}
}
