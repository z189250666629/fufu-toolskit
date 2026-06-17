package admincore

import (
	"net/url"
	"strings"
)

type NamedURL struct {
	Name string `json:"name,omitempty"`
	URL  string `json:"url"`
}

func NormalizeHTTPSOrigin(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return strings.TrimRight(raw, "/")
	}
	return "https://" + u.Host
}

func NormalizeHTTPBaseURL(value string) string {
	value = strings.TrimRight(strings.TrimSpace(value), "/")
	if value == "" {
		return ""
	}
	if !strings.HasPrefix(value, "http://") && !strings.HasPrefix(value, "https://") {
		return ""
	}
	return value
}

func NormalizeNamedURLs(urls []NamedURL, legacyURL string, defaultName func(index int) string) []NamedURL {
	combined := append([]NamedURL(nil), urls...)
	if strings.TrimSpace(legacyURL) != "" {
		combined = append(combined, NamedURL{URL: legacyURL})
	}
	out := []NamedURL{}
	seen := map[string]bool{}
	for _, entry := range combined {
		normalizedURL := NormalizeHTTPBaseURL(entry.URL)
		if normalizedURL == "" || seen[normalizedURL] {
			continue
		}
		seen[normalizedURL] = true
		name := strings.TrimSpace(entry.Name)
		if name == "" && defaultName != nil {
			name = defaultName(len(out))
		}
		out = append(out, NamedURL{Name: name, URL: normalizedURL})
	}
	return out
}

func MergeNamedURLs(existing, extra []NamedURL) []NamedURL {
	seen := map[string]bool{}
	for _, entry := range existing {
		seen[entry.URL] = true
	}
	for _, entry := range extra {
		if seen[entry.URL] {
			continue
		}
		seen[entry.URL] = true
		existing = append(existing, entry)
	}
	return existing
}

func MaskSecret(secret string) string {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return ""
	}
	runes := []rune(secret)
	if len(runes) <= 8 {
		return "••••"
	}
	return string(runes[:4]) + "…" + string(runes[len(runes)-4:])
}
