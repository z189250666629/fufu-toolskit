import type { ActivityConfig, ActivityGameConfig, ActivityGameRoute, DynamicPrizePoolConfig, DynamicPrizePoolTier, SaleCardPlan, SubscriptionPlanMapping } from './types';

export type GameMode = 'slot' | 'scratch' | 'dragonboat';

export const DEFAULT_SCRATCH_MAX_REVEALS = 6;
export const SCRATCH_SAFE_CELLS = 7;

export type SaleCardTierOption = {
  quota: number;
  label: string;
  planText: string;
  planIds: string[];
  primaryPlanId?: string;
};

export const GAME_MODE_OPTIONS: { value: GameMode; label: string }[] = [
  { value: 'slot', label: '老虎机' },
  { value: 'scratch', label: '刮刮乐' },
  { value: 'dragonboat', label: '端午捕粽' }
];

export function numberValue(value: unknown): number {
  const next = Number(value);
  return Number.isFinite(next) ? next : 0;
}

export function isActivityEnabled(activity: ActivityConfig = {}): boolean {
  if (typeof activity.enabled === 'boolean') return activity.enabled;
  if (typeof activity.disabled === 'boolean') return !activity.disabled;
  return true;
}

export function activityEnabledPatch(enabled: boolean): ActivityConfig {
  return { enabled, disabled: undefined };
}

export function rateToPercentInput(value: unknown): string {
  const percent = numberValue(value) * 100;
  return String(Number(percent.toFixed(2)));
}

export function percentInputToRate(value: string): number {
  const percent = Number(value);
  return Number.isFinite(percent) ? Math.max(0, percent) / 100 : 0;
}

export function normalizeScratchMaxReveals(value: unknown): number {
  const count = Math.floor(numberValue(value));
  if (count <= 0) return DEFAULT_SCRATCH_MAX_REVEALS;
  return Math.max(1, Math.min(SCRATCH_SAFE_CELLS, count));
}

export function formatQuota(value: number): string {
  return Number.isInteger(value) ? String(value) : String(Number(value.toFixed(2)));
}

export function uniqueText(values: unknown[]): string[] {
  const seen = new Set<string>();
  const out: string[] = [];
  for (const value of values) {
    const text = String(value ?? '').trim();
    if (text === '' || seen.has(text)) continue;
    seen.add(text);
    out.push(text);
  }
  return out;
}

export function buildSaleCardTierOptions(plans: SaleCardPlan[] = []): SaleCardTierOption[] {
  const byQuota = new Map<number, SaleCardPlan[]>();
  for (const plan of plans) {
    const quota = numberValue(plan.quota);
    if (quota <= 0) continue;
    byQuota.set(quota, [...(byQuota.get(quota) ?? []), plan]);
  }
  return [...byQuota.entries()]
    .sort(([a], [b]) => a - b)
    .map(([quota, rows]) => {
      const names = uniqueText(rows.map((row) => row.name || row.id));
      const planIds = uniqueText(rows.map((row) => row.id));
      return {
        quota,
        label: names.join(' / ') || `${formatQuota(quota)} 次卡`,
        planText: planIds.join(' / ') || '-',
        planIds,
        primaryPlanId: planIds[0]
      };
    });
}

export function buildSubscriptionTierOptions(activity: ActivityConfig): SaleCardTierOption[] {
  const byQuota = new Map<number, SubscriptionPlanMapping[]>();
  for (const mapping of normalizeSubscriptionPlanMappings(activity.subscriptionPlanMappings)) {
    const quota = numberValue(mapping.dollars);
    if (quota <= 0) continue;
    byQuota.set(quota, [...(byQuota.get(quota) ?? []), mapping]);
  }
  return [...byQuota.entries()]
    .sort(([a], [b]) => a - b)
    .map(([quota, rows]) => {
      const names = uniqueText(rows.map((row) => row.title || (row.planId ? `订阅计划 ${row.planId}` : '订阅映射')));
      const planText = uniqueText(rows.map((row) => row.planId ? `planId:${row.planId}` : row.title)).join(' / ') || 'subscription';
      return {
        quota,
        label: names.join(' / ') || `${formatQuota(quota)} 次订阅档`,
        planText,
        planIds: []
      };
    });
}

export function buildActivityTierOptions(plans: SaleCardPlan[] = [], activity: ActivityConfig = {}): SaleCardTierOption[] {
  const byQuota = new Map<number, SaleCardTierOption>();
  for (const option of buildSaleCardTierOptions(plans)) {
    byQuota.set(option.quota, option);
  }
  for (const option of buildSubscriptionTierOptions(activity)) {
    const current = byQuota.get(option.quota);
    if (!current) {
      byQuota.set(option.quota, option);
      continue;
    }
    byQuota.set(option.quota, {
      ...current,
      label: uniqueText([current.label, option.label]).join(' / '),
      planText: uniqueText([current.planText, option.planText]).join(' / '),
      planIds: current.planIds,
      primaryPlanId: current.primaryPlanId
    });
  }
  return [...byQuota.values()].sort((a, b) => a.quota - b.quota);
}

export function normalizeGameMode(value: unknown): GameMode {
  const text = String(value ?? '').trim().toLowerCase();
  if (text === 'scratch' || text === '刮刮乐') return 'scratch';
  if (text === 'dragonboat' || text === 'dragon' || text === 'duanwu' || text === '端午' || text === '黄金矿工' || text === '端午捕粽') return 'dragonboat';
  return 'slot';
}

export function gameModeLabel(game: string): string {
  return GAME_MODE_OPTIONS.find((option) => option.value === normalizeGameMode(game))?.label ?? String(game || '-');
}

export function normalizeGameConfigs(configs: ActivityGameConfig[] = [], activity?: ActivityConfig): ActivityGameConfig[] {
  const byGame = new Map<GameMode, ActivityGameConfig>();
  const fallbackTarget = numberValue(activity?.targetExpectedValue);
  for (const mode of GAME_MODE_OPTIONS) {
    byGame.set(mode.value, {
      game: mode.value,
      targetExpectedValue: fallbackTarget
    });
  }
  for (const item of configs) {
    const game = normalizeGameMode(item.game);
    byGame.set(game, {
      game,
      targetExpectedValue: numberValue(item.targetExpectedValue)
    });
  }
  return GAME_MODE_OPTIONS.map((mode) => byGame.get(mode.value) ?? { game: mode.value });
}

export function patchGameConfig(configs: ActivityGameConfig[], game: GameMode, patch: Partial<ActivityGameConfig>): ActivityGameConfig[] {
  return normalizeGameConfigs(configs.map((item) => (
    normalizeGameMode(item.game) === game ? { ...item, ...patch, game } : item
  )));
}

export function normalizeGameRoutes(routes: ActivityGameRoute[] = []): ActivityGameRoute[] {
  const byTier = new Map<number, ActivityGameRoute>();
  for (const route of routes) {
    const dollars = numberValue((route as ActivityGameRoute & { quota?: unknown; tier?: unknown }).dollars ?? (route as ActivityGameRoute & { quota?: unknown }).quota ?? (route as ActivityGameRoute & { tier?: unknown }).tier);
    if (dollars <= 0) continue;
    const drawCount = numberValue(route.drawCount);
    byTier.set(dollars, { dollars, game: normalizeGameMode(route.game), ...(drawCount > 0 ? { drawCount } : {}) });
  }
  return [...byTier.entries()]
    .sort(([a], [b]) => a - b)
    .map(([, route]) => route);
}

export function normalizeSubscriptionPlanMappings(mappings: SubscriptionPlanMapping[] = []): SubscriptionPlanMapping[] {
  return mappings
    .map((mapping) => {
      const planId = Math.floor(numberValue(mapping.planId));
      const title = String(mapping.title ?? '').trim();
      const dollars = numberValue(mapping.dollars);
      const match = String(mapping.match ?? '').trim().toLowerCase() === 'exact' ? 'exact' : 'contains';
      return {
        ...(planId > 0 ? { planId } : {}),
        ...(title ? { title, match } : {}),
        dollars
      } as SubscriptionPlanMapping;
    })
    .filter((mapping) => numberValue(mapping.dollars) > 0 && (numberValue(mapping.planId) > 0 || String(mapping.title ?? '').trim() !== ''));
}

export function upsertSubscriptionPlanMapping(mappings: SubscriptionPlanMapping[], index: number, patch: Partial<SubscriptionPlanMapping>): SubscriptionPlanMapping[] {
  const current = [...mappings];
  current[index] = { ...(current[index] ?? { dollars: 0 }), ...patch };
  return normalizeSubscriptionPlanMappings(current);
}

export function gameRoutesFromActivity(activity: ActivityConfig): ActivityGameRoute[] {
  if (Array.isArray(activity.gameRoutes)) {
    return normalizeGameRoutes(activity.gameRoutes);
  }
  return normalizeGameRoutes((activity.scratchTiers ?? []).map((dollars) => ({ dollars: Number(dollars), game: 'scratch' })));
}

export function materializeGameRoutesForSaleTiers(activity: ActivityConfig, saleTierOptions: SaleCardTierOption[]): ActivityGameRoute[] {
  const routes = gameRoutesFromActivity(activity);
  const byTier = new Map<number, ActivityGameRoute>();
  for (const route of routes) {
    byTier.set(numberValue(route.dollars), route);
  }
  for (const option of saleTierOptions) {
    const current = byTier.get(option.quota);
    const game = normalizeGameMode(current?.game);
    const explicit = numberValue(current?.drawCount);
    const legacySlotCount = game === 'slot' ? spinMapDrawCount(activity, option.quota) : 0;
    byTier.set(option.quota, {
      dollars: option.quota,
      game,
      drawCount: explicit > 0 ? explicit : legacySlotCount > 0 ? legacySlotCount : 1
    });
  }
  return normalizeGameRoutes([...byTier.values()]);
}

export function stripComputedExpectedValues(activity: ActivityConfig): ActivityConfig {
  const source = activity as ActivityConfig & {
    actualExpectedValue?: unknown;
  };
  const rawGameConfigs = (source.gameConfigs ?? []) as Array<ActivityGameConfig & { actualExpectedValue?: unknown }>;
  const { actualExpectedValue: _actualExpectedValue, ...rest } = source;
  return {
    ...rest,
    gameConfigs: rawGameConfigs.map((item) => {
      const { actualExpectedValue: _itemActualExpectedValue, ...config } = item;
      return config;
    })
  };
}

export function materializeActivityGameRoutes(activity: ActivityConfig, salePlans: SaleCardPlan[] = []): ActivityConfig {
  const sanitized = stripComputedExpectedValues(activity);
  sanitized.scratchMaxReveals = normalizeScratchMaxReveals(sanitized.scratchMaxReveals);
  const saleTierOptions = buildActivityTierOptions(salePlans, sanitized);
  if (saleTierOptions.length === 0) {
    return sanitized;
  }
  const gameRoutes = materializeGameRoutesForSaleTiers(sanitized, saleTierOptions);
  return { ...sanitized, gameRoutes, scratchTiers: scratchTiersFromGameRoutes(gameRoutes) };
}

export function scratchTiersFromGameRoutes(routes: ActivityGameRoute[]): number[] {
  return normalizeGameRoutes(routes)
    .filter((route) => route.game === 'scratch')
    .map((route) => route.dollars);
}

export function normalizeDynamicPrizePool(pool?: DynamicPrizePoolConfig): DynamicPrizePoolConfig {
  return {
    enabled: Boolean(pool?.enabled),
    contributionRate: numberValue(pool?.contributionRate),
    jackpotRate: numberValue(pool?.jackpotRate),
    secondRate: numberValue(pool?.secondRate),
    thirdRate: numberValue(pool?.thirdRate),
    tierEconomics: (pool?.tierEconomics ?? [])
      .map((tier) => ({
        dollars: numberValue(tier.dollars),
        revenue: numberValue(tier.revenue),
        cost: numberValue(tier.cost)
      }))
      .filter((tier) => tier.dollars > 0)
  };
}

export function dynamicTierForQuota(pool: DynamicPrizePoolConfig, quota: number): DynamicPrizePoolTier {
  return (pool.tierEconomics ?? []).find((tier) => numberValue(tier.dollars) === quota) ?? { dollars: quota, revenue: 0, cost: 0 };
}

export function upsertDynamicTier(pool: DynamicPrizePoolConfig, quota: number, patch: Partial<DynamicPrizePoolTier>): DynamicPrizePoolConfig {
  const current = normalizeDynamicPrizePool(pool);
  const tiers = (current.tierEconomics ?? []).filter((tier) => numberValue(tier.dollars) !== quota);
  const next = { ...dynamicTierForQuota(current, quota), ...patch, dollars: quota };
  tiers.push(next);
  tiers.sort((a, b) => numberValue(a.dollars) - numberValue(b.dollars));
  return { ...current, tierEconomics: tiers };
}

export function upsertGameRoute(routes: ActivityGameRoute[], dollars: number, patch: Partial<ActivityGameRoute>): ActivityGameRoute[] {
  const current = normalizeGameRoutes(routes).find((route) => numberValue(route.dollars) === dollars);
  const game = normalizeGameMode(patch.game ?? current?.game);
  const drawCount = numberValue(patch.drawCount ?? current?.drawCount);
  return normalizeGameRoutes([
    ...routes.filter((route) => numberValue(route.dollars) !== dollars),
    { dollars, game, ...(drawCount > 0 ? { drawCount } : {}) }
  ]);
}

export function spinMapDrawCount(activity: ActivityConfig, quota: number): number {
  for (const [key, value] of Object.entries(activity.spinMap ?? {})) {
    if (numberValue(key) === quota && numberValue(value) > 0) return numberValue(value);
  }
  return 0;
}

export function padDatePart(value: number): string {
  return String(value).padStart(2, '0');
}

export function formatActivityDateText(date: Date): string {
  return `${date.getFullYear()}-${padDatePart(date.getMonth() + 1)}-${padDatePart(date.getDate())} ${padDatePart(date.getHours())}:${padDatePart(date.getMinutes())}:${padDatePart(date.getSeconds())}`;
}

export function activityDateTimeValue(text?: string, timestamp?: number): string {
  const source = text?.trim()
    ? new Date(text.trim().replace(' ', 'T'))
    : timestamp
      ? new Date(timestamp * 1000)
      : null;
  if (!source || Number.isNaN(source.getTime())) {
    return '';
  }
  return `${source.getFullYear()}-${padDatePart(source.getMonth() + 1)}-${padDatePart(source.getDate())}T${padDatePart(source.getHours())}:${padDatePart(source.getMinutes())}:${padDatePart(source.getSeconds())}`;
}

export function activityDatePatch(kind: 'start' | 'end', value: string): ActivityConfig {
  if (!value) {
    return kind === 'start' ? { startText: '', startTS: undefined } : { endText: '', endTS: undefined };
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return {};
  }
  const seconds = Math.floor(date.getTime() / 1000);
  return kind === 'start'
    ? { startText: formatActivityDateText(date), startTS: seconds }
    : { endText: formatActivityDateText(date), endTS: seconds };
}
