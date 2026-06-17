package main

import "strings"

type navLineResponse struct {
	Categories []navLineCategory `json:"categories"`
}

type navLineCategory struct {
	Kind  string    `json:"kind"`
	Name  string    `json:"name"`
	Lines []navLine `json:"lines"`
}

type navLine struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type navToolsResponse struct {
	Cards []NavigationCardConfig `json:"cards"`
}

func navigationToolsForRuntime() []NavigationCardConfig {
	cfg := defaultNavigationConfig()
	if unifiedConfig == nil {
		return navigationCardsWithRuntimeLines(cfg.Cards, navLineCategories())
	}
	cfg = unifiedConfig.Snapshot().Navigation
	return navigationCardsWithRuntimeLines(cfg.Cards, navLineCategories())
}

func navigationCardsWithRuntimeLines(cards []NavigationCardConfig, categories []navLineCategory) []NavigationCardConfig {
	out := make([]NavigationCardConfig, 0, len(cards))
	for _, card := range cards {
		if strings.TrimSpace(card.LineKind) != "" {
			card.LineKind = strings.ToLower(strings.TrimSpace(card.LineKind))
			card.Href = ""
			card.Links = navigationLinksForLineKind(categories, card.LineKind)
		}
		if card.Href == "" && len(card.Links) == 0 {
			continue
		}
		out = append(out, card)
	}
	return sortNavigationCardsForDisplay(out)
}

func navigationLinksForLineKind(categories []navLineCategory, kind string) []NavigationLinkConfig {
	for _, category := range categories {
		if !strings.EqualFold(category.Kind, kind) {
			continue
		}
		links := make([]NavigationLinkConfig, 0, len(category.Lines))
		for _, line := range category.Lines {
			links = append(links, NavigationLinkConfig{Label: line.Name, Href: line.URL, Ping: line.URL})
		}
		return links
	}
	return nil
}

func navLineCategories() []navLineCategory {
	sites, _ := managedSitesForRuntime()
	defaults := defaultNavLineCategoriesByKind()
	apiLines := []navLine{}
	tokenLines := []navLine{}
	for _, s := range sites {
		lineName := s.LineName
		if strings.TrimSpace(lineName) == "" {
			lineName = s.Name
		}
		line := navLine{Name: lineName, URL: s.URL}
		if strings.EqualFold(s.Category, "token") {
			tokenLines = append(tokenLines, line)
		} else {
			apiLines = append(apiLines, line)
		}
	}
	if len(apiLines) == 0 {
		apiLines = defaults["api"].Lines
	}
	if len(tokenLines) == 0 {
		tokenLines = defaults["token"].Lines
	}
	return []navLineCategory{
		{Kind: "api", Name: "API 次数站", Lines: apiLines},
		{Kind: "token", Name: "Token 站", Lines: tokenLines},
	}
}

func defaultNavLineCategoriesByKind() map[string]navLineCategory {
	out := map[string]navLineCategory{}
	for _, category := range defaultNavigationLineCategories() {
		lines := make([]navLine, 0, len(category.Lines))
		for _, line := range category.Lines {
			lines = append(lines, navLine{Name: line.Name, URL: line.URL})
		}
		out[category.Kind] = navLineCategory{Kind: category.Kind, Name: category.Name, Lines: lines}
	}
	return out
}
