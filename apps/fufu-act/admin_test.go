package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleAdminStatsReturns500OnStatsQueryError(t *testing.T) {
	setupScratchLockTestDB(t)
	t.Setenv("ADMIN_TOKEN", "")
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/admin/stats?token=Chukayu98", nil)
	w := httptest.NewRecorder()

	handleAdminStats(w, req)

	if w.Code != http.StatusInternalServerError || !strings.Contains(w.Body.String(), "服务器错误") {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
}
