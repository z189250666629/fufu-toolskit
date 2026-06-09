package tokens

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"fufu/newapi"
)

func TestNormalizeKeys(t *testing.T) {
	got := NormalizeKeys([]string{"abcdefghijkl sk-defghijkl\nabcdefghijkl"})
	if len(got) != 2 || got[0] != "sk-abcdefghijkl" || got[1] != "sk-defghijkl" {
		t.Fatalf("NormalizeKeys = %#v", got)
	}
}

func TestBatchSearchFoundAndMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Query().Get("token")
		if key == "foundxxxxxx" {
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": []any{map[string]any{"id": 1, "key": "foundxxxxxx", "name": "card", "remain_quota": 500000, "interval_unit": 3, "status": 1}}})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": []any{}})
	}))
	defer server.Close()
	svc := NewService(newapi.NewClient(newapi.Site{URL: server.URL, Token: "x", UserID: "1"}))
	_, found, missing, err := svc.BatchSearch(context.Background(), []string{"foundxxxxxx", "missingxxxxx"})
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 || found[0].Key != "sk-foundxxxxxx" {
		t.Fatalf("found = %#v", found)
	}
	if len(missing) != 1 || missing[0] != "sk-missingxxxxx" {
		t.Fatalf("missing = %#v", missing)
	}
}

func TestQuotaConversion(t *testing.T) {
	svc := &Service{QuotaUnit: 500000}
	if svc.DollarsToQuota(2.5) != 1250000 {
		t.Fatalf("bad dollars to quota")
	}
	if svc.QuotaToDollars(1000000) != 2 {
		t.Fatalf("bad quota to dollars")
	}
}

func TestSearchCreateUpdateDelete(t *testing.T) {
	var created map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/token/":
			_ = json.NewDecoder(r.Body).Decode(&created)
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
		case r.Method == http.MethodGet && r.URL.Path == "/api/token/search":
			keyword := r.URL.Query().Get("keyword")
			if keyword == "created-name" {
				_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": []any{map[string]any{"id": 2, "key": "createdkeyxx", "name": "created-name", "remain_quota": 10, "status": 1}}})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": []any{}})
		case r.Method == http.MethodPut && r.URL.Path == "/api/token/":
			var raw map[string]any
			_ = json.NewDecoder(r.Body).Decode(&raw)
			if raw["name"] != "renamed" {
				t.Fatalf("bad update body: %#v", raw)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
		case r.Method == http.MethodDelete && r.URL.Path == "/api/token/2":
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()
	svc := NewService(newapi.NewClient(newapi.Site{URL: server.URL, Token: "x", UserID: "1"}))
	if res, _, err := svc.CreateToken(context.Background(), map[string]any{"name": "created-name"}); err != nil || !res.OK() || created["name"] != "created-name" {
		t.Fatalf("CreateToken res=%+v created=%#v err=%v", res, created, err)
	}
	found, err := svc.SearchTokenByName(context.Background(), "created-name")
	if err != nil || found == nil || found.ID != 2 {
		t.Fatalf("SearchTokenByName found=%#v err=%v", found, err)
	}
	found.Raw["name"] = "renamed"
	if res, _, err := svc.UpdateTokenRaw(context.Background(), found.Raw); err != nil || !res.OK() {
		t.Fatalf("UpdateTokenRaw res=%+v err=%v", res, err)
	}
	if ok, res, err := svc.DeleteToken(context.Background(), 2); err != nil || !ok || !res.OK() {
		t.Fatalf("DeleteToken ok=%v res=%+v err=%v", ok, res, err)
	}
}
