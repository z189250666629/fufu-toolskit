package config

import (
	"encoding/json"
	"testing"
)

func TestDecodeManagedSitesJSONUsesNumbers(t *testing.T) {
	data, err := decodeManagedSitesJSON(`{"managedApiSites":[{"name":"primary","quotaUnit":1234567890123456789}]}`)
	if err != nil {
		t.Fatalf("decodeManagedSitesJSON: %v", err)
	}
	items := coerceItems(data)
	if len(items) != 1 {
		t.Fatalf("items = %#v", items)
	}
	if quota, ok := items[0]["quotaUnit"].(json.Number); !ok || quota.String() != "1234567890123456789" {
		t.Fatalf("quotaUnit = %#v", items[0]["quotaUnit"])
	}
}

func TestDecodeManagedSitesJSONReportsInvalidJSON(t *testing.T) {
	if data, err := decodeManagedSitesJSON(`not-json`); err == nil || data != nil {
		t.Fatalf("expected invalid JSON error, got data=%#v err=%v", data, err)
	}
}
