package newapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

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
	return req, nil
}
