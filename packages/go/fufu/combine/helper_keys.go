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
	return tokenkeys.NormalizeKeys(raw)
}

func keyMask(key string) string { return displayKey(key) }
