package newapi

import "net/http"
import "testing"

func TestNormalizeSiteDefaultsAndURL(t *testing.T) {
	site := normalizeSite(Site{URL: " https://api.example.test/// "})

	if site.URL != "https://api.example.test" {
		t.Fatalf("URL = %q", site.URL)
	}
	if site.UserID != "1" || site.QuotaUnit != DefaultQuotaUnit || site.Currency != "$" || site.RechargeRatio != 1 {
		t.Fatalf("defaults = %#v", site)
	}
}

func TestRequestEndpointAndJSONContentTypeHelpers(t *testing.T) {
	if got := requestEndpoint("api/token"); got != "/api/token" {
		t.Fatalf("requestEndpoint no slash = %q", got)
	}
	if got := requestEndpoint("/api/token"); got != "/api/token" {
		t.Fatalf("requestEndpoint slash = %q", got)
	}
	if !shouldSetJSONContentType(http.MethodPost, map[string]any{"name": "x"}) {
		t.Fatalf("POST body should set json content type")
	}
	if shouldSetJSONContentType(http.MethodGet, map[string]any{"name": "x"}) {
		t.Fatalf("GET should not set json content type")
	}
	if shouldSetJSONContentType(http.MethodPost, nil) {
		t.Fatalf("nil body should not set json content type")
	}
}

func TestNormalizeSiteTrimsStringFieldsBeforeDefaults(t *testing.T) {
	client := NewClient(Site{URL: " https://api.example.test/// ", Token: " secret ", UserID: "  ", Currency: "  "})

	if client.Site.URL != "https://api.example.test" || client.Site.Token != "secret" || client.Site.UserID != "1" || client.Site.Currency != "$" {
		t.Fatalf("normalized site = %#v", client.Site)
	}
	if got := client.Headers().Get("Authorization"); got != "Bearer secret" {
		t.Fatalf("Authorization = %q", got)
	}
	if got := client.Headers().Get("New-Api-User"); got != "1" {
		t.Fatalf("New-Api-User = %q", got)
	}
}
