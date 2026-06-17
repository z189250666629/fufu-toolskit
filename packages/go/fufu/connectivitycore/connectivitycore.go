package connectivitycore

import (
	"encoding/json"
	"fmt"
	"fufu/config"
	"net"
	"net/url"
	"strings"
)

type GroupInput struct {
	ID   string
	Name string
	URLs []string
}

func ParseGroupsJSON(raw string) ([]map[string]any, error) {
	var groups []map[string]any
	if err := json.Unmarshal([]byte(raw), &groups); err != nil {
		return nil, err
	}
	return SanitizeGroups(groups), nil
}

func BuildGroups(inputs []GroupInput) []map[string]any {
	raw := make([]map[string]any, 0, len(inputs))
	for _, input := range inputs {
		raw = append(raw, map[string]any{"id": input.ID, "name": input.Name, "urls": input.URLs})
	}
	return SanitizeGroups(raw)
}

func TargetURLs(explicitValue, legacyValue, fallbackValue string) []string {
	if urls := SplitPublicTargetList(firstNonEmpty(explicitValue, legacyValue)); len(urls) > 0 {
		return urls
	}
	return PublicBrowserTargets(SplitPublicTargetList(fallbackValue))
}

func SplitPublicTargetList(value string) []string {
	parts := strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == '\n' || r == '\r' || r == '\t' })
	out := []string{}
	for _, part := range parts {
		if u, ok := PublicBrowserTargetOrigin(part); ok {
			out = append(out, u)
		}
	}
	return out
}

func SanitizeGroups(groups []map[string]any) []map[string]any {
	out := []map[string]any{}
	for _, group := range groups {
		urls := []string{}
		for _, raw := range rawURLs(group["urls"]) {
			if u, ok := PublicBrowserTargetOrigin(raw); ok {
				urls = append(urls, u)
			}
		}
		if len(urls) == 0 {
			continue
		}
		id := strings.TrimSpace(fmt.Sprint(group["id"]))
		name := strings.TrimSpace(fmt.Sprint(group["name"]))
		if id == "" || id == "<nil>" {
			id = name
		}
		if name == "" || name == "<nil>" {
			name = id
		}
		if id == "" || id == "<nil>" {
			id = "custom"
		}
		if name == "" || name == "<nil>" {
			name = "URL 组"
		}
		out = append(out, map[string]any{"id": id, "name": name, "urls": urls})
	}
	return out
}

func PublicBrowserTargets(urls []string) []string {
	out := []string{}
	for _, target := range urls {
		if u, ok := PublicBrowserTargetOrigin(target); ok {
			out = append(out, u)
		}
	}
	return out
}

func IsPublicBrowserTarget(target string) bool {
	_, ok := PublicBrowserTargetOrigin(target)
	return ok
}

func PublicBrowserTargetOrigin(target string) (string, bool) {
	target = config.NormalizeBaseURL(target)
	if target == "" {
		return "", false
	}
	u, err := url.Parse(target)
	if err != nil {
		return "", false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", false
	}
	host := strings.ToLower(strings.Trim(u.Hostname(), "."))
	if host == "" || host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return "", false
	}
	ip := net.ParseIP(host)
	if ip != nil && (ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified()) {
		return "", false
	}
	return (&url.URL{Scheme: u.Scheme, Host: u.Host}).String(), true
}

func rawURLs(value any) []string {
	switch v := value.(type) {
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			out = append(out, fmt.Sprint(item))
		}
		return out
	case []string:
		return v
	default:
		return nil
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
