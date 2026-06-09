package combine

import "strings"

func ensureFullKey(key string) string {
	s := strings.TrimSpace(key)
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, "sk-") {
		return s
	}
	return "sk-" + s
}

func displayKey(key string) string {
	full := ensureFullKey(key)
	bare := strings.TrimPrefix(full, "sk-")
	r := []rune(bare)
	if len(r) <= 8 {
		return full
	}
	return "sk-" + string(r[:4]) + "…" + string(r[len(r)-4:])
}

func normalizeKeys(raw []string) []string {
	seen := map[string]bool{}
	keys := []string{}
	for _, item := range raw {
		key := ensureFullKey(strings.TrimSpace(item))
		if key == "" || key == "sk-" || seen[key] {
			continue
		}
		seen[key] = true
		keys = append(keys, key)
	}
	return keys
}

func keyMask(key string) string { return displayKey(key) }
