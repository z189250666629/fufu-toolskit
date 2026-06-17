package activityapp

import "fufu/salecore"

const (
	saleCardSlotSpecial55     = salecore.DefaultSaleCardSlotSpecial55
	saleCardSlotMonth         = salecore.DefaultSaleCardSlotMonth
	saleCardDefaultTokenGroup = salecore.DefaultSaleCardTokenGroup
)

var saleCardSlotDefs = salecore.DefaultSaleCardSlotDefinitions()

func saleCardPlanSlot(planID string) string {
	return salecore.SaleCardPlanSlot(saleCardPlanTemplates(), planID)
}

func saleCardPlanTemplates() map[string]SaleCardPlan {
	return salecore.DefaultSaleCardPlanTemplates()
}

func defaultSaleCardSchedule() SaleCardScheduleConfig {
	return salecore.DefaultSaleCardSchedule(saleCardScheduleCatalog())
}
