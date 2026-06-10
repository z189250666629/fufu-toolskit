package combine

import (
	"fufu/newapi"
	"fufu/tokens"
	"sort"
)

func resolvedFromToken(t tokens.Token) ResolvedToken {
	return ResolvedToken{ID: t.ID, Key: t.Key, Name: t.Name, RemainQuota: t.RemainQuota, UsedQuota: t.UsedQuota, IntervalUnit: t.IntervalUnit, Group: t.Group, Status: t.Status, Raw: t.Raw}
}

func tokenFromRaw(raw map[string]any) ResolvedToken {
	if raw == nil {
		raw = map[string]any{}
	}
	return ResolvedToken{ID: toInt(raw["id"]), Key: ensureFullKey(getString(raw, "key")), Name: getString(raw, "name"), RemainQuota: toInt64(raw["remain_quota"]), UsedQuota: toInt64(raw["used_quota"]), IntervalUnit: toInt(raw["interval_unit"]), Group: stringOrDefault(getString(raw, "group"), "mix"), Status: intOrDefault(toIntDefault(raw["status"], 1), 1), Raw: raw}
}

func cloneMap(raw map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range raw {
		out[k] = v
	}
	return out
}

func dataList(data map[string]any) []map[string]any {
	return newapi.PayloadItemsTopLevel(data, "data")
}

func findTokenByName(data map[string]any, name string) map[string]any {
	for _, item := range dataList(data) {
		if getString(item, "name") == name {
			return item
		}
	}
	return nil
}

func findResolvedByID(tokens []ResolvedToken, id int) *ResolvedToken {
	for i := range tokens {
		if tokens[i].ID == id {
			return &tokens[i]
		}
	}
	return nil
}

func uniqueIDs(tokens []ResolvedToken) []int {
	seen := map[int]bool{}
	ids := []int{}
	for _, t := range tokens {
		if !seen[t.ID] {
			seen[t.ID] = true
			ids = append(ids, t.ID)
		}
	}
	return ids
}

func majorityGroup(tokens []ResolvedToken) string {
	counts := map[string]int{}
	for _, t := range tokens {
		g := t.Group
		if g == "" {
			g = "mix"
		}
		counts[g]++
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
	return "mix"
}
