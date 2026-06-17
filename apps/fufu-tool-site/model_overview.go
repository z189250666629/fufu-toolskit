package main

import (
	"context"
	"net/url"
)

func buildOverview(ctx context.Context, q url.Values) map[string]any {
	status := getModelStatus(ctx, false)
	return map[string]any{"configured": status.Configured, "configError": status.ConfigError, "generatedAt": status.GeneratedAt, "sites": status.Sites, "totals": status.Totals, "modelAvailability": status.Models, "allLogs": []any{}}
}
