package newapi

import "testing"

func TestPayloadItemsReadsTopLevelAndNestedCandidates(t *testing.T) {
	data := map[string]any{
		"data": map[string]any{
			"tokens": []any{
				map[string]any{"id": float64(1)},
				"skip",
				map[string]any{"id": float64(2)},
			},
		},
	}

	got := PayloadItems(data, "data", "items", "tokens")
	if len(got) != 2 || got[0]["id"] != float64(1) || got[1]["id"] != float64(2) {
		t.Fatalf("PayloadItems = %#v", got)
	}
}

func TestPayloadItemsPrefersTopLevelCandidates(t *testing.T) {
	data := map[string]any{
		"items": []any{
			map[string]any{"id": "top"},
		},
		"data": map[string]any{
			"items": []any{
				map[string]any{"id": "nested"},
			},
		},
	}

	got := PayloadItems(data, "data", "items")
	if len(got) != 1 || got[0]["id"] != "top" {
		t.Fatalf("PayloadItems should prefer top-level items: %#v", got)
	}
}
