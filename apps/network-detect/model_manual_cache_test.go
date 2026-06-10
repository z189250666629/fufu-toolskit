package main

import "testing"

func TestApplyManualRecomputesCachedRowSummary(t *testing.T) {
	status := &ModelStatus{
		Totals: map[string]int{"unknown": 1, "operational": 0, "degraded": 0, "down": 0},
		Models: []ModelRow{
			{
				Model:           "gpt-test",
				Status:          "unknown",
				ConfiguredSites: 1,
				PerSite: map[string]*ModelCell{
					"site-a": {Configured: true, SiteName: "site-a", Model: "gpt-test", Status: "unknown"},
				},
			},
		},
	}

	applyManual(status, "site-a", "gpt-test", testRecord{OK: true, Status: "operational", TestedAt: 123}, 456)

	row := status.Models[0]
	if row.Status != "operational" || row.OperationalSites != 1 || row.ConfiguredSites != 1 {
		t.Fatalf("row summary not recomputed: %#v", row)
	}
	if status.Totals["unknown"] != 0 || status.Totals["operational"] != 1 {
		t.Fatalf("totals not recomputed: %#v", status.Totals)
	}
	cell := row.PerSite["site-a"]
	if cell.SuccessCount != 1 || cell.RequestCount != 1 || cell.LastSeenAt != 123 || cell.NextTestAllowedAt != 456 {
		t.Fatalf("cell not updated consistently: %#v", cell)
	}
}
