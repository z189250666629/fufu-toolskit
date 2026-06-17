package activityapp

import (
	"strings"
	"testing"

	"fufu/activity"
	"fufu/newapi"
	"fufu/tokens"
)

func TestPlanLoginCardForTokenUsesShopPurchaseInsideWindow(t *testing.T) {
	cfg := activity.DefaultConfig()
	cfg.GameConfigs = []activity.GameConfig{{Game: activity.GameSlot, TargetExpectedValue: 4.5, ActualExpectedValue: 4.5}}

	plan, err := planLoginCardForToken("sk-shop-card", &tokens.Token{
		Name:          "shop-card",
		Status:        1,
		IntervalQuota: int64(newapi.DefaultQuotaUnit) * 100,
		CreatedTime:   cfg.StartTS - 1,
	}, ShopPurchaseLookup{Configured: true, PurchaseTime: cfg.StartText}, cfg, newapi.DefaultQuotaUnit)

	if err != nil {
		t.Fatalf("planLoginCardForToken() error = %v", err)
	}
	if plan.CardKey != "sk-shop-card" || plan.CardName != "shop-card" || plan.Dollars != 100 || plan.TotalSpins != 1 || plan.Source != "shop" || plan.PurchaseTime != cfg.StartText {
		t.Fatalf("plan = %#v", plan)
	}
}

func TestPlanLoginCardForTokenUsesSlotDrawCountByCardTier(t *testing.T) {
	cfg := activity.DefaultConfig()
	cfg.GameConfigs = []activity.GameConfig{{Game: activity.GameSlot, TargetExpectedValue: 4.5, ActualExpectedValue: 4.5}}

	cases := []struct {
		name  string
		tier  int64
		draws int
	}{
		{"100 card", 100, 1},
		{"300 card", 300, 3},
		{"1000 card", 1000, 10},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			plan, err := planLoginCardForToken("sk-tier-card", &tokens.Token{
				Name:          c.name,
				Status:        1,
				IntervalQuota: int64(newapi.DefaultQuotaUnit) * c.tier,
				CreatedTime:   cfg.StartTS,
			}, ShopPurchaseLookup{Configured: true}, cfg, newapi.DefaultQuotaUnit)

			if err != nil {
				t.Fatalf("planLoginCardForToken() error = %v", err)
			}
			if plan.TotalSpins != c.draws {
				t.Fatalf("tier %d total spins=%d, want %d; plan=%#v", c.tier, plan.TotalSpins, c.draws, plan)
			}
		})
	}
}

func TestPlanLoginCardForTokenAllowsPurchasedScratchTierWithoutSpinMap(t *testing.T) {
	cfg := activity.DefaultConfig()
	cfg.GameConfigs = []activity.GameConfig{
		{Game: activity.GameSlot, TargetExpectedValue: 4.5, ActualExpectedValue: 4.5},
		{Game: activity.GameScratch, TargetExpectedValue: 2.5, ActualExpectedValue: 2.5},
	}
	cfg.ScratchTiers = []int{55}

	plan, err := planLoginCardForToken("sk-scratch-card", &tokens.Token{
		Name:          "scratch-card",
		Status:        1,
		IntervalQuota: int64(newapi.DefaultQuotaUnit) * 55,
		CreatedTime:   cfg.StartTS - 1,
	}, ShopPurchaseLookup{Configured: true, PurchaseTime: cfg.StartText}, cfg, newapi.DefaultQuotaUnit)

	if err != nil {
		t.Fatalf("planLoginCardForToken() error = %v", err)
	}
	if plan.Dollars != 55 || plan.TotalSpins != 1 || plan.Source != "shop" || plan.PurchaseTime != cfg.StartText {
		t.Fatalf("plan = %#v", plan)
	}
}

func TestPlanLoginCardForTokenRejectsOutOfWindowShopTokenWithoutPurchase(t *testing.T) {
	cfg := activity.DefaultConfig()

	_, err := planLoginCardForToken("sk-outside-card", &tokens.Token{
		Name:          "outside-card",
		Status:        1,
		IntervalQuota: int64(newapi.DefaultQuotaUnit) * 100,
		CreatedTime:   cfg.StartTS - 1,
	}, ShopPurchaseLookup{Configured: true}, cfg, newapi.DefaultQuotaUnit)

	if err == nil {
		t.Fatal("expected error")
	}
	httpErr, ok := err.(httpErr)
	if !ok || httpErr.Status != 403 || !strings.Contains(httpErr.Message, "不在活动期间") {
		t.Fatalf("error = %#v", err)
	}
}
