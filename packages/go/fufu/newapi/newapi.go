package newapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const DefaultQuotaUnit int64 = 500000

type Site struct {
	Name                string  `json:"name"`
	URL                 string  `json:"url"`
	Token               string  `json:"-"`
	UserID              string  `json:"userId"`
	Kind                string  `json:"kind,omitempty"`
	SkipUserHeader      bool    `json:"skipUserHeader,omitempty"`
	QuotaUnit           int64   `json:"quotaUnit"`
	Currency            string  `json:"currency"`
	RechargeRatio       float64 `json:"rechargeRatio"`
	ChannelListEndpoint string  `json:"channelListEndpoint,omitempty"`
	Note                string  `json:"note,omitempty"`
}

type PublicSite struct {
	Name                string  `json:"name"`
	URL                 string  `json:"url"`
	UserID              string  `json:"userId"`
	Kind                string  `json:"kind,omitempty"`
	SkipUserHeader      bool    `json:"skipUserHeader,omitempty"`
	QuotaUnit           int64   `json:"quotaUnit"`
	Currency            string  `json:"currency"`
	RechargeRatio       float64 `json:"rechargeRatio"`
	ChannelListEndpoint string  `json:"channelListEndpoint,omitempty"`
	Note                string  `json:"note,omitempty"`
}

func (s Site) Public() PublicSite {
	return PublicSite{Name: s.Name, URL: s.URL, UserID: s.UserID, Kind: s.Kind, SkipUserHeader: s.SkipUserHeader, QuotaUnit: s.QuotaUnit, Currency: s.Currency, RechargeRatio: s.RechargeRatio, ChannelListEndpoint: s.ChannelListEndpoint, Note: s.Note}
}

type Client struct {
	Site       Site
	HTTPClient *http.Client
}

func NewClient(site Site) *Client {
	return &Client{Site: normalizeSite(site), HTTPClient: &http.Client{Timeout: 60 * time.Second}}
}

type Response struct {
	StatusCode int
	Body       string
}

func (r Response) OK() bool { return r.StatusCode >= 200 && r.StatusCode < 300 }
func (r Response) BodyOr(fallback string) string {
	if strings.TrimSpace(r.Body) == "" {
		return fallback
	}
	return r.Body
}

func (c *Client) Headers() http.Header {
	headers := http.Header{}
	headers.Set("Accept", "application/json")
	if c.Site.Token != "" {
		headers.Set("Authorization", "Bearer "+c.Site.Token)
	}
	if !c.Site.SkipUserHeader {
		headers.Set("New-Api-User", strings.TrimSpace(c.Site.UserID))
	}
	return headers
}

func (c *Client) Request(ctx context.Context, method, endpoint string, body any) (Response, map[string]any, error) {
	if c == nil {
		return Response{}, nil, fmt.Errorf("nil NewAPI client")
	}
	endpoint = requestEndpoint(endpoint)
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return Response{}, nil, err
		}
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, strings.ToUpper(method), strings.TrimRight(c.Site.URL, "/")+endpoint, reader)
	if err != nil {
		return Response{}, nil, err
	}
	for key, values := range c.Headers() {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	if shouldSetJSONContentType(method, body) {
		req.Header.Set("Content-Type", "application/json")
	}
	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return Response{}, nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return Response{StatusCode: resp.StatusCode}, nil, err
	}
	apiResp := Response{StatusCode: resp.StatusCode, Body: string(respBody)}
	return apiResp, decodeResponsePayload(respBody), nil
}

func (c *Client) Get(ctx context.Context, endpoint string) (Response, map[string]any, error) {
	return c.Request(ctx, http.MethodGet, endpoint, nil)
}
func (c *Client) Post(ctx context.Context, endpoint string, body any) (Response, map[string]any, error) {
	return c.Request(ctx, http.MethodPost, endpoint, body)
}
func (c *Client) Put(ctx context.Context, endpoint string, body any) (Response, map[string]any, error) {
	return c.Request(ctx, http.MethodPut, endpoint, body)
}
func (c *Client) Delete(ctx context.Context, endpoint string) (Response, map[string]any, error) {
	return c.Request(ctx, http.MethodDelete, endpoint, nil)
}

func IsSuccess(data map[string]any) bool {
	if data == nil {
		return true
	}
	if ok, exists := data["success"].(bool); exists {
		return ok
	}
	return true
}

func ErrorMessage(data map[string]any, status int, fallback string) string {
	if value, ok := messageFromPayload(data); ok {
		return value
	}
	if status > 0 {
		return fmt.Sprintf("NewAPI HTTP %d", status)
	}
	if strings.TrimSpace(fallback) != "" {
		return fallback
	}
	return "NewAPI 请求失败"
}

func UpstreamStatusMessage(r Response, fallback string) string {
	if r.StatusCode > 0 {
		return fmt.Sprintf("%s（上游状态 %d）", fallback, r.StatusCode)
	}
	return fallback
}
