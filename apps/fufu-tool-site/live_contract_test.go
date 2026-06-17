package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestLiveToolSiteGenerateMergeContract(t *testing.T) {
	if os.Getenv("FUFU_LIVE_TOOL_SITE_GENERATE_MERGE") != "1" {
		t.Skip("set FUFU_LIVE_TOOL_SITE_GENERATE_MERGE=1 to run the live generate+merge contract")
	}
	initLiveToolSiteRuntime(t)
	cookie := liveAdminCookie(t)

	count := liveIntEnv("FUFU_LIVE_GENERATE_COUNT", 3)
	quota := liveFloatEnv("FUFU_LIVE_GENERATE_QUOTA", 111)
	intervalUnit := liveIntEnv("FUFU_LIVE_GENERATE_INTERVAL_UNIT", 9)
	group := liveStringEnv("FUFU_LIVE_GENERATE_GROUP", "mix")

	generatedKeys := []string{}
	merged := false
	defer func() {
		if !merged && len(generatedKeys) > 0 {
			liveDeleteKeysBySearch(t, cookie, generatedKeys)
		}
	}()

	generateReq := jsonRequest(t, http.MethodPost, "/api/generate", map[string]any{
		"count":        count,
		"quota":        quota,
		"intervalUnit": intervalUnit,
		"group":        group,
	})
	generateReq.AddCookie(cookie)
	generateRec := httptest.NewRecorder()
	route(generateRec, generateReq)
	if generateRec.Code != http.StatusOK {
		t.Fatalf("live generate status=%d error=%q", generateRec.Code, liveErrorFromRecorder(generateRec))
	}
	var generateBody struct {
		Keys   []string `json:"keys"`
		Errors []string `json:"errors"`
	}
	if err := json.Unmarshal(generateRec.Body.Bytes(), &generateBody); err != nil {
		t.Fatalf("decode live generate response: %v", err)
	}
	generatedKeys = generateBody.Keys
	if len(generateBody.Errors) > 0 || len(generatedKeys) != count {
		t.Fatalf("live generate produced keys=%d errors=%d", len(generatedKeys), len(generateBody.Errors))
	}

	mergeReq := jsonRequest(t, http.MethodPost, "/api/merge", map[string]any{
		"keys":         generatedKeys,
		"intervalUnit": intervalUnit,
	})
	mergeReq.AddCookie(cookie)
	mergeRec := httptest.NewRecorder()
	route(mergeRec, mergeReq)
	if mergeRec.Code != http.StatusOK {
		t.Fatalf("live merge accepted status=%d error=%q", mergeRec.Code, liveErrorFromRecorder(mergeRec))
	}
	var accepted struct {
		JobID string `json:"jobId"`
	}
	if err := json.Unmarshal(mergeRec.Body.Bytes(), &accepted); err != nil || strings.TrimSpace(accepted.JobID) == "" {
		t.Fatalf("decode live merge accepted response: jobID=%q err=%v", accepted.JobID, err)
	}

	status := livePollMergeStatus(t, cookie, accepted.JobID)
	if status.Status != "done" || !status.Result.Success {
		t.Fatalf("live merge failed: status=%q error=%q success=%v", status.Status, status.Error, status.Result.Success)
	}
	merged = true
	deleteOK := 0
	for _, item := range status.Result.DeleteResults {
		if item.OK {
			deleteOK++
		}
	}
	if deleteOK != count || status.Result.NewCard.RemainQuota <= 0 || status.Result.NewCard.IntervalUnit != intervalUnit || strings.TrimSpace(status.Result.NewCard.Key) == "" {
		t.Fatalf("live merge result: deleteOK=%d/%d quota=%d unit=%d newKeyPresent=%v", deleteOK, count, status.Result.NewCard.RemainQuota, status.Result.NewCard.IntervalUnit, strings.TrimSpace(status.Result.NewCard.Key) != "")
	}

	liveDeleteKeysBySearch(t, cookie, []string{status.Result.NewCard.Key})
}

func TestLiveToolSiteMCYStockContract(t *testing.T) {
	if os.Getenv("FUFU_LIVE_MCY_STOCK") != "1" {
		t.Skip("set FUFU_LIVE_MCY_STOCK=1 to run the live MCY stock contract")
	}
	initLiveToolSiteRuntime(t)
	cookie := liveAdminCookie(t)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/sale-cards/stock", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	route(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("live MCY stock status=%d error=%q", rec.Code, liveErrorFromRecorder(rec))
	}
	var body struct {
		Plans []struct {
			PlanID       string `json:"planId"`
			CurrentStock int    `json:"currentStock"`
		} `json:"plans"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode live MCY stock response: %v", err)
	}
	if len(body.Plans) == 0 {
		t.Fatal("live MCY stock returned zero plans")
	}
}

func TestLiveToolSiteSaleCardRunContract(t *testing.T) {
	if os.Getenv("FUFU_LIVE_SALE_CARD_RUN") != "1" {
		t.Skip("set FUFU_LIVE_SALE_CARD_RUN=1 to run the live sale-card upload contract")
	}
	initLiveToolSiteRuntime(t)
	cookie := liveAdminCookie(t)

	plan := liveStringEnv("FUFU_LIVE_SALE_CARD_PLAN", "fufu-mix-special-55")
	count := liveIntEnv("FUFU_LIVE_SALE_CARD_COUNT", 1)
	req := jsonRequest(t, http.MethodPost, "/api/admin/sale-cards/run", map[string]any{
		"plan":  plan,
		"count": count,
	})
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	route(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("live sale-card run status=%d error=%q", rec.Code, liveErrorFromRecorder(rec))
	}
	var body struct {
		Generated int      `json:"generated"`
		Uploaded  int      `json:"uploaded"`
		Keys      []string `json:"keys"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode live sale-card run response: %v", err)
	}
	if body.Generated != count || body.Uploaded != count || len(body.Keys) != count {
		t.Fatalf("live sale-card counts generated=%d uploaded=%d keys=%d want=%d", body.Generated, body.Uploaded, len(body.Keys), count)
	}
}

func TestLiveToolSiteSaleCardTestKeyContract(t *testing.T) {
	if os.Getenv("FUFU_LIVE_SALE_CARD_TEST_KEY") != "1" {
		t.Skip("set FUFU_LIVE_SALE_CARD_TEST_KEY=1 to run the live sale-card test-key contract")
	}
	initLiveToolSiteRuntime(t)
	cookie := liveAdminCookie(t)

	plan := liveStringEnv("FUFU_LIVE_SALE_CARD_TEST_PLAN", "fufu-mix-special-55")
	count := liveIntEnv("FUFU_LIVE_SALE_CARD_TEST_COUNT", 1)
	generatedKeys := []string{}
	defer func() {
		liveDeleteKeysBySearch(t, cookie, generatedKeys)
	}()

	req := jsonRequest(t, http.MethodPost, "/api/admin/sale-cards/test-key", map[string]any{
		"plan":  plan,
		"count": count,
	})
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	route(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("live sale-card test-key status=%d error=%q", rec.Code, liveErrorFromRecorder(rec))
	}
	var body struct {
		Generated int      `json:"generated"`
		Game      string   `json:"game"`
		DrawCount int      `json:"drawCount"`
		Keys      []string `json:"keys"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode live sale-card test-key response: %v", err)
	}
	generatedKeys = body.Keys
	if body.Generated != count || len(body.Keys) != count || strings.TrimSpace(body.Game) == "" || body.DrawCount <= 0 {
		t.Fatalf("live test-key response generated=%d keys=%d game=%q drawCount=%d want count=%d", body.Generated, len(body.Keys), body.Game, body.DrawCount, count)
	}
}

type liveMergeStatus struct {
	Status string `json:"status"`
	Error  string `json:"error"`
	Result struct {
		Success bool `json:"success"`
		NewCard struct {
			Key          string `json:"key"`
			RemainQuota  int64  `json:"remain_quota"`
			IntervalUnit int    `json:"interval_unit"`
			Group        string `json:"group"`
		} `json:"newCard"`
		DeleteResults []struct {
			OK bool `json:"ok"`
		} `json:"deleteResults"`
	} `json:"result"`
}

func initLiveToolSiteRuntime(t *testing.T) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := initRuntime(wd); err != nil {
		t.Fatalf("init live tool site runtime: %v", err)
	}
	t.Cleanup(shutdownRuntime)
}

func liveAdminCookie(t *testing.T) *http.Cookie {
	t.Helper()
	loginReq := jsonRequest(t, http.MethodPost, "/api/admin/session", map[string]any{"token": os.Getenv("ADMIN_TOKEN")})
	loginRec := httptest.NewRecorder()
	route(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("live admin login status=%d error=%q", loginRec.Code, liveErrorFromRecorder(loginRec))
	}
	return adminSessionCookieFromRecorder(t, loginRec)
}

func livePollMergeStatus(t *testing.T, cookie *http.Cookie, jobID string) liveMergeStatus {
	t.Helper()
	deadline := time.Now().Add(90 * time.Second)
	for {
		req := httptest.NewRequest(http.MethodGet, "/api/merge-status/"+jobID, nil)
		req.AddCookie(cookie)
		rec := httptest.NewRecorder()
		route(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("live merge status status=%d error=%q", rec.Code, liveErrorFromRecorder(rec))
		}
		var status liveMergeStatus
		if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
			t.Fatalf("decode live merge status: %v", err)
		}
		if status.Status == "done" || status.Status == "error" {
			return status
		}
		if time.Now().After(deadline) {
			t.Fatalf("live merge status timed out at %q", status.Status)
		}
		time.Sleep(time.Second)
	}
}

func liveDeleteKeysBySearch(t *testing.T, cookie *http.Cookie, keys []string) {
	t.Helper()
	if len(keys) == 0 {
		return
	}
	searchReq := jsonRequest(t, http.MethodPost, "/api/search-keys", map[string]any{"keys": keys})
	searchReq.AddCookie(cookie)
	searchRec := httptest.NewRecorder()
	route(searchRec, searchReq)
	if searchRec.Code != http.StatusOK {
		t.Logf("cleanup search status=%d error=%q", searchRec.Code, liveErrorFromRecorder(searchRec))
		return
	}
	var body struct {
		Found []struct {
			ID int `json:"id"`
		} `json:"found"`
	}
	if err := json.Unmarshal(searchRec.Body.Bytes(), &body); err != nil {
		t.Logf("cleanup search decode failed: %v", err)
		return
	}
	for _, item := range body.Found {
		if item.ID <= 0 {
			continue
		}
		req := httptest.NewRequest(http.MethodDelete, "/api/token/"+strconv.Itoa(item.ID), nil)
		req.AddCookie(cookie)
		rec := httptest.NewRecorder()
		route(rec, req)
		if rec.Code != http.StatusOK {
			t.Logf("cleanup delete token id=%d status=%d error=%q", item.ID, rec.Code, liveErrorFromRecorder(rec))
		}
	}
}

func liveErrorFromRecorder(rec *httptest.ResponseRecorder) string {
	var body struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err == nil {
		return body.Error
	}
	return ""
}

func liveStringEnv(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func liveIntEnv(name string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return n
}

func liveFloatEnv(name string, fallback float64) float64 {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	n, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fallback
	}
	return n
}
