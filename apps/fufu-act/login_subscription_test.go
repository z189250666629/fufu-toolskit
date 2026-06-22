package activityapp

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"fufu/newapi"
)

type subscriptionRuntimeTestConfig struct {
	UserID     int64
	Username   string
	Subs       []subscriptionSummary
	PlanTitles map[int64]string
	OnMutation func(string, map[string]any)
	Handler    func(http.ResponseWriter, *http.Request) bool
}

func useSubscriptionRuntimeServer(t *testing.T, userID int64, username string, subs []subscriptionSummary, onMutation func(string, map[string]any)) {
	useSubscriptionRuntimeServerWithConfig(t, subscriptionRuntimeTestConfig{
		UserID:     userID,
		Username:   username,
		Subs:       subs,
		OnMutation: onMutation,
	})
}

func useSubscriptionRuntimeServerWithConfig(t *testing.T, cfg subscriptionRuntimeTestConfig) {
	t.Helper()
	userID := cfg.UserID
	username := cfg.Username
	subs := cfg.Subs
	planTitles := map[int64]string{}
	for _, sub := range subs {
		if _, ok := planTitles[sub.PlanID]; !ok {
			planTitles[sub.PlanID] = "plan-" + itoa64(sub.PlanID)
		}
	}
	for id, title := range cfg.PlanTitles {
		planTitles[id] = title
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if cfg.Handler != nil && cfg.Handler(w, r) {
			return
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/user/"+itoa64(userID):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"data": map[string]any{
					"id":       userID,
					"username": username,
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/subscription/admin/users/"+itoa64(userID)+"/subscriptions":
			items := make([]map[string]any, 0, len(subs))
			for _, sub := range subs {
				items = append(items, map[string]any{
					"subscription": map[string]any{
						"id":           sub.ID,
						"user_id":      sub.UserID,
						"plan_id":      sub.PlanID,
						"amount_total": sub.AmountTotal,
						"amount_used":  sub.AmountUsed,
						"start_time":   sub.StartTime,
						"end_time":     sub.EndTime,
						"status":       sub.Status,
					},
				})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": items})
		case r.Method == http.MethodGet && r.URL.Path == "/api/subscription/admin/plans":
			items := make([]map[string]any, 0, len(planTitles))
			for planID, title := range planTitles {
				items = append(items, map[string]any{
					"plan": map[string]any{
						"id":           planID,
						"title":        title,
						"total_amount": newapi.DefaultQuotaUnit * 100,
					},
				})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": items})
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/api/subscription/admin/user_subscriptions/"):
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode subscription mutation body: %v", err)
			}
			if cfg.OnMutation != nil {
				cfg.OnMutation(r.URL.Path, body)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
		default:
			t.Fatalf("unexpected subscription request %s %s", r.Method, r.URL.String())
		}
	}))
	t.Cleanup(server.Close)

	oldSite, oldClient, oldErr := snapshotSubscriptionRuntime()
	setSubscriptionRuntime(newapi.Site{URL: server.URL, Token: "token", UserID: "1", QuotaUnit: newapi.DefaultQuotaUnit}, nil)
	t.Cleanup(func() {
		if oldClient == nil && oldErr == nil && oldSite == (newapi.Site{}) {
			setSubscriptionRuntime(newapi.Site{}, nil)
			return
		}
		setSubscriptionRuntime(oldSite, oldErr)
	})
}

func parseLoginResponse(t *testing.T, body string) map[string]any {
	t.Helper()
	var data map[string]any
	if err := json.Unmarshal([]byte(body), &data); err != nil {
		t.Fatalf("decode response body %q: %v", body, err)
	}
	return data
}

func itoa64(value int64) string {
	return strconv.FormatInt(value, 10)
}

func TestHandleLoginWithSubscriptionIdentityCreatesAndReusesOpaqueCard(t *testing.T) {
	setupScratchLockTestDB(t)
	useSubscriptionRuntimeServer(t, 123, "alice", []subscriptionSummary{{
		ID:          901,
		UserID:      123,
		PlanID:      1,
		AmountTotal: newapi.DefaultQuotaUnit * 100,
		StartTime:   actStartTS + 60,
		EndTime:     actEndTS + 3600,
		Status:      "active",
	}}, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"userId":123,"username":"alice"}`))
	w := httptest.NewRecorder()
	handleLogin(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	first := parseLoginResponse(t, w.Body.String())
	cardKey := strings.TrimSpace(first["cardKey"].(string))
	if !strings.HasPrefix(cardKey, "actsub-") {
		t.Fatalf("cardKey=%q, want opaque subscription key", cardKey)
	}

	card, ok, err := lookupCardBySubscriptionID(901)
	if err != nil || !ok {
		t.Fatalf("lookup subscription card ok=%v err=%v", ok, err)
	}
	if card.CardKey != cardKey || !card.UserID.Valid || card.UserID.Int64 != 123 || !card.Username.Valid || card.Username.String != "alice" {
		t.Fatalf("stored card=%#v", card)
	}

	req2 := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"userId":123,"username":"alice"}`))
	w2 := httptest.NewRecorder()
	handleLogin(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", w2.Code, w2.Body.String())
	}
	second := parseLoginResponse(t, w2.Body.String())
	if second["cardKey"] != cardKey {
		t.Fatalf("second cardKey=%v want %q", second["cardKey"], cardKey)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM cards WHERE subscription_id=?`, 901).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("cards for subscription 901 = %d, want 1", count)
	}
}

func TestHandleLoginWithSubscriptionIdentityRejectsUsernameMismatch(t *testing.T) {
	setupScratchLockTestDB(t)
	useSubscriptionRuntimeServer(t, 123, "alice", nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"userId":123,"username":"bob"}`))
	w := httptest.NewRecorder()
	handleLogin(w, req)

	if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "不匹配") {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestHandleLoginWithSubscriptionIdentityRejectsMissingEligibleSubscription(t *testing.T) {
	setupScratchLockTestDB(t)
	useSubscriptionRuntimeServer(t, 123, "alice", []subscriptionSummary{{
		ID:          901,
		UserID:      123,
		PlanID:      1,
		AmountTotal: newapi.DefaultQuotaUnit * 100,
		StartTime:   actStartTS - 60,
		EndTime:     actEndTS + 3600,
		Status:      "active",
	}}, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"userId":123,"username":"alice"}`))
	w := httptest.NewRecorder()
	handleLogin(w, req)

	if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "有效订阅") {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestHandleLoginWithSubscriptionIdentityExcludesRewardPlanTitle(t *testing.T) {
	setupScratchLockTestDB(t)
	useSubscriptionRuntimeServerWithConfig(t, subscriptionRuntimeTestConfig{
		UserID:   123,
		Username: "alice",
		Subs: []subscriptionSummary{{
			ID:          901,
			UserID:      123,
			PlanID:      7,
			AmountTotal: newapi.DefaultQuotaUnit * 100,
			StartTime:   actStartTS + 60,
			EndTime:     actEndTS + 3600,
			Status:      "active",
		}},
		PlanTitles: map[int64]string{
			7: rewardPlanTitlePrefix + "sub901:q5000000:kdeadbeef",
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"userId":123,"username":"alice"}`))
	w := httptest.NewRecorder()
	handleLogin(w, req)

	if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "有效订阅") {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestSubscriptionCardLoginByCardKeyBypassesTokenRevalidation(t *testing.T) {
	setupScratchLockTestDB(t)
	useSubscriptionRuntimeServer(t, 123, "alice", []subscriptionSummary{{
		ID:          901,
		UserID:      123,
		PlanID:      1,
		AmountTotal: newapi.DefaultQuotaUnit * 100,
		StartTime:   actStartTS + 60,
		EndTime:     actEndTS + 3600,
		Status:      "active",
	}}, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"userId":123,"username":"alice"}`))
	w := httptest.NewRecorder()
	handleLogin(w, req)
	cardKey := parseLoginResponse(t, w.Body.String())["cardKey"].(string)

	oldTokenSvc := tokenSvc
	oldTokenConfigErr := tokenConfigErr
	tokenSvc = nil
	tokenConfigErr = errors.New("broken token runtime")
	t.Cleanup(func() {
		tokenSvc = oldTokenSvc
		tokenConfigErr = oldTokenConfigErr
	})

	req2 := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"cardKey":"`+cardKey+`"}`))
	w2 := httptest.NewRecorder()
	handleLogin(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", w2.Code, w2.Body.String())
	}
}

func TestSubscriptionCardSpinBypassesTokenRevalidation(t *testing.T) {
	setupScratchLockTestDB(t)
	useSubscriptionRuntimeServer(t, 123, "alice", []subscriptionSummary{{
		ID:          901,
		UserID:      123,
		PlanID:      1,
		AmountTotal: newapi.DefaultQuotaUnit * 100,
		StartTime:   actStartTS + 60,
		EndTime:     actEndTS + 3600,
		Status:      "active",
	}}, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"userId":123,"username":"alice"}`))
	w := httptest.NewRecorder()
	handleLogin(w, req)
	cardKey := parseLoginResponse(t, w.Body.String())["cardKey"].(string)

	oldTokenSvc := tokenSvc
	oldTokenConfigErr := tokenConfigErr
	tokenSvc = nil
	tokenConfigErr = errors.New("broken token runtime")
	t.Cleanup(func() {
		tokenSvc = oldTokenSvc
		tokenConfigErr = oldTokenConfigErr
	})

	req2 := httptest.NewRequest(http.MethodPost, "/api/spin", strings.NewReader(`{"cardKey":"`+cardKey+`"}`))
	w2 := httptest.NewRecorder()
	handleSpin(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", w2.Code, w2.Body.String())
	}
}

func TestProcessCreditsRoutesSubscriptionCardPrizeToRewardPlanBinding(t *testing.T) {
	setupScratchLockTestDB(t)
	var mu sync.Mutex
	var planCreateBody map[string]any
	var bindBody map[string]any
	rewardPlanCreated := false
	rewardSubID := int64(0)
	useSubscriptionRuntimeServerWithConfig(t, subscriptionRuntimeTestConfig{
		UserID:   123,
		Username: "alice",
		Subs: []subscriptionSummary{{
			ID:          901,
			UserID:      123,
			PlanID:      1,
			AmountTotal: newapi.DefaultQuotaUnit * 100,
			StartTime:   actStartTS + 60,
			EndTime:     time.Now().Add(24 * time.Hour).Unix(),
			Status:      "active",
		}},
		Handler: func(w http.ResponseWriter, r *http.Request) bool {
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/api/subscription/admin/plans":
				mu.Lock()
				created := rewardPlanCreated
				mu.Unlock()
				items := []any{
					map[string]any{"plan": map[string]any{
						"id":             1,
						"title":          "normal-plan",
						"total_amount":   newapi.DefaultQuotaUnit * 100,
						"duration_unit":  "month",
						"duration_value": 1,
					}},
				}
				if created {
					items = append(items, map[string]any{"plan": map[string]any{
						"id":             77,
						"title":          rewardPlanPoolTitle(1),
						"total_amount":   newapi.DefaultQuotaUnit * 10,
						"duration_unit":  "month",
						"duration_value": 1,
						"enabled":        false,
					}})
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": items})
				return true
			case r.Method == http.MethodGet && r.URL.Path == "/api/subscription/admin/users/123/subscriptions":
				mu.Lock()
				subID := rewardSubID
				mu.Unlock()
				items := []any{
					map[string]any{"subscription": map[string]any{
						"id":           901,
						"user_id":      123,
						"plan_id":      1,
						"amount_total": newapi.DefaultQuotaUnit * 100,
						"amount_used":  0,
						"start_time":   actStartTS + 60,
						"end_time":     time.Now().Add(24 * time.Hour).Unix(),
						"status":       "active",
					}},
				}
				if subID > 0 {
					items = append(items, map[string]any{"subscription": map[string]any{
						"id":           subID,
						"user_id":      123,
						"plan_id":      77,
						"amount_total": newapi.DefaultQuotaUnit * 10,
						"amount_used":  0,
						"start_time":   time.Now().Unix(),
						"end_time":     time.Now().Add(30 * 24 * time.Hour).Unix(),
						"status":       "active",
					}})
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": items})
				return true
			case r.Method == http.MethodPost && r.URL.Path == "/api/subscription/admin/plans":
				if err := json.NewDecoder(r.Body).Decode(&planCreateBody); err != nil {
					t.Fatalf("decode plan body: %v", err)
				}
				mu.Lock()
				rewardPlanCreated = true
				mu.Unlock()
				_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]any{"id": 77}})
				return true
			case r.Method == http.MethodPut && r.URL.Path == "/api/subscription/admin/plans/77":
				_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
				return true
			case r.Method == http.MethodPost && r.URL.Path == "/api/subscription/admin/users/123/subscriptions":
				if err := json.NewDecoder(r.Body).Decode(&bindBody); err != nil {
					t.Fatalf("decode bind body: %v", err)
				}
				mu.Lock()
				rewardSubID = 9901
				mu.Unlock()
				_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
				return true
			}
			return false
		},
	})

	if _, err := db.Exec(`INSERT INTO cards (card_key, card_name, dollars, total_spins, source, subscription_id, user_id, username) VALUES (?,?,?,?,?,?,?,?)`,
		"actsub-credit", "alice", 100, 1, "subscription", 901, 123, "alice",
	); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO credit_queue (card_key, prize_dollars, status) VALUES (?,?,?)`, "actsub-credit", 10, "pending"); err != nil {
		t.Fatal(err)
	}

	processCredits()

	if planCreateBody == nil {
		t.Fatal("expected reward flow to create a temporary subscription plan")
	}
	plan, _ := planCreateBody["plan"].(map[string]any)
	if plan == nil {
		t.Fatalf("planCreateBody=%#v", planCreateBody)
	}
	if title := strings.TrimSpace(plan["title"].(string)); !strings.HasPrefix(strings.ToLower(title), rewardPlanTitlePrefix) {
		t.Fatalf("reward plan title=%q", title)
	}
	if got := int64(plan["total_amount"].(float64)); got != newapi.DefaultQuotaUnit*10 {
		t.Fatalf("total_amount=%d want %d", got, newapi.DefaultQuotaUnit*10)
	}
	if bindBody == nil {
		t.Fatal("expected reward flow to bind temporary plan to user")
	}
	if got := int64(bindBody["plan_id"].(float64)); got != 77 {
		t.Fatalf("plan_id=%d want 77", got)
	}

	var status string
	if err := db.QueryRow(`SELECT status FROM credit_queue WHERE card_key=?`, "actsub-credit").Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != creditStatusDone {
		t.Fatalf("status=%q want %q", status, creditStatusDone)
	}

	var rewardPlanID, rewardSubscriptionID int64
	if err := db.QueryRow(`SELECT reward_plan_id,reward_subscription_id FROM reward_issuance WHERE card_key=?`, "actsub-credit").Scan(&rewardPlanID, &rewardSubscriptionID); err != nil {
		t.Fatalf("reward issuance record: %v", err)
	}
	if rewardPlanID != 77 || rewardSubscriptionID != 9901 {
		t.Fatalf("reward issuance record = (%d,%d), want (77,9901)", rewardPlanID, rewardSubscriptionID)
	}
}
