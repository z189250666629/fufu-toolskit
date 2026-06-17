package tokens

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"fufu/newapi"
)

func TestLiveNewAPITokenCreateResolveSearchGetDeleteContract(t *testing.T) {
	site := liveNewAPISiteFromEnv(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	svc := NewService(newapi.NewClient(site))
	name := "live-" + strconv.FormatInt(time.Now().UnixMilli(), 36)
	quota := svc.DollarsToQuota(1)
	created, err := svc.CreateTokenAndResolveKey(ctx, map[string]any{
		"name":              name,
		"remain_quota":      quota,
		"unlimited_quota":   false,
		"expired_time":      -1,
		"group":             liveNewAPIGroupFromEnv(),
		"interval_quota":    quota,
		"interval_time":     -1,
		"trigger_last_time": 0,
		"interval_unit":     liveNewAPIIntervalUnitFromEnv(),
	}, name)
	if err != nil {
		t.Fatalf("create+resolve live token: %v", err)
	}
	if created.Token.ID <= 0 || strings.TrimSpace(created.Key) == "" {
		t.Fatalf("created live token missing id or key shape: id=%d keyPresent=%v", created.Token.ID, strings.TrimSpace(created.Key) != "")
	}
	deleted := false
	defer func() {
		if deleted || created.Token.ID <= 0 {
			return
		}
		ok, _, err := svc.DeleteToken(context.Background(), created.Token.ID)
		if err != nil || !ok {
			t.Logf("cleanup live token id=%d failed: ok=%v err=%v", created.Token.ID, ok, err)
		}
	}()

	byName, err := svc.SearchTokenByName(ctx, name)
	if err != nil || byName == nil || byName.ID != created.Token.ID {
		t.Fatalf("search by name failed: found=%v err=%v", byName != nil, err)
	}
	byKey, err := svc.SearchTokenByKey(ctx, created.Key)
	if err != nil || byKey == nil || byKey.ID != created.Token.ID {
		t.Fatalf("search by key failed: found=%v err=%v", byKey != nil, err)
	}
	detail, err := svc.GetToken(ctx, created.Token.ID)
	if err != nil || detail.ID != created.Token.ID || strings.TrimSpace(detail.Key) == "" {
		t.Fatalf("get token detail failed: id=%d keyPresent=%v err=%v", detail.ID, strings.TrimSpace(detail.Key) != "", err)
	}
	ok, _, err := svc.DeleteToken(ctx, created.Token.ID)
	if err != nil || !ok {
		t.Fatalf("delete live token id=%d failed: ok=%v err=%v", created.Token.ID, ok, err)
	}
	deleted = true
}

func liveNewAPISiteFromEnv(t *testing.T) newapi.Site {
	t.Helper()
	if os.Getenv("FUFU_LIVE_CONTRACT") != "1" {
		t.Skip("set FUFU_LIVE_CONTRACT=1 to run live NewAPI contract tests")
	}
	baseURL := strings.TrimSpace(os.Getenv("NEWAPI_BASE_URL"))
	token := strings.TrimSpace(os.Getenv("NEWAPI_ACCESS_TOKEN"))
	userID := strings.TrimSpace(os.Getenv("NEWAPI_USER_ID"))
	if baseURL == "" || token == "" || userID == "" {
		t.Skip("NEWAPI_BASE_URL, NEWAPI_ACCESS_TOKEN and NEWAPI_USER_ID are required for live NewAPI contract tests")
	}
	return newapi.Site{
		Name:      "live-contract",
		URL:       baseURL,
		Token:     token,
		UserID:    userID,
		QuotaUnit: liveNewAPIQuotaUnitFromEnv(),
	}
}

func liveNewAPIGroupFromEnv() string {
	if value := strings.TrimSpace(os.Getenv("FUFU_LIVE_NEWAPI_GROUP")); value != "" {
		return value
	}
	return "mix"
}

func liveNewAPIIntervalUnitFromEnv() int {
	value := strings.TrimSpace(os.Getenv("FUFU_LIVE_NEWAPI_INTERVAL_UNIT"))
	if value == "" {
		return 9
	}
	n, err := strconv.Atoi(value)
	if err != nil || n == 0 {
		return 9
	}
	return n
}

func liveNewAPIQuotaUnitFromEnv() int64 {
	value := strings.TrimSpace(os.Getenv("FUFU_LIVE_NEWAPI_QUOTA_UNIT"))
	if value == "" {
		return newapi.DefaultQuotaUnit
	}
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil || n <= 0 {
		fmt.Fprintf(os.Stderr, "invalid FUFU_LIVE_NEWAPI_QUOTA_UNIT, using default\n")
		return newapi.DefaultQuotaUnit
	}
	return n
}
