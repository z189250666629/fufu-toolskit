package activityapp

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"fufu/newapi"
	"fufu/tokens"
)

func TestNewAPICreditQuotaAdapterCreatesPoolPlanAndBindsUserSubscription(t *testing.T) {
	setupScratchLockTestDB(t)

	var mu sync.Mutex
	var planCreateBody map[string]any
	var planUpdateBody map[string]any
	var bindBody map[string]any
	planCreated := false
	rewardSubID := int64(0)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/subscription/admin/users/999/subscriptions":
			mu.Lock()
			subID := rewardSubID
			mu.Unlock()
			items := []any{
				map[string]any{"subscription": map[string]any{
					"id":           321,
					"user_id":      999,
					"plan_id":      11,
					"amount_total": 50000000,
					"amount_used":  0,
					"start_time":   time.Now().Add(-time.Hour).Unix(),
					"end_time":     time.Now().Add(48 * time.Hour).Unix(),
					"status":       "active",
				}},
			}
			if subID > 0 {
				items = append(items, map[string]any{"subscription": map[string]any{
					"id":           subID,
					"user_id":      999,
					"plan_id":      77,
					"amount_total": 5000000,
					"amount_used":  0,
					"start_time":   time.Now().Unix(),
					"end_time":     time.Now().Add(30 * 24 * time.Hour).Unix(),
					"status":       "active",
				}})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": items})
		case r.Method == http.MethodGet && r.URL.Path == "/api/subscription/admin/plans":
			mu.Lock()
			created := planCreated
			mu.Unlock()
			items := []any{
				map[string]any{"plan": map[string]any{
					"id":             11,
					"title":          "normal-plan",
					"total_amount":   50000000,
					"duration_unit":  "month",
					"duration_value": 1,
				}},
			}
			if created {
				items = append(items, map[string]any{"plan": map[string]any{
					"id":             77,
					"title":          rewardPlanPoolTitle(1),
					"total_amount":   5000000,
					"duration_unit":  "month",
					"duration_value": 1,
					"enabled":        false,
				}})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": items})
		case r.Method == http.MethodPost && r.URL.Path == "/api/subscription/admin/plans":
			if err := json.NewDecoder(r.Body).Decode(&planCreateBody); err != nil {
				t.Fatalf("decode plan create body: %v", err)
			}
			mu.Lock()
			planCreated = true
			mu.Unlock()
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]any{"id": 77}})
		case r.Method == http.MethodPut && r.URL.Path == "/api/subscription/admin/plans/77":
			if err := json.NewDecoder(r.Body).Decode(&planUpdateBody); err != nil {
				t.Fatalf("decode plan update body: %v", err)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
		case r.Method == http.MethodPost && r.URL.Path == "/api/subscription/admin/users/999/subscriptions":
			if err := json.NewDecoder(r.Body).Decode(&bindBody); err != nil {
				t.Fatalf("decode bind body: %v", err)
			}
			mu.Lock()
			rewardSubID = 901
			mu.Unlock()
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)

	adapter := newAPICreditQuotaAdapter{
		subscriptionClient:    newapi.NewClient(newapi.Site{URL: server.URL, Token: "token", UserID: "1"}),
		subscriptionQuotaUnit: 500000,
	}
	card := Card{
		CardKey:        "actsub-test-card",
		Source:         "subscription",
		SubscriptionID: sql.NullInt64{Int64: 321, Valid: true},
		UserID:         sql.NullInt64{Int64: 999, Valid: true},
	}

	if err := adapter.AddQuota(card, 10); err != nil {
		t.Fatalf("AddQuota: %v", err)
	}
	if planCreateBody == nil || planUpdateBody == nil || bindBody == nil {
		t.Fatalf("create=%v update=%v bind=%v", planCreateBody != nil, planUpdateBody != nil, bindBody != nil)
	}
	assertRewardPlanPayload(t, planCreateBody["plan"], rewardPlanPoolTitle(1), 5000000, "month", 1, 0)
	assertRewardPlanPayload(t, planUpdateBody["plan"], rewardPlanPoolTitle(1), 5000000, "month", 1, 0)
	if got := int64(bindBody["plan_id"].(float64)); got != 77 {
		t.Fatalf("plan_id=%d", got)
	}

	var rewardPlanID, rewardSubscriptionID int64
	if err := db.QueryRow(`SELECT reward_plan_id,reward_subscription_id FROM reward_issuance WHERE card_key=?`, card.CardKey).Scan(&rewardPlanID, &rewardSubscriptionID); err != nil {
		t.Fatalf("reward issuance record: %v", err)
	}
	if rewardPlanID != 77 || rewardSubscriptionID != 901 {
		t.Fatalf("reward issuance=(%d,%d), want (77,901)", rewardPlanID, rewardSubscriptionID)
	}
}

func TestNewAPICreditQuotaAdapterRequiresSubscriptionIDsForRewardFlow(t *testing.T) {
	adapter := newAPICreditQuotaAdapter{
		subscriptionClient: newapi.NewClient(newapi.Site{URL: "https://example.test", Token: "token", UserID: "1"}),
	}
	card := Card{
		CardKey: "actsub-test-card",
		Source:  "subscription",
		UserID:  sql.NullInt64{Int64: 123, Valid: true},
	}

	err := adapter.AddQuota(card, 10)
	if err == nil || err.Error() != "subscription id is missing" {
		t.Fatalf("err=%v", err)
	}
}

func TestNewAPICreditQuotaAdapterClosesTokenIdleConnectionsAfterQuotaAttempt(t *testing.T) {
	transport := &closeTrackingCreditTransport{}
	client := newapi.NewClient(newapi.Site{URL: "https://newapi.example.test", Token: "token", UserID: "1"})
	client.HTTPClient = &http.Client{Transport: transport}
	adapter := newAPICreditQuotaAdapter{
		service: tokens.NewService(client),
	}
	card := Card{CardKey: "sk-credit-close-123456"}

	if err := adapter.AddQuota(card, 10); err != nil {
		t.Fatalf("AddQuota: %v", err)
	}
	if got := transport.closeCount(); got == 0 {
		t.Fatal("credit quota adapter should close idle NewAPI connections after the attempt")
	}
	if got := transport.missingCloseCount(); got != 0 {
		t.Fatalf("credit quota NewAPI requests should use Connection: close, missing=%d", got)
	}
}

func TestNewAPICreditQuotaAdapterFallsBackAfterBindTimeout(t *testing.T) {
	setupScratchLockTestDB(t)

	oldTimeout := creditQuotaTimeout
	oldRequestTimeout := subscriptionRewardRequestTimeout
	oldRetryDelay := subscriptionRewardRetryDelay
	oldAttempts := subscriptionRewardRetryAttempts
	oldCooldown := subscriptionRewardBindRetryCooldown
	creditQuotaTimeout = 200 * time.Millisecond
	subscriptionRewardRequestTimeout = 30 * time.Millisecond
	subscriptionRewardRetryDelay = 5 * time.Millisecond
	subscriptionRewardRetryAttempts = 1
	subscriptionRewardBindRetryCooldown = 50 * time.Millisecond
	t.Cleanup(func() {
		creditQuotaTimeout = oldTimeout
		subscriptionRewardRequestTimeout = oldRequestTimeout
		subscriptionRewardRetryDelay = oldRetryDelay
		subscriptionRewardRetryAttempts = oldAttempts
		subscriptionRewardBindRetryCooldown = oldCooldown
	})

	var mu sync.Mutex
	rewardBound := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/subscription/admin/plans":
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": []any{
				map[string]any{"plan": map[string]any{"id": 11, "title": "normal-plan", "total_amount": 50000000, "duration_unit": "day", "duration_value": 2}},
				map[string]any{"plan": map[string]any{"id": 77, "title": rewardPlanPoolTitle(1), "total_amount": 5000000, "duration_unit": "day", "duration_value": 2, "enabled": false}},
			}})
		case r.Method == http.MethodPut && r.URL.Path == "/api/subscription/admin/plans/77":
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
		case r.Method == http.MethodGet && r.URL.Path == "/api/subscription/admin/users/999/subscriptions":
			mu.Lock()
			bound := rewardBound
			mu.Unlock()
			items := []any{
				map[string]any{"subscription": map[string]any{
					"id":           321,
					"user_id":      999,
					"plan_id":      11,
					"amount_total": 50000000,
					"amount_used":  0,
					"start_time":   time.Now().Add(-time.Hour).Unix(),
					"end_time":     time.Now().Add(48 * time.Hour).Unix(),
					"status":       "active",
				}},
			}
			if bound {
				items = append(items, map[string]any{"subscription": map[string]any{
					"id":           901,
					"user_id":      999,
					"plan_id":      77,
					"amount_total": 5000000,
					"amount_used":  0,
					"start_time":   time.Now().Unix(),
					"end_time":     time.Now().Add(24 * time.Hour).Unix(),
					"status":       "active",
				}})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": items})
		case r.Method == http.MethodPost && r.URL.Path == "/api/subscription/admin/users/999/subscriptions":
			mu.Lock()
			rewardBound = true
			mu.Unlock()
			time.Sleep(100 * time.Millisecond)
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)

	client := newapi.NewClient(newapi.Site{URL: server.URL, Token: "token", UserID: "1"})
	client.HTTPClient.Timeout = 0
	adapter := newAPICreditQuotaAdapter{
		subscriptionClient:    client,
		subscriptionQuotaUnit: 500000,
	}
	card := Card{
		CardKey:        "actsub-timeout",
		Source:         "subscription",
		SubscriptionID: sql.NullInt64{Int64: 321, Valid: true},
		UserID:         sql.NullInt64{Int64: 999, Valid: true},
	}

	if err := adapter.AddQuota(card, 10); err != nil {
		t.Fatalf("AddQuota should treat timed-out bind with observed active reward sub as success: %v", err)
	}
	var rewardSubscriptionID int64
	if err := db.QueryRow(`SELECT reward_subscription_id FROM reward_issuance WHERE card_key=?`, card.CardKey).Scan(&rewardSubscriptionID); err != nil {
		t.Fatalf("reward issuance record: %v", err)
	}
	if rewardSubscriptionID != 901 {
		t.Fatalf("reward_subscription_id=%d want 901", rewardSubscriptionID)
	}
}

func TestNewAPICreditQuotaAdapterReconfiguresExistingPoolPlanBeforeBinding(t *testing.T) {
	setupScratchLockTestDB(t)

	var planUpdateBody map[string]any
	var bindBody map[string]any
	var mu sync.Mutex
	rewardSubID := int64(0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/subscription/admin/users/999/subscriptions":
			mu.Lock()
			subID := rewardSubID
			mu.Unlock()
			items := []any{
				map[string]any{"subscription": map[string]any{
					"id":           321,
					"user_id":      999,
					"plan_id":      11,
					"amount_total": 50000000,
					"amount_used":  0,
					"start_time":   time.Now().Add(-time.Hour).Unix(),
					"end_time":     time.Now().Add(48 * time.Hour).Unix(),
					"status":       "active",
				}},
			}
			if subID > 0 {
				items = append(items, map[string]any{"subscription": map[string]any{
					"id":           subID,
					"user_id":      999,
					"plan_id":      77,
					"amount_total": 5000000,
					"amount_used":  0,
					"start_time":   time.Now().Unix(),
					"end_time":     time.Now().Add(30 * 24 * time.Hour).Unix(),
					"status":       "active",
				}})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": items})
		case r.Method == http.MethodGet && r.URL.Path == "/api/subscription/admin/plans":
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": []any{
				map[string]any{"plan": map[string]any{
					"id":             11,
					"title":          "normal-plan",
					"total_amount":   50000000,
					"duration_unit":  "month",
					"duration_value": 1,
				}},
				map[string]any{"plan": map[string]any{
					"id":             77,
					"title":          rewardPlanPoolTitle(1),
					"total_amount":   123,
					"duration_unit":  "day",
					"duration_value": 99,
					"enabled":        true,
				}},
			}})
		case r.Method == http.MethodPut && r.URL.Path == "/api/subscription/admin/plans/77":
			if err := json.NewDecoder(r.Body).Decode(&planUpdateBody); err != nil {
				t.Fatalf("decode plan update body: %v", err)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
		case r.Method == http.MethodPost && r.URL.Path == "/api/subscription/admin/users/999/subscriptions":
			if err := json.NewDecoder(r.Body).Decode(&bindBody); err != nil {
				t.Fatalf("decode bind body: %v", err)
			}
			mu.Lock()
			rewardSubID = 901
			mu.Unlock()
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)

	adapter := newAPICreditQuotaAdapter{
		subscriptionClient:    newapi.NewClient(newapi.Site{URL: server.URL, Token: "token", UserID: "1"}),
		subscriptionQuotaUnit: 500000,
	}
	card := Card{
		CardKey:        "actsub-reuse-plan",
		Source:         "subscription",
		SubscriptionID: sql.NullInt64{Int64: 321, Valid: true},
		UserID:         sql.NullInt64{Int64: 999, Valid: true},
	}

	if err := adapter.AddQuota(card, 10); err != nil {
		t.Fatalf("AddQuota: %v", err)
	}
	if planUpdateBody == nil || bindBody == nil {
		t.Fatalf("update=%v bind=%v", planUpdateBody != nil, bindBody != nil)
	}
	assertRewardPlanPayload(t, planUpdateBody["plan"], rewardPlanPoolTitle(1), 5000000, "month", 1, 0)
	if got := int64(bindBody["plan_id"].(float64)); got != 77 {
		t.Fatalf("plan_id=%d", got)
	}
}

func TestNewAPICreditQuotaAdapterReusesPoolPlanAcrossSequentialWinners(t *testing.T) {
	setupScratchLockTestDB(t)

	var mu sync.Mutex
	createCount := 0
	bindCount := 0
	nextRewardSubID := int64(900)
	rewardPlanCreated := false
	rewardSubsByUser := map[int64][]int64{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/subscription/admin/users/") && strings.HasSuffix(r.URL.Path, "/subscriptions"):
			var userID int64
			if _, err := fmt.Sscanf(r.URL.Path, "/api/subscription/admin/users/%d/subscriptions", &userID); err != nil {
				t.Fatalf("parse user path %q: %v", r.URL.Path, err)
			}
			mu.Lock()
			rewardSubs := append([]int64(nil), rewardSubsByUser[userID]...)
			mu.Unlock()
			items := []any{
				map[string]any{"subscription": map[string]any{
					"id":           400 + userID,
					"user_id":      userID,
					"plan_id":      11,
					"amount_total": 50000000,
					"amount_used":  0,
					"start_time":   time.Now().Add(-time.Hour).Unix(),
					"end_time":     time.Now().Add(48 * time.Hour).Unix(),
					"status":       "active",
				}},
			}
			for _, subID := range rewardSubs {
				items = append(items, map[string]any{"subscription": map[string]any{
					"id":           subID,
					"user_id":      userID,
					"plan_id":      77,
					"amount_total": 5000000,
					"amount_used":  0,
					"start_time":   time.Now().Unix(),
					"end_time":     time.Now().Add(30 * 24 * time.Hour).Unix(),
					"status":       "active",
				}})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": items})
		case r.Method == http.MethodGet && r.URL.Path == "/api/subscription/admin/plans":
			mu.Lock()
			created := rewardPlanCreated
			mu.Unlock()
			items := []any{
				map[string]any{"plan": map[string]any{
					"id":             11,
					"title":          "normal-plan",
					"total_amount":   50000000,
					"duration_unit":  "month",
					"duration_value": 1,
				}},
			}
			if created {
				items = append(items, map[string]any{"plan": map[string]any{
					"id":             77,
					"title":          rewardPlanPoolTitle(1),
					"total_amount":   5000000,
					"duration_unit":  "month",
					"duration_value": 1,
					"enabled":        false,
				}})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": items})
		case r.Method == http.MethodPost && r.URL.Path == "/api/subscription/admin/plans":
			createCount++
			mu.Lock()
			rewardPlanCreated = true
			mu.Unlock()
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]any{"id": 77}})
		case r.Method == http.MethodPut && r.URL.Path == "/api/subscription/admin/plans/77":
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/api/subscription/admin/users/") && strings.HasSuffix(r.URL.Path, "/subscriptions"):
			var userID int64
			if _, err := fmt.Sscanf(r.URL.Path, "/api/subscription/admin/users/%d/subscriptions", &userID); err != nil {
				t.Fatalf("parse bind path %q: %v", r.URL.Path, err)
			}
			bindCount++
			mu.Lock()
			nextRewardSubID++
			rewardSubsByUser[userID] = append(rewardSubsByUser[userID], nextRewardSubID)
			mu.Unlock()
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)

	adapter := newAPICreditQuotaAdapter{
		subscriptionClient:    newapi.NewClient(newapi.Site{URL: server.URL, Token: "token", UserID: "1"}),
		subscriptionQuotaUnit: 500000,
	}
	first := Card{
		CardKey:        "actsub-user1",
		Source:         "subscription",
		SubscriptionID: sql.NullInt64{Int64: 401, Valid: true},
		UserID:         sql.NullInt64{Int64: 1, Valid: true},
	}
	second := Card{
		CardKey:        "actsub-user2",
		Source:         "subscription",
		SubscriptionID: sql.NullInt64{Int64: 402, Valid: true},
		UserID:         sql.NullInt64{Int64: 2, Valid: true},
	}

	if err := adapter.AddQuota(first, 10); err != nil {
		t.Fatalf("first AddQuota: %v", err)
	}
	if err := adapter.AddQuota(second, 10); err != nil {
		t.Fatalf("second AddQuota: %v", err)
	}
	if createCount != 1 {
		t.Fatalf("createCount=%d want 1", createCount)
	}
	if bindCount != 2 {
		t.Fatalf("bindCount=%d want 2", bindCount)
	}
}

func TestNewAPICreditQuotaAdapterHoldsBusySlotAfterTimeoutAndUsesNextPoolSlot(t *testing.T) {
	setupScratchLockTestDB(t)

	oldRequestTimeout := subscriptionRewardRequestTimeout
	oldRetryDelay := subscriptionRewardRetryDelay
	oldAttempts := subscriptionRewardRetryAttempts
	oldCooldown := subscriptionRewardBindRetryCooldown
	subscriptionRewardRequestTimeout = 30 * time.Millisecond
	subscriptionRewardRetryDelay = 5 * time.Millisecond
	subscriptionRewardRetryAttempts = 1
	subscriptionRewardBindRetryCooldown = time.Minute
	t.Cleanup(func() {
		subscriptionRewardRequestTimeout = oldRequestTimeout
		subscriptionRewardRetryDelay = oldRetryDelay
		subscriptionRewardRetryAttempts = oldAttempts
		subscriptionRewardBindRetryCooldown = oldCooldown
	})

	var mu sync.Mutex
	createBodies := []map[string]any{}
	createdPlans := map[int64]string{}
	nextPlanID := int64(70)
	bindCanceled := make(chan struct{})
	var bindCanceledOnce sync.Once
	rewardSubsByUser := map[int64][]int64{
		2: {902},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/subscription/admin/users/") && strings.HasSuffix(r.URL.Path, "/subscriptions"):
			var userID int64
			if _, err := fmt.Sscanf(r.URL.Path, "/api/subscription/admin/users/%d/subscriptions", &userID); err != nil {
				t.Fatalf("parse user path %q: %v", r.URL.Path, err)
			}
			mu.Lock()
			rewardSubs := append([]int64(nil), rewardSubsByUser[userID]...)
			plans := map[int64]string{}
			for id, title := range createdPlans {
				plans[id] = title
			}
			mu.Unlock()
			sourcePlanID := int64(11)
			rewardPlanID := int64(0)
			for id, title := range plans {
				if title == rewardPlanPoolTitle(2) {
					rewardPlanID = id
				}
			}
			items := []any{
				map[string]any{"subscription": map[string]any{
					"id":           500 + userID,
					"user_id":      userID,
					"plan_id":      sourcePlanID,
					"amount_total": 50000000,
					"amount_used":  0,
					"start_time":   time.Now().Add(-time.Hour).Unix(),
					"end_time":     time.Now().Add(48 * time.Hour).Unix(),
					"status":       "active",
				}},
			}
			for _, subID := range rewardSubs {
				if rewardPlanID == 0 {
					continue
				}
				items = append(items, map[string]any{"subscription": map[string]any{
					"id":           subID,
					"user_id":      userID,
					"plan_id":      rewardPlanID,
					"amount_total": 5000000,
					"amount_used":  0,
					"start_time":   time.Now().Unix(),
					"end_time":     time.Now().Add(30 * 24 * time.Hour).Unix(),
					"status":       "active",
				}})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": items})
		case r.Method == http.MethodGet && r.URL.Path == "/api/subscription/admin/plans":
			mu.Lock()
			plans := map[int64]string{}
			for id, title := range createdPlans {
				plans[id] = title
			}
			mu.Unlock()
			items := []any{
				map[string]any{"plan": map[string]any{
					"id":             11,
					"title":          "normal-plan",
					"total_amount":   50000000,
					"duration_unit":  "month",
					"duration_value": 1,
				}},
			}
			for id, title := range plans {
				items = append(items, map[string]any{"plan": map[string]any{
					"id":             id,
					"title":          title,
					"total_amount":   5000000,
					"duration_unit":  "month",
					"duration_value": 1,
					"enabled":        false,
				}})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": items})
		case r.Method == http.MethodPost && r.URL.Path == "/api/subscription/admin/plans":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode plan create body: %v", err)
			}
			mu.Lock()
			nextPlanID++
			planID := nextPlanID
			createBodies = append(createBodies, body)
			createdPlans[planID] = body["plan"].(map[string]any)["title"].(string)
			mu.Unlock()
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]any{"id": planID}})
		case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/api/subscription/admin/plans/"):
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/api/subscription/admin/users/"):
			var userID int64
			if _, err := fmt.Sscanf(r.URL.Path, "/api/subscription/admin/users/%d/subscriptions", &userID); err != nil {
				t.Fatalf("parse bind path %q: %v", r.URL.Path, err)
			}
			if userID == 1 {
				_, _ = io.ReadAll(r.Body)
				_ = r.Body.Close()
				select {
				case <-r.Context().Done():
					bindCanceledOnce.Do(func() { close(bindCanceled) })
					return
				case <-time.After(100 * time.Millisecond):
					_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
				}
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)

	client := newapi.NewClient(newapi.Site{URL: server.URL, Token: "token", UserID: "1"})
	client.HTTPClient.Timeout = 0
	adapter := newAPICreditQuotaAdapter{
		subscriptionClient:    client,
		subscriptionQuotaUnit: 500000,
	}
	first := Card{
		CardKey:        "actsub-user-timeout",
		Source:         "subscription",
		SubscriptionID: sql.NullInt64{Int64: 501, Valid: true},
		UserID:         sql.NullInt64{Int64: 1, Valid: true},
	}
	second := Card{
		CardKey:        "actsub-user-success",
		Source:         "subscription",
		SubscriptionID: sql.NullInt64{Int64: 502, Valid: true},
		UserID:         sql.NullInt64{Int64: 2, Valid: true},
	}

	if err := adapter.AddQuota(first, 10); err == nil {
		t.Fatal("first AddQuota should time out and keep its pool slot leased")
	}
	select {
	case <-bindCanceled:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed-out subscription bind request was not canceled")
	}
	var firstSlot int
	var firstState string
	var firstNextBindAt int64
	if err := db.QueryRow(`SELECT slot,state,next_bind_at FROM reward_plan_pool WHERE lease_card_key=?`, first.CardKey).Scan(&firstSlot, &firstState, &firstNextBindAt); err != nil {
		t.Fatalf("lookup first lease: %v", err)
	}
	if firstSlot != 1 || firstState != rewardPlanLeaseStateLeased || firstNextBindAt <= time.Now().Unix() {
		t.Fatalf("first lease=(slot=%d state=%q next=%d)", firstSlot, firstState, firstNextBindAt)
	}

	if err := adapter.AddQuota(second, 10); err != nil {
		t.Fatalf("second AddQuota: %v", err)
	}
	if len(createBodies) != 2 {
		t.Fatalf("created plan count=%d want 2", len(createBodies))
	}
	firstPlanTitle := createBodies[0]["plan"].(map[string]any)["title"].(string)
	secondPlanTitle := createBodies[1]["plan"].(map[string]any)["title"].(string)
	if firstPlanTitle != rewardPlanPoolTitle(1) || secondPlanTitle != rewardPlanPoolTitle(2) {
		t.Fatalf("created titles=(%q,%q)", firstPlanTitle, secondPlanTitle)
	}
}

func TestRewardPlanDurationFromPlanPrefersFriendlyUnits(t *testing.T) {
	t.Run("year maps to months", func(t *testing.T) {
		got := rewardPlanDurationFromPlan(subscriptionPlanSummary{
			DurationUnit:  "year",
			DurationValue: 1,
		})
		if got.Unit != "month" || got.Value != 12 || got.CustomSeconds != 0 {
			t.Fatalf("got=%+v", got)
		}
	})

	t.Run("hours map to days when possible", func(t *testing.T) {
		got := rewardPlanDurationFromPlan(subscriptionPlanSummary{
			DurationUnit:  "hour",
			DurationValue: 24 * 7,
		})
		if got.Unit != "day" || got.Value != 7 || got.CustomSeconds != 0 {
			t.Fatalf("got=%+v", got)
		}
	})

	t.Run("custom whole days map to day unit", func(t *testing.T) {
		got := rewardPlanDurationFromPlan(subscriptionPlanSummary{
			DurationUnit:  "custom",
			CustomSeconds: int64(14 * 24 * time.Hour / time.Second),
		})
		if got.Unit != "day" || got.Value != 14 || got.CustomSeconds != 0 {
			t.Fatalf("got=%+v", got)
		}
	})
}

func assertRewardPlanPayload(t *testing.T, raw any, title string, totalAmount int64, durationUnit string, durationValue, customSeconds int64) {
	t.Helper()
	plan, ok := raw.(map[string]any)
	if !ok || plan == nil {
		t.Fatalf("plan payload=%#v", raw)
	}
	if got := plan["title"].(string); got != title {
		t.Fatalf("title=%q want %q", got, title)
	}
	if got := int64(plan["total_amount"].(float64)); got != totalAmount {
		t.Fatalf("total_amount=%d want %d", got, totalAmount)
	}
	if got := plan["duration_unit"].(string); got != durationUnit {
		t.Fatalf("duration_unit=%q want %q", got, durationUnit)
	}
	if got := int64(plan["duration_value"].(float64)); got != durationValue {
		t.Fatalf("duration_value=%d want %d", got, durationValue)
	}
	if got := int64(plan["custom_seconds"].(float64)); got != customSeconds {
		t.Fatalf("custom_seconds=%d want %d", got, customSeconds)
	}
	if got, _ := plan["enabled"].(bool); got {
		t.Fatalf("enabled=%v", got)
	}
}

type closeTrackingCreditTransport struct {
	mu         sync.Mutex
	closeCalls int
	missing    int
}

func (t *closeTrackingCreditTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if !req.Close || !strings.EqualFold(req.Header.Get("Connection"), "close") {
		t.mu.Lock()
		t.missing++
		t.mu.Unlock()
	}
	body := `{"success":true}`
	switch {
	case req.Method == http.MethodGet && req.URL.Path == "/api/token/search":
		body = `{"success":true,"data":[{"id":7,"key":"sk-credit-close-123456","name":"credit-card","remain_quota":10,"status":1}]}`
	case req.Method == http.MethodPut && req.URL.Path == "/api/token/":
		body = `{"success":true}`
	default:
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Body:       io.NopCloser(strings.NewReader(`{"success":false,"message":"unexpected request"}`)),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
		Request:    req,
	}, nil
}

func (t *closeTrackingCreditTransport) CloseIdleConnections() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.closeCalls++
}

func (t *closeTrackingCreditTransport) closeCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.closeCalls
}

func (t *closeTrackingCreditTransport) missingCloseCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.missing
}
