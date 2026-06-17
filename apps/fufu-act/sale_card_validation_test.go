package activityapp

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func validBaseSaleCardPlan() SaleCardPlan {
	return SaleCardPlan{Count: 2, Quota: 55, Group: saleCardDefaultTokenGroup, IntervalUnit: 9, ItemID: 29, SKUID: 66}
}

func TestValidateSaleCardPlanAcceptsCountOrTargetMode(t *testing.T) {
	if err := validateSaleCardPlan(validBaseSaleCardPlan()); err != nil {
		t.Fatalf("count-mode plan should be valid: %v", err)
	}
	restock := validBaseSaleCardPlan()
	restock.Count = 0
	restock.TargetStock = 500
	if err := validateSaleCardPlan(restock); err != nil {
		t.Fatalf("restock-mode plan should be valid: %v", err)
	}
}

func TestValidateSaleCardPlanRejectsInvalidFields(t *testing.T) {
	type tc struct {
		name    string
		mutate  func(p *SaleCardPlan)
		wantMsg string
	}
	for _, c := range []tc{
		{"negative target", func(p *SaleCardPlan) { p.TargetStock = -1 }, "target stock cannot be negative"},
		{"target over 2000", func(p *SaleCardPlan) { p.TargetStock = 2001 }, "target stock must be 2000 or fewer"},
		{"count below 1 no target", func(p *SaleCardPlan) { p.Count = 0 }, "count must be between 1 and 100"},
		{"count above 100 no target", func(p *SaleCardPlan) { p.Count = 101 }, "count must be between 1 and 100"},
		{"non-positive quota", func(p *SaleCardPlan) { p.Quota = 0 }, "quota must be positive"},
		{"zero interval unit", func(p *SaleCardPlan) { p.IntervalUnit = 0 }, "interval unit is required"},
		{"missing item id", func(p *SaleCardPlan) { p.ItemID = 0 }, "MCY item_id and sku_id are required"},
		{"missing sku id", func(p *SaleCardPlan) { p.SKUID = 0 }, "MCY item_id and sku_id are required"},
	} {
		t.Run(c.name, func(t *testing.T) {
			plan := validBaseSaleCardPlan()
			c.mutate(&plan)
			err := validateSaleCardPlan(plan)
			if err == nil || !errors.Is(err, ErrSaleCardInvalidPlan) {
				t.Fatalf("err=%v, want ErrSaleCardInvalidPlan", err)
			}
			if !strings.Contains(err.Error(), c.wantMsg) {
				t.Fatalf("err=%q, want substring %q", err.Error(), c.wantMsg)
			}
		})
	}
}

func TestSaleCardPlanSlotMapsKnownAndUnknownPlans(t *testing.T) {
	cases := map[string]string{
		"fufu-mix-special-55":                saleCardSlotSpecial55,
		"fufu-mix-month-100":                 saleCardSlotMonth,
		"fufu-mix-month-1000":                saleCardSlotMonth,
		"fufu-mix-month-custom-not-template": "",
		"unknown-plan":                       "",
		"":                                   "",
	}
	for plan, want := range cases {
		if got := saleCardPlanSlot(plan); got != want {
			t.Fatalf("saleCardPlanSlot(%q)=%q, want %q", plan, got, want)
		}
	}
}

func TestSaleCardPlanFromRunRequestOverridesTemplateFields(t *testing.T) {
	unique := false
	plan, err := saleCardPlanFromRunRequest(saleCardRunRequest{
		Plan:        "fufu-mix-special-55",
		Count:       7,
		TargetStock: 100,
		Name:        "自定义卡",
		Quota:       77,
		Group:       "custom",
		ItemID:      99,
		SKUID:       88,
		Remark:      "自定义备注",
		Unique:      &unique,
	})
	if err != nil {
		t.Fatalf("saleCardPlanFromRunRequest error: %v", err)
	}
	if plan.Count != 7 || plan.TargetStock != 100 || plan.Name != "自定义卡" || plan.Quota != 77 {
		t.Fatalf("overrides not applied: %#v", plan)
	}
	if plan.Group != "custom" || plan.ItemID != 99 || plan.SKUID != 88 || plan.Remark != "自定义备注" || plan.Unique {
		t.Fatalf("overrides not applied: %#v", plan)
	}
}

func TestSaleCardPlanFromRunRequestKeepsTemplateWhenOverridesZero(t *testing.T) {
	plan, err := saleCardPlanFromRunRequest(saleCardRunRequest{Plan: "fufu-mix-special-55", Count: 3})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	// TargetStock not sent (0) -> template default (0) kept; item/sku from template.
	if plan.TargetStock != 0 || plan.ItemID != 29 || plan.SKUID != 66 || plan.Quota != 55 || plan.IntervalUnit != 3 {
		t.Fatalf("template fields should remain: %#v", plan)
	}
}

func TestSaleCardPlanFromRunRequestRejectsUnknownPlan(t *testing.T) {
	if _, err := saleCardPlanFromRunRequest(saleCardRunRequest{Plan: "does-not-exist"}); err == nil {
		t.Fatal("unknown plan should be rejected")
	}
}

func TestWriteSaleCardRunErrorMapsStatusCodes(t *testing.T) {
	cases := []struct {
		name string
		err  error
		code int
		msg  string
	}{
		{"invalid plan", fmt.Errorf("%w: bad", ErrSaleCardInvalidPlan), http.StatusBadRequest, ""},
		{"shop login", fmt.Errorf("%w: x", ErrShopLoginFailed), http.StatusBadGateway, "MCY 上架失败"},
		{"shop request", fmt.Errorf("%w: x", ErrShopRequestFailed), http.StatusBadGateway, "MCY 上架失败"},
		{"shop invalid response", fmt.Errorf("%w: x", ErrShopInvalidResponse), http.StatusBadGateway, "MCY 上架失败"},
		{"generation failed", fmt.Errorf("%w: x", ErrSaleCardGenerationFailed), http.StatusBadGateway, "次数 fufu 生成卡密失败"},
		{"unknown", errors.New("boom"), http.StatusInternalServerError, "服务器错误"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			writeSaleCardRunError(w, c.err)
			if w.Code != c.code {
				t.Fatalf("code=%d, want %d", w.Code, c.code)
			}
			if c.msg != "" && !strings.Contains(w.Body.String(), c.msg) {
				t.Fatalf("body=%q, want substring %q", w.Body.String(), c.msg)
			}
		})
	}
}
