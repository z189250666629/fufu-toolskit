import type { SaleCardConfig, SaleCardPlan } from './types';

export const SALE_SLOTS: { group: string; label: string; defaultTime: string }[] = [
  { group: 'special55', label: '55 次混合特惠卡', defaultTime: '09:00' },
  { group: 'month', label: '月次卡', defaultTime: '09:30' }
];

export type SlotState = {
  time: string;
  enabled: boolean;
  jobs: Record<string, { targetStock: number; enabled: boolean }>;
};

export type TargetStockValidation = {
  ok: boolean;
  target: number;
  message?: string;
};

export function buildSlotState(config?: SaleCardConfig): Record<string, SlotState> {
  const plans = config?.plans ?? [];
  const slots = config?.schedule?.slots ?? [];
  const state: Record<string, SlotState> = {};
  for (const def of SALE_SLOTS) {
    const saved = slots.find((slot) => slot.group === def.group);
    const jobs: Record<string, { targetStock: number; enabled: boolean }> = {};
    for (const plan of plans.filter((p) => (p.slot || '') === def.group)) {
      const savedJob = saved?.jobs?.find((job) => job.plan === plan.id);
      jobs[plan.id] = { targetStock: savedJob?.targetStock ?? 0, enabled: Boolean(savedJob?.enabled) };
    }
    state[def.group] = { time: saved?.time || def.defaultTime, enabled: Boolean(saved?.enabled), jobs };
  }
  return state;
}

export function buildSaleCardSchedule(
  enabled: boolean,
  timezone: string,
  slotState: Record<string, SlotState>,
  plans: SaleCardPlan[]
): NonNullable<SaleCardConfig['schedule']> {
  return {
    enabled,
    timezone,
    slots: SALE_SLOTS.map((def) => {
      const slot = slotState[def.group];
      const slotPlans = plans.filter((plan) => (plan.slot || '') === def.group);
      return {
        group: def.group,
        time: slot.time,
        enabled: slot.enabled,
        jobs: slotPlans.map((plan) => ({
          plan: plan.id,
          targetStock: slot.jobs[plan.id]?.targetStock ?? 0,
          enabled: Boolean(slot.jobs[plan.id]?.enabled)
        }))
      };
    })
  };
}

export function validateTargetStock(value: unknown): TargetStockValidation {
  const target = Math.trunc(Number(value));
  if (!Number.isFinite(target) || target < 0 || target > 2000) {
    return { ok: false, target: 0, message: '补卡目标库存必须在 0 到 2000 之间。' };
  }
  return { ok: true, target };
}
