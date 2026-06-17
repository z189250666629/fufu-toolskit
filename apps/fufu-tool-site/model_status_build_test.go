package main

import (
	"testing"

	"fufu/newapi"
)

func TestProjectModelStatusBuildsRowsGroupsManualAndTotals(t *testing.T) {
	now := int64(1_000)
	status := projectModelStatus(
		modelStatusBuildPlan{
			Sites:         []newapi.Site{{Name: "primary", Category: "api", URL: "https://primary.example.test", UserID: "7", QuotaUnit: 500000, Currency: "$", RechargeRatio: 1}},
			Now:           now,
			WindowSeconds: modelStatusWindowSeconds,
		},
		[]modelStatusSiteData{{
			Site: newapi.Site{Name: "primary", Category: "api", URL: "https://primary.example.test", UserID: "7", QuotaUnit: 500000, Currency: "$", RechargeRatio: 1},
			SuccessLogs: []LogRow{
				{ModelName: "gpt-a", Group: "vip", CreatedAt: 990},
				{ModelName: "gpt-a", Group: "default", CreatedAt: 970},
			},
			ErrorLogs: []LogRow{
				{ModelName: "gpt-a", Group: "vip", CreatedAt: 980},
			},
			Channels: []Channel{
				{ID: 1, Status: channelStatusEnabled, Models: []string{"gpt-a"}, Groups: []string{"default", "vip"}},
				{ID: 2, Status: 0, Models: []string{"gpt-a"}, Groups: []string{"vip"}},
				{ID: 3, Status: channelStatusEnabled, Models: []string{"gpt-b"}, Groups: []string{"default"}},
			},
			Pricing: map[string]Pricing{
				"gpt-a": {Input: 1, Output: 2, Currency: "$"},
			},
			LogError:      "log warning",
			ChannelsError: "channel warning",
			PricingError:  "pricing warning",
		}},
		func(siteName, model, group string) modelManualProjection {
			if siteName == "primary" && model == "gpt-a" && group == "vip" {
				return modelManualProjection{Record: testRecord{OK: true, Group: "vip", TestedAt: 995}, HasRecord: true, NextAllowedAt: 1_100}
			}
			return modelManualProjection{}
		},
	)

	if !status.Configured || status.GeneratedAt != now || status.ExpiresAt != now+modelStatusWindowSeconds {
		t.Fatalf("status timing/config = %#v", status)
	}
	if len(status.Sites) != 1 {
		t.Fatalf("sites = %#v", status.Sites)
	}
	site := status.Sites[0]
	if site.Site.Name != "primary" || site.Status != "degraded" || site.RequestCount != 3 || site.SuccessCount != 2 || site.FailureCount != 1 {
		t.Fatalf("site status = %#v", site)
	}
	if site.LogError != "log warning" || site.ChannelsError != "channel warning" || site.PricingError != "pricing warning" {
		t.Fatalf("site errors = %#v", site)
	}
	if len(site.Groups) != 2 || site.Groups[0] != "default" || site.Groups[1] != "vip" {
		t.Fatalf("site groups = %#v", site.Groups)
	}

	if len(status.Models) != 2 || status.Models[0].Model != "gpt-a" || status.Models[1].Model != "gpt-b" {
		t.Fatalf("models should be sorted by name, got %#v", status.Models)
	}
	row := status.Models[0]
	if row.Status != "degraded" || row.ConfiguredSites != 1 || row.OperationalSites != 0 {
		t.Fatalf("row summary = %#v", row)
	}
	cell := row.PerSite["primary"]
	if cell == nil || cell.RequestCount != 3 || cell.SuccessCount != 2 || cell.FailureCount != 1 || cell.Status != "degraded" {
		t.Fatalf("cell = %#v", cell)
	}
	if cell.EnabledChannelCount != 1 || cell.TotalChannelCount != 2 || cell.LastSeenAt != 990 {
		t.Fatalf("cell channel/seen summary = %#v", cell)
	}
	if cell.Pricing == nil || cell.Pricing.Input != 1 || cell.Pricing.Output != 2 {
		t.Fatalf("cell pricing = %#v", cell.Pricing)
	}
	groupCell := cell.GroupStats["vip"]
	if groupCell == nil || groupCell.RequestCount != 2 || groupCell.SuccessCount != 1 || groupCell.FailureCount != 1 || groupCell.NextTestAllowedAt != 1_100 {
		t.Fatalf("vip group cell = %#v", groupCell)
	}
	if rec, ok := groupCell.ManualTest.(testRecord); !ok || !rec.OK || rec.Group != "vip" {
		t.Fatalf("manual record = %#v", groupCell.ManualTest)
	}

	wantTotals := map[string]int{
		"siteCount":    1,
		"modelCount":   2,
		"requestCount": 3,
		"successCount": 2,
		"failureCount": 1,
		"operational":  0,
		"degraded":     1,
		"down":         0,
		"unknown":      1,
	}
	for key, want := range wantTotals {
		if got := status.Totals[key]; got != want {
			t.Fatalf("total %s = %d, want %d; totals=%#v", key, got, want, status.Totals)
		}
	}
}

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

func TestProjectModelStatusKeepsEmptyConfiguredShape(t *testing.T) {
	status := projectModelStatus(
		modelStatusBuildPlan{ConfigError: "public config error", Now: 200, WindowSeconds: 60},
		nil,
		nil,
	)

	if status.Configured || status.ConfigError != "public config error" {
		t.Fatalf("empty status config = %#v", status)
	}
	if status.GeneratedAt != 200 || status.ExpiresAt != 260 || status.RefreshEverySeconds != 60 {
		t.Fatalf("empty status timing = %#v", status)
	}
	if len(status.Models) != 0 || status.Totals["siteCount"] != 0 || status.Totals["modelCount"] != 0 {
		t.Fatalf("empty status model/totals = %#v", status)
	}
}
