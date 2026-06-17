package main

import "fufu/navcore"

type NavigationConfig = navcore.NavigationConfig
type NavigationCardConfig = navcore.NavigationCardConfig
type NavigationLinkConfig = navcore.NavigationLinkConfig
type NavigationLineCategory = navcore.NavigationLineCategory
type NavigationLine = navcore.NavigationLine

func defaultNavigationConfig() NavigationConfig {
	return NavigationConfig{Cards: []NavigationCardConfig{
		{ID: "api", Stamp: "次数", Title: "API 次数站", Accent: "clay", LineKind: "api"},
		{ID: "token", Stamp: "额度", Title: "Token 站", Accent: "moss", LineKind: "token"},
		{
			ID:          "terminal",
			Stamp:       "终端",
			Title:       "Web Terminal",
			Description: "服务器网页管理终端",
			Accent:      "moss",
			Links: []NavigationLinkConfig{
				{Label: "线路 一", Href: "https://terminal.fufuapi.top", Ping: "https://terminal.fufuapi.top"},
				{Label: "线路 二", Href: "https://terminal.if.tc", Ping: "https://terminal.if.tc"},
			},
		},
		{ID: "build", Stamp: "造物", Title: "Build", Description: "AI 画图生成", Accent: "stone", Href: "https://build.fufuapi.online"},
		{ID: "status", Stamp: "状态", Title: "API / 模型状态", Description: "连通性检测、模型可用性与手动测试", Accent: "moss", Href: "/status"},
		{ID: "combine", Stamp: "合卡", Title: "合卡工具", Description: "自助合并额度卡，复用统一 NewAPI 配置", Accent: "clay", Href: "/combine"},
		{ID: "activity", Stamp: "活动", Title: "活动前台", Description: "抽奖、刮刮卡与福利入口", Accent: "stone", Href: "/activity"},
	}}
}

func defaultNavigationLineCategories() []NavigationLineCategory {
	return []NavigationLineCategory{
		{
			Kind: "api",
			Name: "API 次数站",
			Lines: navcore.DefaultNavigationLines([]string{
				"https://api.fufuapi.top",
				"https://api.fufuapi.online",
				"https://api.fufuflower.top",
			}),
		},
		{
			Kind: "token",
			Name: "Token 站",
			Lines: navcore.DefaultNavigationLines([]string{
				"https://token.fufuapi.top",
				"https://token.fufuapi.online",
				"https://token.fufuflower.top",
			}),
		},
	}
}

func normalizeNavigationConfig(cfg NavigationConfig) NavigationConfig {
	return navcore.NormalizeNavigationConfig(cfg, defaultNavigationConfig())
}

func normalizeNavigationAccent(value string) string {
	return navcore.NormalizeNavigationAccent(value)
}

func normalizeNavigationLinks(links []NavigationLinkConfig) []NavigationLinkConfig {
	return navcore.NormalizeNavigationLinks(links)
}

func cloneNavigationConfig(cfg NavigationConfig) NavigationConfig {
	return navcore.CloneNavigationConfig(cfg)
}

func sortNavigationCardsForDisplay(cards []NavigationCardConfig) []NavigationCardConfig {
	return navcore.SortNavigationCardsForDisplay(cards)
}

func navigationIDFromTitle(title string) string {
	return navcore.NavigationIDFromTitle(title)
}

func navigationCardResponses(cards []NavigationCardConfig) []map[string]any {
	return navcore.NavigationCardResponses(cards)
}
