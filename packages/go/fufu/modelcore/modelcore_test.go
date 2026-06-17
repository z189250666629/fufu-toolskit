package modelcore

import (
	"encoding/json"
	"testing"
)

func TestSanitizeLogAndChannelUseAliasesAndSortedLists(t *testing.T) {
	log := SanitizeLog(map[string]any{"modelName": "gpt-test", "groups": "vip|default", "createdTime": json.Number("42")})
	if log.ModelName != "gpt-test" || log.Group != "vip|default" || log.CreatedAt != 42 {
		t.Fatalf("log = %#v", log)
	}

	ch := SanitizeChannel(map[string]any{"id": 3, "channel_name": "main", "status": 1, "models": "b,a,a", "groups": []any{"vip", "default"}})
	if ch.ID != 3 || ch.Name != "main" || len(ch.Models) != 2 || ch.Models[0] != "a" || ch.Models[1] != "b" {
		t.Fatalf("channel = %#v", ch)
	}
	if len(ch.Groups) != 2 || ch.Groups[0] != "default" || ch.Groups[1] != "vip" {
		t.Fatalf("groups = %#v", ch.Groups)
	}
}

func TestStatusAndRowSummary(t *testing.T) {
	if StatusFromCounts(8, 2) != "operational" || StatusFromCounts(1, 9) != "degraded" || StatusFromCounts(0, 3) != "down" || StatusFromCounts(0, 0) != "unknown" {
		t.Fatalf("unexpected statuses")
	}

	row := ModelRow{PerSite: map[string]*ModelCell{
		"a": {Configured: true, Status: "operational"},
		"b": {Configured: true, Status: "down"},
	}}
	RecomputeModelRowSummary(&row)
	if row.Status != "down" || row.ConfiguredSites != 2 || row.OperationalSites != 1 {
		t.Fatalf("row = %#v", row)
	}
}

func TestSelectModelTestChannelsSortsFastEnabledMatches(t *testing.T) {
	got := SelectModelTestChannels([]Channel{
		{ID: 3, Status: 1, Models: []string{"m"}, Groups: []string{"g"}, ResponseTime: 0},
		{ID: 2, Status: 1, Models: []string{"m"}, Groups: []string{"g"}, ResponseTime: 20},
		{ID: 1, Status: 0, Models: []string{"m"}, Groups: []string{"g"}, ResponseTime: 1},
		{ID: 4, Status: 1, Models: []string{"x"}, Groups: []string{"g"}, ResponseTime: 1},
	}, "m", "g")

	if len(got) != 2 || got[0].ID != 2 || got[1].ID != 3 {
		t.Fatalf("channels = %#v", got)
	}
}

func TestPricingFromRawAppliesRechargeRatio(t *testing.T) {
	got := PricingFromRaw(PricingSite{RechargeRatio: 0.5, Currency: "¥"}, map[string]any{"prompt_ratio": 2, "completion_ratio": 4})
	if got.Input != 1 || got.Output != 2 || got.Currency != "¥" {
		t.Fatalf("pricing = %#v", got)
	}
}

func TestPricingFromRawSupportsRequestPricing(t *testing.T) {
	got := PricingFromRaw(PricingSite{RechargeRatio: 2, Currency: "$"}, map[string]any{
		"quota_type":  1,
		"model_ratio": 0,
		"model_price": 0.975,
	})

	if got.Type != "request" || got.Request != 1.95 || got.Input != 0 || got.Output != 0 || got.Currency != "$" {
		t.Fatalf("pricing = %#v", got)
	}
}
