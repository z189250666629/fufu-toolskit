package navcore

import "testing"

func TestNormalizeNavigationConfigBuildsStableCards(t *testing.T) {
	got := NormalizeNavigationConfig(NavigationConfig{Cards: []NavigationCardConfig{
		{Title: "  My Tool  ", Stamp: " ", Accent: "bad", Links: []NavigationLinkConfig{{Href: " https://example.test/tool "}}},
		{ID: "api", Title: "API Lines", Stamp: "次数", Accent: "clay", LineKind: " API "},
		{Title: "no-target"},
		{Title: ""},
	}}, NavigationConfig{})

	if len(got.Cards) != 2 {
		t.Fatalf("cards = %#v", got.Cards)
	}
	card := got.Cards[0]
	if card.ID != "my-tool" || card.Title != "My Tool" || card.Stamp != "工具" || card.Accent != "moss" {
		t.Fatalf("normalized card = %#v", card)
	}
	if len(card.Links) != 1 || card.Links[0].Label != "线路 一" || card.Links[0].Href != "https://example.test/tool" || card.Links[0].Ping != "https://example.test/tool" {
		t.Fatalf("normalized links = %#v", card.Links)
	}
	lineCard := got.Cards[1]
	if lineCard.ID != "api" || lineCard.LineKind != "api" || len(lineCard.Links) != 0 {
		t.Fatalf("line sourced card should be kept without static links, got %#v", lineCard)
	}
}

func TestNormalizeNavigationConfigFallsBackToCallerProvidedConfig(t *testing.T) {
	fallback := NavigationConfig{Cards: []NavigationCardConfig{{ID: "fallback", Title: "Fallback", Href: "/fallback"}}}
	got := NormalizeNavigationConfig(NavigationConfig{}, fallback)
	if len(got.Cards) != 1 || got.Cards[0].ID != "fallback" {
		t.Fatalf("expected caller-provided fallback navigation, got %#v", got)
	}
}

func TestSortNavigationCardsForDisplayPutsLinkListCardsFirst(t *testing.T) {
	cards := []NavigationCardConfig{
		{ID: "status", Title: "Status", Href: "/status"},
		{ID: "api", Title: "API", Links: []NavigationLinkConfig{{Label: "A", Href: "https://api-a.example.test"}, {Label: "B", Href: "https://api-b.example.test"}}},
		{ID: "build", Title: "Build", Href: "https://build.example.test"},
		{ID: "token", Title: "Token", Links: []NavigationLinkConfig{{Label: "T", Href: "https://token.example.test"}}},
	}

	got := SortNavigationCardsForDisplay(cards)
	gotIDs := []string{}
	for _, card := range got {
		gotIDs = append(gotIDs, card.ID)
	}
	wantIDs := []string{"api", "token", "status", "build"}
	for i, want := range wantIDs {
		if gotIDs[i] != want {
			t.Fatalf("display order = %#v, want %#v", gotIDs, wantIDs)
		}
	}
	if cards[0].ID != "status" {
		t.Fatalf("sort should not mutate input order, got %#v", cards)
	}
}

func TestDefaultNavigationLinesNamesCallerProvidedURLs(t *testing.T) {
	got := DefaultNavigationLines([]string{"https://a.example.test", "https://b.example.test", "https://c.example.test", "https://d.example.test"})
	if len(got) != 4 || got[0].Name != "线路 一" || got[3].Name != "线路 4" {
		t.Fatalf("lines = %#v", got)
	}
}

func TestNavigationIDFromTitleUsesAsciiSlug(t *testing.T) {
	if got := NavigationIDFromTitle("API / 模型状态"); got != "api" {
		t.Fatalf("NavigationIDFromTitle = %q", got)
	}
	if got := NavigationIDFromTitle("构建"); got != "" {
		t.Fatalf("non-ascii title should produce empty slug, got %q", got)
	}
}
