package activityapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"fufu/newapi"
	"fufu/tokens"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestGenerateAndUploadSaleCardsCreatesTokensAndUploadsEncryptedMCY(t *testing.T) {
	setMCYCookieForTest(t, "manage_token=test")

	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/token/tokens" || r.URL.Query().Get("tokenCount") != "2" {
			t.Fatalf("unexpected NewAPI request %s %s", r.Method, r.URL.String())
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode NewAPI body: %v", err)
		}
		if name := strings.TrimSpace(fmt.Sprint(body["name"])); !strings.HasPrefix(name, "daily-special-") {
			t.Fatalf("token name = %q, want daily-special-*", name)
		}
		if got, want := int64(body["remain_quota"].(float64)), int64(55_000); got != want {
			t.Fatalf("remain_quota=%d, want %d; body=%#v", got, want, body)
		}
		if got, want := int64(body["interval_quota"].(float64)), int64(55_000); got != want {
			t.Fatalf("interval_quota=%d, want %d; body=%#v", got, want, body)
		}
		if body["group"] != "mix" || int(body["interval_unit"].(float64)) != 9 || body["unlimited_quota"] != false {
			t.Fatalf("unexpected token create body: %#v", body)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data": []any{
				map[string]any{"id": 1, "key": "generated-a", "name": "x", "remain_quota": 55_000, "interval_unit": 9, "status": 1},
				map[string]any{"id": 2, "key": "sk-generated-b", "name": "x", "remain_quota": 55_000, "interval_unit": 9, "status": 1},
			},
		})
	}))
	t.Cleanup(tokenSrv.Close)

	var uploadHits atomic.Int32
	mcySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uploadHits.Add(1)
		if r.Method != http.MethodPost || r.URL.Path != "/plugin/virtual-card-ship/card/add" {
			t.Fatalf("unexpected MCY request %s %s", r.Method, r.URL.String())
		}
		if got := r.Header.Get("Content-Type"); !strings.HasPrefix(got, "text/plain") {
			t.Fatalf("Content-Type=%q, want text/plain", got)
		}
		if r.Header.Get("Cookie") != "manage_token=test" {
			t.Fatalf("Cookie=%q", r.Header.Get("Cookie"))
		}
		secret := r.Header.Get("Secret")
		if len(secret) != 32 || r.Header.Get("Signature") == "" {
			t.Fatalf("missing encrypted MCY headers: Secret=%q Signature=%q", secret, r.Header.Get("Signature"))
		}
		payload := testDecodeMCYRequest(t, r.Body, secret)
		if got, want := int(payload["item_id"].(float64)), 29; got != want {
			t.Fatalf("item_id=%d, want %d; payload=%#v", got, want, payload)
		}
		if got, want := int(payload["sku_id"].(float64)), 66; got != want {
			t.Fatalf("sku_id=%d, want %d; payload=%#v", got, want, payload)
		}
		if got, want := int(payload["unique"].(float64)), 1; got != want {
			t.Fatalf("unique=%d, want %d; payload=%#v", got, want, payload)
		}
		if got, want := payload["card"], "sk-generated-a\nsk-generated-b"; got != want {
			t.Fatalf("card=%q, want %q", got, want)
		}
		if payload["remark"] != "FuFu 55次混合特惠卡" || int(payload["upload_type"].(float64)) != 0 {
			t.Fatalf("unexpected upload payload: %#v", payload)
		}
		testWriteEncryptedMCYResponse(t, w, map[string]any{"code": 200, "msg": "ok"})
	}))
	t.Cleanup(mcySrv.Close)
	t.Setenv("MCY_BASE_URL", mcySrv.URL)
	t.Setenv("MCY_UPLOAD_ENDPOINT", "/plugin/virtual-card-ship/card/add")

	svc := tokens.NewService(newapi.NewClient(newapi.Site{URL: tokenSrv.URL, Token: "test-token", UserID: "1", QuotaUnit: 1000}))
	result, err := generateAndUploadSaleCards(context.Background(), svc, SaleCardPlan{
		ID:            "daily-special",
		Name:          "FuFu 55次混合特惠卡",
		Count:         2,
		Quota:         55,
		Group:         "mix",
		IntervalUnit:  9,
		ItemID:        29,
		SKUID:         66,
		Remark:        "FuFu 55次混合特惠卡",
		Unique:        true,
		TokenNameSlug: "daily-special",
	})

	if err != nil {
		t.Fatalf("generateAndUploadSaleCards error: %v", err)
	}
	if result.Uploaded != 2 || strings.Join(result.Keys, ",") != "sk-generated-a,sk-generated-b" {
		t.Fatalf("result=%#v", result)
	}
	if uploadHits.Load() != 1 {
		t.Fatalf("uploadHits=%d, want 1", uploadHits.Load())
	}
}

func TestGenerateAndUploadSaleCardsRestockTopsUpToTargetByMCYStock(t *testing.T) {
	setMCYCookieForTest(t, "manage_token=test")

	var createHits, stockHits, uploadHits atomic.Int32
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/token/tokens" {
			t.Fatalf("unexpected NewAPI request %s", r.URL.Path)
		}
		createHits.Add(1)
		if got := r.URL.Query().Get("tokenCount"); got != "3" {
			t.Fatalf("tokenCount=%q, want 3 (target 5 - MCY stock 2)", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": []any{
			map[string]any{"id": 1, "key": "restock-a"},
			map[string]any{"id": 2, "key": "restock-b"},
			map[string]any{"id": 3, "key": "restock-c"},
		}})
	}))
	t.Cleanup(tokenSrv.Close)

	mcySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/plugin/virtual-card-ship/card/get":
			// Real shop stock: 2 unsold cards for this item/sku (equal-* → data.total).
			stockHits.Add(1)
			testWriteEncryptedMCYResponse(t, w, map[string]any{"code": 200, "data": map[string]any{"total": 2, "list": []any{}}})
		case "/plugin/virtual-card-ship/card/add":
			uploadHits.Add(1)
			payload := testDecodeMCYRequest(t, r.Body, r.Header.Get("Secret"))
			if got := payload["card"]; got != "sk-restock-a\nsk-restock-b\nsk-restock-c" {
				t.Fatalf("uploaded card=%q", got)
			}
			testWriteEncryptedMCYResponse(t, w, map[string]any{"code": 200, "msg": "ok"})
		default:
			t.Fatalf("unexpected MCY request %s", r.URL.Path)
		}
	}))
	t.Cleanup(mcySrv.Close)
	t.Setenv("MCY_BASE_URL", mcySrv.URL)

	svc := tokens.NewService(newapi.NewClient(newapi.Site{URL: tokenSrv.URL, Token: "test-token", UserID: "1", QuotaUnit: 1000}))
	result, err := generateAndUploadSaleCards(context.Background(), svc, SaleCardPlan{
		ID:            "restock-plan",
		Name:          "Restock plan",
		TargetStock:   5,
		Quota:         55,
		Group:         "mix",
		IntervalUnit:  9,
		ItemID:        29,
		SKUID:         66,
		TokenNameSlug: "restock-plan",
	})

	if err != nil {
		t.Fatalf("restock error: %v", err)
	}
	if result.CurrentStock != 2 || result.TargetStock != 5 || result.ToUpload != 3 || result.Uploaded != 3 {
		t.Fatalf("restock result=%#v, want current=2 target=5 toUpload=3 uploaded=3", result)
	}
	if stockHits.Load() != 1 || createHits.Load() != 1 || uploadHits.Load() != 1 {
		t.Fatalf("stockHits=%d createHits=%d uploadHits=%d, want 1/1/1", stockHits.Load(), createHits.Load(), uploadHits.Load())
	}
}

func TestGenerateAndUploadSaleCardsRestockSkipsWhenMCYStockMeetsTarget(t *testing.T) {
	setMCYCookieForTest(t, "manage_token=test")

	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("no tokens should be created when stock already meets target: %s", r.URL.Path)
	}))
	t.Cleanup(tokenSrv.Close)

	mcySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/plugin/virtual-card-ship/card/get" {
			t.Fatalf("only stock should be queried when nothing needs restocking: %s", r.URL.Path)
		}
		// MCY already holds 9 unsold cards; target 5 → nothing to upload.
		testWriteEncryptedMCYResponse(t, w, map[string]any{"code": 200, "data": map[string]any{"total": 9, "list": []any{}}})
	}))
	t.Cleanup(mcySrv.Close)
	t.Setenv("MCY_BASE_URL", mcySrv.URL)

	svc := tokens.NewService(newapi.NewClient(newapi.Site{URL: tokenSrv.URL, Token: "test-token", UserID: "1", QuotaUnit: 1000}))
	result, err := generateAndUploadSaleCards(context.Background(), svc, SaleCardPlan{
		ID:            "restock-full",
		Name:          "Restock full",
		TargetStock:   5,
		Quota:         55,
		Group:         "mix",
		IntervalUnit:  9,
		ItemID:        29,
		SKUID:         66,
		TokenNameSlug: "restock-full",
	})

	if err != nil {
		t.Fatalf("restock error: %v", err)
	}
	if result.CurrentStock != 9 || result.ToUpload != 0 || result.Uploaded != 0 || len(result.Keys) != 0 {
		t.Fatalf("over-target restock result=%#v, want current=9 toUpload=0 uploaded=0", result)
	}
}

func TestGenerateAndUploadSaleCardsDoesNotUploadWhenNewAPIHasNoKeys(t *testing.T) {
	setMCYCookieForTest(t, "manage_token=test")
	tokenSrv := newSaleCardTokenServer(t, map[string]any{"success": true, "data": []any{}})
	var uploadHits atomic.Int32
	mcySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uploadHits.Add(1)
		t.Fatalf("MCY upload should not be called when NewAPI returns no keys")
	}))
	t.Cleanup(mcySrv.Close)
	t.Setenv("MCY_BASE_URL", mcySrv.URL)

	svc := tokens.NewService(newapi.NewClient(newapi.Site{URL: tokenSrv.URL, Token: "test-token", UserID: "1", QuotaUnit: 1000}))
	result, err := generateAndUploadSaleCards(context.Background(), svc, SaleCardPlan{
		ID:            "missing-keys",
		Name:          "Missing keys",
		Count:         1,
		Quota:         55,
		Group:         "mix",
		IntervalUnit:  9,
		ItemID:        29,
		SKUID:         66,
		TokenNameSlug: "missing-keys",
	})

	if err == nil || !errors.Is(err, ErrSaleCardGenerationFailed) {
		t.Fatalf("result=%#v err=%v, want ErrSaleCardGenerationFailed", result, err)
	}
	if uploadHits.Load() != 0 {
		t.Fatalf("uploadHits=%d, want 0", uploadHits.Load())
	}
}

func TestGenerateAndUploadSaleCardsReportsMCYUploadFailure(t *testing.T) {
	setMCYCookieForTest(t, "manage_token=test")
	tokenSrv := newSaleCardTokenServer(t, map[string]any{"success": true, "data": []any{map[string]any{"id": 1, "key": "generated-a"}}})
	mcySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		testWriteEncryptedMCYResponse(t, w, map[string]any{"code": 500, "msg": "shop rejected upload"})
	}))
	t.Cleanup(mcySrv.Close)
	t.Setenv("MCY_BASE_URL", mcySrv.URL)

	svc := tokens.NewService(newapi.NewClient(newapi.Site{URL: tokenSrv.URL, Token: "test-token", UserID: "1", QuotaUnit: 1000}))
	result, err := generateAndUploadSaleCards(context.Background(), svc, SaleCardPlan{
		ID:            "shop-fail",
		Name:          "Shop fail",
		Count:         1,
		Quota:         55,
		Group:         "mix",
		IntervalUnit:  9,
		ItemID:        29,
		SKUID:         66,
		TokenNameSlug: "shop-fail",
	})

	if err == nil || !errors.Is(err, ErrShopRequestFailed) || !strings.Contains(err.Error(), "shop rejected upload") {
		t.Fatalf("result=%#v err=%v, want MCY upload failure", result, err)
	}
	if len(result.Keys) != 1 || result.Uploaded != 0 {
		t.Fatalf("failed upload should report generated keys but no uploaded count: %#v", result)
	}
}

func TestSaleCardPlanTemplatesIncludeSpecialAndMonthlyCards(t *testing.T) {
	plans := saleCardPlanTemplates()

	special := plans["fufu-mix-special-55"]
	if special.ItemID != 29 || special.SKUID != 66 || special.Quota != 55 || special.IntervalUnit != 9 || special.Group != "mix" {
		t.Fatalf("special plan mismatch: %#v", special)
	}
	monthly100 := plans["fufu-mix-month-100"]
	if monthly100.ItemID != 28 || monthly100.SKUID != 65 || monthly100.Quota != 100 || monthly100.IntervalUnit != 9 || monthly100.Group != "mix" {
		t.Fatalf("monthly 100 plan mismatch: %#v", monthly100)
	}
	monthly1000 := plans["fufu-mix-month-1000"]
	if monthly1000.ItemID != 28 || monthly1000.SKUID != 62 || monthly1000.Quota != 1000 || monthly1000.IntervalUnit != 9 || monthly1000.Group != "mix" {
		t.Fatalf("monthly 1000 plan mismatch: %#v", monthly1000)
	}
}

func TestHandleAdminSaleCardsRunExecutesKnownPlan(t *testing.T) {
	setMCYCookieForTest(t, "manage_token=test")
	t.Setenv("ADMIN_TOKEN", "test-admin-token")

	tokenSrv := newSaleCardTokenServer(t, map[string]any{"success": true, "data": []any{map[string]any{"id": 1, "key": "generated-a"}}})
	oldTokenSvc := tokenSvc
	oldTokenConfigErr := tokenConfigErr
	t.Cleanup(func() {
		tokenSvc = oldTokenSvc
		tokenConfigErr = oldTokenConfigErr
	})
	tokenConfigErr = nil
	tokenSvc = tokens.NewService(newapi.NewClient(newapi.Site{URL: tokenSrv.URL, Token: "test-token", UserID: "1", QuotaUnit: 1000}))

	mcySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload := testDecodeMCYRequest(t, r.Body, r.Header.Get("Secret"))
		if got := payload["card"]; got != "sk-generated-a" {
			t.Fatalf("uploaded card=%q", got)
		}
		testWriteEncryptedMCYResponse(t, w, map[string]any{"code": 200, "msg": "ok"})
	}))
	t.Cleanup(mcySrv.Close)
	t.Setenv("MCY_BASE_URL", mcySrv.URL)

	req := httptest.NewRequest(http.MethodPost, "/api/admin/sale-cards/run", strings.NewReader(`{"plan":"fufu-mix-special-55","count":1}`))
	req.Header.Set("Authorization", "Bearer test-admin-token")
	w := httptest.NewRecorder()

	apiRoute(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	var body SaleCardListingResult
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, w.Body.String())
	}
	if body.Uploaded != 1 || strings.Join(body.Keys, ",") != "sk-generated-a" || body.PlanID != "fufu-mix-special-55" {
		t.Fatalf("response=%#v", body)
	}
}

func newSaleCardTokenServer(t *testing.T, payload map[string]any) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/token/tokens" {
			t.Fatalf("unexpected NewAPI request %s %s", r.Method, r.URL.String())
		}
		_ = json.NewEncoder(w).Encode(payload)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func setMCYCookieForTest(t *testing.T, value string) {
	t.Helper()
	old := getMCYCookie()
	setMCYCookie(value)
	t.Cleanup(func() { setMCYCookie(old) })
}
