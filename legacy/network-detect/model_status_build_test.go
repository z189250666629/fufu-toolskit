package main

import (
	"testing"

	"fufu/newapi"
)

func TestProjectModelStatusMergesRuntimeLinesByLogicalSite(t *testing.T) {
	now := int64(2_000)
	status := projectModelStatus(
		modelStatusBuildPlan{
			Sites: []newapi.Site{
				{Name: "次数fufu", Category: "api", LineName: "国内加速", URL: "https://api-a.example.test", UserID: "1", QuotaUnit: 500000, Currency: "$", RechargeRatio: 1},
				{Name: "次数fufu", Category: "api", LineName: "海外线路", URL: "https://api-b.example.test", UserID: "1", QuotaUnit: 500000, Currency: "$", RechargeRatio: 1},
			},
			Now:           now,
			WindowSeconds: modelStatusWindowSeconds,
		},
		[]modelStatusSiteData{
			{
				Site:        newapi.Site{Name: "次数fufu", Category: "api", LineName: "国内加速", URL: "https://api-a.example.test", UserID: "1", QuotaUnit: 500000, Currency: "$", RechargeRatio: 1},
				SuccessLogs: []LogRow{{ModelName: "gpt-a", Group: "mix", RequestID: "req-1", CreatedAt: 1_990}},
				Channels: []Channel{
					{ID: 10, Status: channelStatusEnabled, Models: []string{"gpt-a"}, Groups: []string{"mix"}},
					{ID: 20, Status: channelStatusEnabled, Models: []string{"gpt-b"}, Groups: []string{"mix"}},
				},
				Pricing: map[string]Pricing{"gpt-a": {Input: 1, Output: 2, Currency: "$"}},
			},
			{
				Site:        newapi.Site{Name: "次数fufu", Category: "api", LineName: "海外线路", URL: "https://api-b.example.test", UserID: "1", QuotaUnit: 500000, Currency: "$", RechargeRatio: 1},
				SuccessLogs: []LogRow{{ModelName: "gpt-a", Group: "mix", RequestID: "req-1", CreatedAt: 1_990}},
				ErrorLogs:   []LogRow{{ModelName: "gpt-a", Group: "vip", RequestID: "req-2", CreatedAt: 1_995}},
				Channels: []Channel{
					{ID: 10, Status: channelStatusEnabled, Models: []string{"gpt-a"}, Groups: []string{"mix"}},
					{ID: 30, Status: channelStatusEnabled, Models: []string{"gpt-a"}, Groups: []string{"vip"}},
				},
				Pricing: map[string]Pricing{"gpt-b": {Input: 3, Output: 4, Currency: "$"}},
			},
		},
		nil,
	)

	if len(status.Sites) != 1 || status.Totals["siteCount"] != 1 {
		t.Fatalf("runtime lines should collapse into one logical site, sites=%#v totals=%#v", status.Sites, status.Totals)
	}
	site := status.Sites[0]
	if site.Site.Name != "次数fufu" || site.RequestCount != 1 || site.SuccessCount != 1 || site.FailureCount != 0 || site.Status != "operational" {
		t.Fatalf("merged site summary = %#v", site)
	}
	if len(site.Groups) != 1 || site.Groups[0] != "mix" {
		t.Fatalf("api model status should be fixed to mix, got %#v", site.Groups)
	}
	if len(status.Models) != 2 {
		t.Fatalf("merged model rows = %#v", status.Models)
	}
	row := status.Models[0]
	if row.Model != "gpt-a" || row.ConfiguredSites != 1 || row.OperationalSites != 1 || row.Status != "operational" {
		t.Fatalf("gpt-a row summary = %#v", row)
	}
	cell := row.PerSite["次数fufu"]
	if cell == nil || cell.RequestCount != 1 || cell.SuccessCount != 1 || cell.FailureCount != 0 || cell.EnabledChannelCount != 1 || cell.TotalChannelCount != 1 {
		t.Fatalf("merged gpt-a cell = %#v", cell)
	}
	if _, ok := row.PerSite["国内加速"]; ok {
		t.Fatalf("line name must not become a separate site key: %#v", row.PerSite)
	}
}
