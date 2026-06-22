package main

import (
	"fufu/connectivitycore"
	"os"
	"strings"
)

func env(name string) string { return strings.TrimSpace(os.Getenv(name)) }

func connectivityGroups() []map[string]any {
	groups, _ := connectivityGroupsWithError()
	return groups
}

func connectivityGroupsWithError() ([]map[string]any, string) {
	if inline := env("CONNECTIVITY_TARGETS"); inline != "" {
		groups, err := connectivitycore.ParseGroupsJSON(inline)
		if err != nil {
			return nil, "CONNECTIVITY_TARGETS 不是有效 JSON"
		}
		if len(groups) > 0 {
			return groups, ""
		}
		return defaultConnectivityGroups(), ""
	}

	inputs := defaultConnectivityGroupInputs()
	managed := managedConnectivityTargetsByKind()
	out := make([]connectivitycore.GroupInput, 0, len(inputs))
	for _, input := range inputs {
		urls := connectivityGroupURLs(
			input.ID,
			input.URLs,
			managed[input.ID],
		)
		if len(urls) == 0 {
			continue
		}
		out = append(out, connectivitycore.GroupInput{
			ID:   input.ID,
			Name: input.Name,
			URLs: urls,
		})
	}
	if len(out) > 0 {
		return connectivitycore.BuildGroups(out), ""
	}
	return defaultConnectivityGroups(), ""
}

func defaultConnectivityGroups() []map[string]any {
	return connectivitycore.BuildGroups(defaultConnectivityGroupInputs())
}

func connectivityGroupURLs(kind string, defaults []string, managed []string) []string {
	if urls := explicitConnectivityURLs(kind); len(urls) > 0 {
		return urls
	}
	return connectivitycore.PublicBrowserTargets(append(append([]string{}, defaults...), managed...))
}

func explicitConnectivityURLs(kind string) []string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "api":
		return connectivitycore.SplitPublicTargetList(firstNonEmpty(env("CONNECTIVITY_API_URLS"), env("FUFU_API_URLS")))
	case "token":
		return connectivitycore.SplitPublicTargetList(firstNonEmpty(env("CONNECTIVITY_TOKEN_URLS"), env("FUFU_TOKEN_URLS")))
	default:
		return nil
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
