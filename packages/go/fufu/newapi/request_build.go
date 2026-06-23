package newapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

type closeConnectionContextKey struct{}

// WithConnectionClose marks NewAPI requests built with this context as
// single-use HTTP requests. This is useful for bounded background jobs where a
// timed-out upstream call must not leave a keep-alive connection behind.
func WithConnectionClose(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, closeConnectionContextKey{}, true)
}

func buildHTTPRequest(ctx context.Context, client *Client, method, endpoint string, body any) (*http.Request, error) {
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, strings.ToUpper(method), strings.TrimRight(client.Site.URL, "/")+requestEndpoint(endpoint), reader)
	if err != nil {
		return nil, err
	}
	for key, values := range client.Headers() {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	if shouldSetJSONContentType(method, body) {
		req.Header.Set("Content-Type", "application/json")
	}
	if shouldCloseConnection(ctx) {
		req.Header.Set("Connection", "close")
		req.Close = true
	}
	return req, nil
}

func shouldCloseConnection(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	closeConnection, _ := ctx.Value(closeConnectionContextKey{}).(bool)
	return closeConnection
}
