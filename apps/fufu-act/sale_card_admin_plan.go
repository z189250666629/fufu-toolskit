package activityapp

import (
	"errors"
	"sort"
	"strings"
)

func saleCardPlanFromRunRequest(req saleCardRunRequest) (SaleCardPlan, error) {
	plan := SaleCardPlan{}
	planID := strings.TrimSpace(req.Plan)
	if planID != "" {
		template, ok := saleCardPlanTemplates()[planID]
		if !ok {
			return SaleCardPlan{}, errors.New("未知上架计划")
		}
		plan = template
	}
	if req.Name != "" {
		plan.Name = req.Name
	}
	if req.Count > 0 {
		plan.Count = req.Count
	}
	if req.TargetStock > 0 {
		plan.TargetStock = req.TargetStock
	}
	if req.Quota > 0 {
		plan.Quota = req.Quota
	}
	if req.Group != "" {
		plan.Group = req.Group
	}
	if req.IntervalUnit != 0 {
		plan.IntervalUnit = req.IntervalUnit
	}
	if req.ItemID > 0 {
		plan.ItemID = req.ItemID
	}
	if req.SKUID > 0 {
		plan.SKUID = req.SKUID
	}
	if req.Remark != "" {
		plan.Remark = req.Remark
	}
	if req.Unique != nil {
		plan.Unique = *req.Unique
	}
	if req.TokenNameSlug != "" {
		plan.TokenNameSlug = req.TokenNameSlug
	}
	if plan.ID == "" {
		plan.ID = planID
	}
	plan = normalizeSaleCardPlan(plan)
	if err := validateSaleCardPlan(plan); err != nil {
		return SaleCardPlan{}, err
	}
	return plan, nil
}

func saleCardPlanList() []SaleCardPlan {
	templates := saleCardPlanTemplates()
	keys := make([]string, 0, len(templates))
	for key := range templates {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	plans := make([]SaleCardPlan, 0, len(keys))
	for _, key := range keys {
		plan := templates[key]
		plan.Count = 0
		plans = append(plans, plan)
	}
	return plans
}
