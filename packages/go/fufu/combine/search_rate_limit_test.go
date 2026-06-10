package combine

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAllowClientSearchRequestScopesAndResetsWindow(t *testing.T) {
	app := NewApp(Config{}, nil)
	start := time.Unix(1700000000, 0)
	clientA := newSearchRateLimitRequest("203.0.113.101:5000")
	clientB := newSearchRateLimitRequest("198.51.100.101:5000")

	for i := 0; i < maxSearchRequestsPerClientWindow; i++ {
		if until, ok := app.allowClientSearchRequest(clientA, start.Add(time.Duration(i)*time.Second)); !ok {
			t.Fatalf("client A attempt %d was unexpectedly limited until %s", i+1, until)
		}
	}
	if until, ok := app.allowClientSearchRequest(clientA, start.Add(searchRequestWindow-time.Second)); ok || !until.Equal(start.Add(searchRequestWindow)) {
		t.Fatalf("client A should be limited until window end, ok=%v until=%s", ok, until)
	}
	if _, ok := app.allowClientSearchRequest(clientB, start.Add(searchRequestWindow-time.Second)); !ok {
		t.Fatal("client B should not be limited by client A")
	}
	if _, ok := app.allowClientSearchRequest(clientA, start.Add(searchRequestWindow)); !ok {
		t.Fatal("client A should be allowed again when its window resets")
	}
}

func TestAllowClientSearchRequestPrunesExpiredRecords(t *testing.T) {
	app := NewApp(Config{}, nil)
	now := time.Unix(1700001000, 0)
	app.searchRequests["old-client"] = searchRequestRecord{Count: maxSearchRequestsPerClientWindow, WindowStart: now.Add(-searchRequestWindow)}
	app.searchRequests["fresh-client"] = searchRequestRecord{Count: 1, WindowStart: now.Add(-time.Second)}

	req := newSearchRateLimitRequest("203.0.113.102:5000")
	if _, ok := app.allowClientSearchRequest(req, now); !ok {
		t.Fatal("new client should be allowed")
	}
	if _, ok := app.searchRequests["old-client"]; ok {
		t.Fatalf("expired search request record should be pruned: %#v", app.searchRequests)
	}
	if _, ok := app.searchRequests["fresh-client"]; !ok {
		t.Fatalf("fresh search request record should remain: %#v", app.searchRequests)
	}
}

func newSearchRateLimitRequest(remoteAddr string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/search-keys", nil)
	req.RemoteAddr = remoteAddr
	return req
}
