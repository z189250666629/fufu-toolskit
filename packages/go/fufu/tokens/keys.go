package tokens

import (
	"sort"
	"strings"
)

func EnsureFullKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	if strings.HasPrefix(key, "sk-") {
		return key
	}
	return "sk-" + key
}

func BareKey(key string) string { return strings.TrimPrefix(EnsureFullKey(key), "sk-") }

func DisplayKey(key string) string {
	value := EnsureFullKey(key)
	if len(value) <= 14 {
		return value
	}
	return value[:7] + "..." + value[len(value)-5:]
}

func NormalizeKeys(raw []string) []string {
	seen := map[string]bool{}
	keys := []string{}
	for _, item := range raw {
		for _, part := range strings.FieldsFunc(item, isKeyPartSeparator) {
			key := EnsureFullKey(part)
			if len(key) <= 10 {
				continue
			}
			bare := BareKey(key)
			if !seen[bare] {
				seen[bare] = true
				keys = append(keys, key)
			}
		}
	}
	return keys
}

func isKeyPartSeparator(r rune) bool {
	switch r {
	case '\n', '\r', '\t', ' ', ',', ';', '，', '；', '"', '\'', '[', ']', '{', '}', '(', ')', '\\':
		return true
	default:
		return false
	}
}

func MajorityGroup(tokens []Token) string {
	counts := map[string]int{}
	for _, t := range tokens {
		if t.Group != "" {
			counts[t.Group]++
		}
	}
	groups := make([]string, 0, len(counts))
	for g := range counts {
		groups = append(groups, g)
	}
	sort.Slice(groups, func(i, j int) bool {
		if counts[groups[i]] == counts[groups[j]] {
			return groups[i] < groups[j]
		}
		return counts[groups[i]] > counts[groups[j]]
	})
	if len(groups) > 0 {
		return groups[0]
	}
	return "default"
}
