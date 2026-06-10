package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"fufu/newapi"
)

func TestLoadSiteLogsPaginatesUntilShortPage(t *testing.T) {
	pages := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/log/self" {
			t.Fatalf("unexpected log path: %s", r.URL.String())
		}
		page := r.URL.Query().Get("p")
		if r.URL.Query().Get("page") != page {
			t.Fatalf("p/page mismatch: %s", r.URL.RawQuery)
		}
		pages = append(pages, page)

		count := modelLogPageSize
		if page == "2" {
			count = 2
		}
		if page != "1" && page != "2" {
			t.Fatalf("unexpected page %s", page)
		}
		items := make([]map[string]any, 0, count)
		for i := 0; i < count; i++ {
			items = append(items, map[string]any{
				"model_name":   fmt.Sprintf("model-%s-%d", page, i),
				"request_id":   fmt.Sprintf("req-%s-%d", page, i),
				"created_time": i,
			})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": items})
	}))
	t.Cleanup(server.Close)

	rows, msg := loadSiteLogs(newapi.Site{URL: server.URL, Token: "token", UserID: "1"}, 2, 10, 20)

	if msg != "" {
		t.Fatalf("msg = %q", msg)
	}
	if len(rows) != modelLogPageSize+2 {
		t.Fatalf("rows = %d, want %d", len(rows), modelLogPageSize+2)
	}
	if strings.Join(pages, ",") != "1,2" {
		t.Fatalf("pages = %#v", pages)
	}
	if rows[len(rows)-1].ModelName != "model-2-1" {
		t.Fatalf("last row = %#v", rows[len(rows)-1])
	}
}

func TestLoadSiteLogsReportsErrorWhenLaterPageFails(t *testing.T) {
	pages := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/log/self" {
			t.Fatalf("unexpected log path: %s", r.URL.String())
		}
		page := r.URL.Query().Get("p")
		pages = append(pages, page)
		if page == "2" {
			http.Error(w, "page failed", http.StatusBadGateway)
			return
		}
		if page != "1" {
			t.Fatalf("unexpected page %s", page)
		}
		items := make([]map[string]any, 0, modelLogPageSize)
		for i := 0; i < modelLogPageSize; i++ {
			items = append(items, map[string]any{
				"model_name": fmt.Sprintf("model-%d", i),
				"request_id": fmt.Sprintf("req-%d", i),
			})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": items})
	}))
	t.Cleanup(server.Close)

	rows, msg := loadSiteLogs(newapi.Site{URL: server.URL, Token: "token", UserID: "1"}, 2, 10, 20)

	if msg == "" || !strings.Contains(msg, "502") {
		t.Fatalf("expected later-page error, rows=%d msg=%q", len(rows), msg)
	}
	if len(rows) != modelLogPageSize {
		t.Fatalf("partial rows should remain available with error, got %d", len(rows))
	}
	if strings.Join(pages, ",") != "1,2" {
		t.Fatalf("pages = %#v", pages)
	}
}
