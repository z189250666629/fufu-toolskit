package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandleAdminSaleCardsConfigReturnsPlansAndDefaultSchedule(t *testing.T) {
	setupSaleCardConfigTestRoot(t)
	t.Setenv("ADMIN_TOKEN", "test-admin-token")

	req := httptest.NewRequest(http.MethodGet, "/api/admin/sale-cards/config", nil)
	req.Header.Set("Authorization", "Bearer test-admin-token")
	w := httptest.NewRecorder()

	apiRoute(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	var body saleCardAdminConfigResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, w.Body.String())
	}
	if len(body.Plans) < 6 {
		t.Fatalf("plans=%#v, want default special/monthly plans", body.Plans)
	}
	if body.Schedule.Time != "09:00" || body.Schedule.Timezone != "Asia/Shanghai" || body.Schedule.Enabled {
		t.Fatalf("default schedule=%#v", body.Schedule)
	}
	if body.Plans[0].ID == "" || body.Plans[0].Name == "" || body.Plans[0].Count != 0 {
		t.Fatalf("plan should expose display metadata but no run count default: %#v", body.Plans[0])
	}
}

func TestHandleAdminSaleCardsConfigPersistsValidatedSchedule(t *testing.T) {
	root := setupSaleCardConfigTestRoot(t)
	t.Setenv("ADMIN_TOKEN", "test-admin-token")
	payload := `{
		"schedule": {
			"enabled": true,
			"time": "08:30",
			"timezone": "Asia/Shanghai",
			"jobs": [
				{"plan": "fufu-mix-special-55", "count": 2, "enabled": true},
				{"plan": "fufu-mix-month-100", "count": 3, "enabled": false}
			]
		}
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/admin/sale-cards/config", strings.NewReader(payload))
	req.Header.Set("Authorization", "Bearer test-admin-token")
	w := httptest.NewRecorder()

	apiRoute(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	var body saleCardAdminConfigResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, w.Body.String())
	}
	if !body.Schedule.Enabled || body.Schedule.Time != "08:30" || len(body.Schedule.Jobs) != 2 {
		t.Fatalf("saved schedule=%#v", body.Schedule)
	}
	if body.Schedule.Jobs[0].Plan != "fufu-mix-special-55" || body.Schedule.Jobs[0].Count != 2 || !body.Schedule.Jobs[0].Enabled {
		t.Fatalf("first job=%#v", body.Schedule.Jobs[0])
	}
	if _, err := os.Stat(filepath.Join(root, "data", "sale-card-schedule.json")); err != nil {
		t.Fatalf("schedule should be persisted: %v", err)
	}
}

func TestHandleAdminSaleCardsConfigRejectsUnknownSchedulePlan(t *testing.T) {
	setupSaleCardConfigTestRoot(t)
	t.Setenv("ADMIN_TOKEN", "test-admin-token")
	payload := `{"schedule":{"enabled":true,"time":"08:30","timezone":"Asia/Shanghai","jobs":[{"plan":"unknown","count":1,"enabled":true}]}}`
	req := httptest.NewRequest(http.MethodPost, "/api/admin/sale-cards/config", strings.NewReader(payload))
	req.Header.Set("Authorization", "Bearer test-admin-token")
	w := httptest.NewRecorder()

	apiRoute(w, req)

	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "未知上架计划") {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
}

func setupSaleCardConfigTestRoot(t *testing.T) string {
	t.Helper()
	oldRoot := rootDir
	root := t.TempDir()
	rootDir = root
	t.Cleanup(func() { rootDir = oldRoot })
	return root
}
