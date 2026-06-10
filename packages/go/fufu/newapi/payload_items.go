package newapi

func PayloadItems(data map[string]any, keys ...string) []map[string]any {
	if len(keys) == 0 {
		keys = []string{"data", "items"}
	}

	candidates := make([]any, 0, len(keys)*2)
	for _, key := range keys {
		candidates = append(candidates, data[key])
	}
	if nested, ok := data["data"].(map[string]any); ok {
		for _, key := range keys {
			candidates = append(candidates, nested[key])
		}
	}

	for _, candidate := range candidates {
		items, ok := candidate.([]any)
		if !ok {
			continue
		}
		out := []map[string]any{}
		for _, item := range items {
			if obj, ok := item.(map[string]any); ok {
				out = append(out, obj)
			}
		}
		return out
	}
	return nil
}
