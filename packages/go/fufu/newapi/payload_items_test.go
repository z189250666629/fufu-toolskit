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

func TestPayloadItemsStopsAtFirstArrayCandidate(t *testing.T) {
	data := map[string]any{
		"items": []any{},
		"data": map[string]any{
			"items": []any{
				map[string]any{"id": "nested"},
			},
		},
	}

	got := PayloadItems(data, "items")
	if got == nil || len(got) != 0 {
		t.Fatalf("PayloadItems should keep first empty array candidate: %#v", got)
	}
}

func TestPayloadItemsTopLevelIgnoresNestedCandidates(t *testing.T) {
	data := map[string]any{
		"data": map[string]any{
			"data": []any{
				map[string]any{"id": "nested"},
			},
		},
	}

	if got := PayloadItemsTopLevel(data, "data"); got != nil {
		t.Fatalf("PayloadItemsTopLevel should ignore nested arrays: %#v", got)
	}

	data["data"] = []any{
		map[string]any{"id": "top"},
		"skip",
	}
	got := PayloadItemsTopLevel(data, "data")
	if len(got) != 1 || got[0]["id"] != "top" {
		t.Fatalf("PayloadItemsTopLevel = %#v", got)
	}
}
