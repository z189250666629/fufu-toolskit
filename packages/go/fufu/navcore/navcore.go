package navcore

import (
	"fmt"
	"sort"
	"strings"
)

type NavigationConfig struct {
	Cards []NavigationCardConfig `json:"cards"`
}

type NavigationCardConfig struct {
	ID          string                 `json:"id"`
	Stamp       string                 `json:"stamp"`
	Title       string                 `json:"title"`
	Description string                 `json:"description,omitempty"`
	Accent      string                 `json:"accent"`
	LineKind    string                 `json:"lineKind,omitempty"`
	Href        string                 `json:"href,omitempty"`
	Links       []NavigationLinkConfig `json:"links,omitempty"`
}

type NavigationLinkConfig struct {
	Label string `json:"label"`
	Href  string `json:"href"`
	Ping  string `json:"ping,omitempty"`
}

type NavigationLineCategory struct {
	Kind  string           `json:"kind"`
	Name  string           `json:"name"`
	Lines []NavigationLine `json:"lines"`
}

type NavigationLine struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

func DefaultNavigationLines(urls []string) []NavigationLine {
	names := []string{"线路 一", "线路 二", "线路 三"}
	lines := make([]NavigationLine, 0, len(urls))
	for i, u := range urls {
		name := fmt.Sprintf("线路 %d", i+1)
		if i < len(names) {
			name = names[i]
		}
		lines = append(lines, NavigationLine{Name: name, URL: u})
	}
	return lines
}

func NormalizeNavigationConfig(cfg, fallback NavigationConfig) NavigationConfig {
	cards := make([]NavigationCardConfig, 0, len(cfg.Cards))
	for _, card := range cfg.Cards {
		card.ID = strings.TrimSpace(card.ID)
		card.Stamp = strings.TrimSpace(card.Stamp)
		card.Title = strings.TrimSpace(card.Title)
		card.Description = strings.TrimSpace(card.Description)
		card.Accent = NormalizeNavigationAccent(card.Accent)
		card.LineKind = NormalizeNavigationLineKind(card.LineKind)
		card.Href = strings.TrimSpace(card.Href)
		card.Links = NormalizeNavigationLinks(card.Links)
		if card.Title == "" {
			continue
		}
		if card.ID == "" {
			card.ID = NavigationIDFromTitle(card.Title)
		}
		if card.ID == "" {
			card.ID = fmt.Sprintf("tool-%d", len(cards)+1)
		}
		if card.Stamp == "" {
			card.Stamp = "工具"
		}
		if card.LineKind == "" && card.Href == "" && len(card.Links) == 0 {
			continue
		}
		cards = append(cards, card)
	}
	if len(cards) == 0 {
		return CloneNavigationConfig(fallback)
	}
	return NavigationConfig{Cards: cards}
}

func NormalizeNavigationAccent(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "clay", "moss", "stone":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "moss"
	}
}

func NormalizeNavigationLineKind(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "api", "token":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return ""
	}
}

func NormalizeNavigationLinks(links []NavigationLinkConfig) []NavigationLinkConfig {
	out := make([]NavigationLinkConfig, 0, len(links))
	for i, link := range links {
		label := strings.TrimSpace(link.Label)
		href := strings.TrimSpace(link.Href)
		ping := strings.TrimSpace(link.Ping)
		if href == "" {
			continue
		}
		if label == "" {
			label = fmt.Sprintf("线路 %d", len(out)+1)
			if i == 0 {
				label = "线路 一"
			}
		}
		if ping == "" {
			ping = href
		}
		out = append(out, NavigationLinkConfig{Label: label, Href: href, Ping: ping})
	}
	return out
}

func CloneNavigationConfig(cfg NavigationConfig) NavigationConfig {
	out := NavigationConfig{Cards: make([]NavigationCardConfig, len(cfg.Cards))}
	for i, card := range cfg.Cards {
		card.Links = append([]NavigationLinkConfig(nil), card.Links...)
		out.Cards[i] = card
	}
	return out
}

func SortNavigationCardsForDisplay(cards []NavigationCardConfig) []NavigationCardConfig {
	out := make([]NavigationCardConfig, len(cards))
	copy(out, cards)
	sort.SliceStable(out, func(i, j int) bool {
		return isNavigationLinkListCard(out[i]) && !isNavigationLinkListCard(out[j])
	})
	return out
}

func isNavigationLinkListCard(card NavigationCardConfig) bool {
	return len(card.Links) > 0
}

func NavigationIDFromTitle(title string) string {
	title = strings.ToLower(strings.TrimSpace(title))
	var b strings.Builder
	lastDash := false
	for _, r := range title {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash && b.Len() > 0 {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func NavigationCardResponses(cards []NavigationCardConfig) []map[string]any {
	out := make([]map[string]any, 0, len(cards))
	for _, card := range cards {
		links := make([]map[string]any, 0, len(card.Links))
		for _, link := range card.Links {
			links = append(links, map[string]any{"label": link.Label, "href": link.Href, "ping": link.Ping})
		}
		out = append(out, map[string]any{
			"id":          card.ID,
			"stamp":       card.Stamp,
			"title":       card.Title,
			"description": card.Description,
			"accent":      card.Accent,
			"lineKind":    card.LineKind,
			"href":        card.Href,
			"links":       links,
		})
	}
	return out
}
