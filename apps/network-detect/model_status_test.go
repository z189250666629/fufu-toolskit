package main

import (
	"strings"
	"testing"

	"fufu/newapi"
)

func TestSanitizeLogAcceptsNewAPIAliases(t *testing.T) {
	row := sanitizeLog(map[string]any{
		"modelName":    "gpt-test",
		"tokenName":    "token-a",
		"groups":       "vip",
		"requestId":    "req-1",
		"quota":        float64(42),
		"created_time": float64(123456),
		"status":       float64(2),
	})

	if row.ModelName != "gpt-test" || row.TokenName != "token-a" || row.Group != "vip" || row.RequestID != "req-1" {
		t.Fatalf("unexpected sanitized log identity: %#v", row)
	}
	if row.Quota != 42 || row.CreatedAt != 123456 || row.Status != 2 {
		t.Fatalf("unexpected sanitized log counters: %#v", row)
	}
}

func TestSanitizeChannelParsesModelsAndGroups(t *testing.T) {
	channel := sanitizeChannel(map[string]any{
		"id":            float64(7),
		"channel_name":  "primary",
		"status":        float64(channelStatusEnabled),
		"models":        `["model-b","model-a","model-b"]`,
		"groups":        "vip default|vip",
		"response_time": float64(321),
	})

	if channel.ID != 7 || channel.Name != "primary" || channel.Status != channelStatusEnabled {
		t.Fatalf("unexpected sanitized channel identity: %#v", channel)
	}
	if strings.Join(channel.Models, ",") != "model-a,model-b" {
		t.Fatalf("models = %#v", channel.Models)
	}
	if strings.Join(channel.Groups, ",") != "default,vip" {
		t.Fatalf("groups = %#v", channel.Groups)
	}
	if channel.ResponseTime != 321 {
		t.Fatalf("response time = %d", channel.ResponseTime)
	}
}

func TestBuildCellSummarizesChannelsLogsAndPricing(t *testing.T) {
	price := Pricing{Input: 0.1, Output: 0.2, Currency: "CNY"}
	cell := buildCell(
		"site-a",
		"model-a",
		[]Channel{
			{Status: channelStatusEnabled, Groups: []string{"vip", "default"}},
			{Status: 2, Groups: []string{"vip"}},
		},
		[]LogRow{{CreatedAt: 100}, {CreatedAt: 300}},
		[]LogRow{{CreatedAt: 200}},
		price,
	)

	if !cell.Configured || cell.SiteName != "site-a" || cell.Model != "model-a" {
		t.Fatalf("unexpected cell identity: %#v", cell)
	}
	if cell.RequestCount != 3 || cell.SuccessCount != 2 || cell.FailureCount != 1 {
		t.Fatalf("unexpected cell counters: %#v", cell)
	}
	if cell.Status != "degraded" {
		t.Fatalf("status = %s", cell.Status)
	}
	if cell.LastSuccessAt != 300 || cell.LastFailureAt != 200 || cell.LastSeenAt != 300 {
		t.Fatalf("unexpected last seen timestamps: %#v", cell)
	}
	if cell.TotalChannelCount != 2 || cell.EnabledChannelCount != 1 {
		t.Fatalf("unexpected channel counts: %#v", cell)
	}
	if strings.Join(cell.Groups, ",") != "default,vip" {
		t.Fatalf("groups = %#v", cell.Groups)
	}
	if cell.Pricing == nil || cell.Pricing.Currency != "CNY" {
		t.Fatalf("pricing = %#v", cell.Pricing)
	}
}

func TestPricingFromRawAppliesRechargeRatioAndFallbacks(t *testing.T) {
	price := pricingFromRaw(newapi.Site{RechargeRatio: 2.5, Currency: "CNY"}, map[string]any{
		"model_ratio":     float64(0.1),
		"completionRatio": float64(0.3),
	})

	if price.Input != 0.25 || price.Output != 0.75 || price.Currency != "CNY" {
		t.Fatalf("pricing = %#v", price)
	}
}
