package combine

import (
	"strings"

	tokenkeys "fufu/tokens"
)

func ensureFullKey(key string) string {
	return tokenkeys.EnsureFullKey(key)
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
	keys := tokenkeys.NormalizeKeys(raw)
	if len(keys) > 0 {
		return keys
	}

	seen := map[string]bool{}
	for _, item := range raw {
		for _, part := range strings.FieldsFunc(item, func(r rune) bool { return r == '\n' || r == '\r' || r == '\t' || r == ' ' || r == ',' || r == ';' }) {
			key := ensureFullKey(part)
			bare := strings.TrimPrefix(key, "sk-")
			if key == "" || key == "sk-" || seen[bare] {
				continue
			}
			seen[bare] = true
			keys = append(keys, key)
		}
	}
	return keys
}

func keyMask(key string) string { return displayKey(key) }
