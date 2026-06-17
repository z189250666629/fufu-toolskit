package connectivitycore

import (
	"reflect"
	"testing"
)

func TestPublicBrowserTargetOrigin(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
		ok   bool
	}{
		{name: "public https origin strips path", in: "https://api.example.test/v1/models?x=1", want: "https://api.example.test", ok: true},
		{name: "public port is preserved", in: "http://example.test:8080/health", want: "http://example.test:8080", ok: true},
		{name: "localhost is rejected", in: "http://localhost:8080", ok: false},
		{name: "private ip is rejected", in: "https://192.168.1.10", ok: false},
		{name: "non-http scheme is rejected", in: "ftp://example.test", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := PublicBrowserTargetOrigin(tt.in)
			if got != tt.want || ok != tt.ok {
				t.Fatalf("PublicBrowserTargetOrigin(%q) = %q,%v want %q,%v", tt.in, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestSanitizeGroups(t *testing.T) {
	got := SanitizeGroups([]map[string]any{
		{"id": " api ", "name": " API ", "urls": []string{" https://api.example.test/v1 ", "http://localhost:8080"}},
		{"name": "Token", "urls": []any{"https://token.example.test/a", "https://10.0.0.1"}},
		{"id": "empty", "name": "Empty", "urls": []string{"http://127.0.0.1"}},
		{"urls": []string{"https://fallback.example.test/path"}},
	})
	want := []map[string]any{
		{"id": "api", "name": "API", "urls": []string{"https://api.example.test"}},
		{"id": "Token", "name": "Token", "urls": []string{"https://token.example.test"}},
		{"id": "custom", "name": "URL 组", "urls": []string{"https://fallback.example.test"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SanitizeGroups() = %#v want %#v", got, want)
	}
}

func TestParseGroupsJSON(t *testing.T) {
	got, err := ParseGroupsJSON(`[{"id":"api","name":"API","urls":["https://api.example.test/v1","http://localhost"]}]`)
	if err != nil {
		t.Fatalf("ParseGroupsJSON returned error: %v", err)
	}
	want := []map[string]any{{"id": "api", "name": "API", "urls": []string{"https://api.example.test"}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseGroupsJSON() = %#v want %#v", got, want)
	}
	if _, err := ParseGroupsJSON(`not-json`); err == nil {
		t.Fatal("ParseGroupsJSON should reject invalid JSON")
	}
}

func TestTargetURLs(t *testing.T) {
	tests := []struct {
		name     string
		explicit string
		legacy   string
		fallback string
		want     []string
	}{
		{name: "explicit wins over legacy and fallback", explicit: "https://explicit.example.test/path", legacy: "https://legacy.example.test", fallback: "https://fallback.example.test", want: []string{"https://explicit.example.test"}},
		{name: "legacy wins over fallback", legacy: "https://legacy.example.test/a", fallback: "https://fallback.example.test", want: []string{"https://legacy.example.test"}},
		{name: "fallback is used when explicit values are empty", fallback: "https://fallback.example.test/v1, http://localhost:8080", want: []string{"https://fallback.example.test"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TargetURLs(tt.explicit, tt.legacy, tt.fallback); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("TargetURLs() = %#v want %#v", got, tt.want)
			}
		})
	}
}
