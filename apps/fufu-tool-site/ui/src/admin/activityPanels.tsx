import { useEffect, useRef, useState } from 'react';
import { Button, Input } from '@heroui/react';
import { DataTable, Metric } from './adminShared';
import {
  GAME_MODE_OPTIONS,
  activityDatePatch,
  activityDateTimeValue,
  buildSaleCardTierOptions,
  dynamicTierForQuota,
  gameModeLabel,
  gameRoutesFromActivity,
  normalizeDynamicPrizePool,
  normalizeGameConfigs,
  normalizeGameMode,
  normalizeGameRoutes,
  numberValue,
  patchGameConfig,
  percentInputToRate,
  rateToPercentInput,
  scratchTiersFromGameRoutes,
  spinMapDrawCount,
  stripComputedExpectedValues,
  upsertDynamicTier,
  upsertGameRoute,
  type GameMode
} from './activityConfigCore';
import type { ActivityConfig, ActivityGameConfig, ActivityGameRoute, ActivityStats, DynamicPrizePoolConfig, DynamicPrizePoolTier, PrizeConfigResponse, SaleCardPlan } from './types';

export function ActivityStatsPanel({ stats }: { stats?: ActivityStats }) {
  const prizeSummary = Array.isArray(stats?.prizeRows) ? stats.prizeRows as Array<Record<string, unknown>> : [];
  const tierRows = Array.isArray(stats?.tierRows) ? stats.tierRows as Array<Record<string, unknown>> : [];
  return (
    <div className="business-stack">
      <div className="metrics">
        <Metric label="总抽奖次数" value={stats?.totalSpins} />
        <Metric label="总中奖金额" value={stats?.totalWon} />
        <Metric label="实际期望值 / 次" value={stats?.expectedValue} />
        <Metric label="活动运行状态" value="实时" />
      </div>
      <DataTable
        columns={['奖项', '次数', '合计', '占比']}
        rows={prizeSummary.map((row) => [row.prize || row.label || '-', row.count, row.total, row.rate || row.ratio])}
        empty={stats ? '暂无奖项统计。' : '正在加载活动统计。'}
      />
      <DataTable
        columns={['面额', '卡数', '已用/总次', '总中奖']}
        rows={tierRows.map((row) => [row.tier || row.quota || '-', row.cards || row.count, `${row.used ?? '-'}/${row.spins ?? row.totalSpins ?? '-'}`, row.totalWon ?? '-'])}
        empty="暂无分面额统计。"
      />
    </div>
  );
}

export function CurrentPrizePanel({ prizes }: { prizes?: PrizeConfigResponse }) {
  const pool = prizes?.prizePool ?? prizes?.prizes ?? [];
  const totalWeight = pool.reduce((sum, item) => sum + Number(item.weight || 0), 0);
  const rows = pool.map((row) => [
    row.label || (row.rank === 'jackpot' ? '大奖' : row.rank === 'second' ? '二等奖' : row.rank === 'third' ? '三等奖' : row.type === 'miss' ? '未中奖' : `$${row.dollars ?? 0}`),
    row.weight ?? 0,
    totalWeight,
    `${Math.round(Number(row.weight || 0) / Math.max(1, totalWeight) * 10000) / 100}%`
  ]);
  return (
    <div className="business-stack">
      <div className="metrics">
        <Metric label="当前奖池" value={prizes?.poolBalance ?? 0} />
      </div>
      <DataTable columns={['统一奖项', '权重', '总权重', '理论概率']} rows={rows} empty={prizes ? '暂无普通奖项。' : '正在加载当前奖池。'} />
    </div>
  );
}

export function ActivityConfigEditor({
  activity,
  stats,
  salePlans = [],
  onChange
}: {
  activity: ActivityConfig;
  stats?: ActivityStats;
  salePlans?: SaleCardPlan[];
  onChange: (activity: ActivityConfig) => void;
}) {
  const [gameConfigs, setGameConfigs] = useState<ActivityGameConfig[]>(() => normalizeGameConfigs(activity.gameConfigs, activity));
  const [scratch, setScratch] = useState<number[]>(() => (activity.scratchRewards ?? []).map(Number));
  const [gameRoutes, setGameRoutes] = useState<ActivityGameRoute[]>(() => gameRoutesFromActivity(activity));
  const [dynamicPool, setDynamicPool] = useState<DynamicPrizePoolConfig>(() => normalizeDynamicPrizePool(activity.dynamicPrizePool));
  const pushedRef = useRef<ActivityConfig | null>(null);
  const saleTierOptions = buildSaleCardTierOptions(salePlans);

  useEffect(() => {
    if (activity === pushedRef.current) return;
    setGameConfigs(normalizeGameConfigs(activity.gameConfigs, activity));
    setScratch((activity.scratchRewards ?? []).map(Number));
    setGameRoutes(gameRoutesFromActivity(activity));
    setDynamicPool(normalizeDynamicPrizePool(activity.dynamicPrizePool));
  }, [activity]);

  function emit(next: ActivityConfig) {
    pushedRef.current = next;
    onChange(next);
  }
  const emitScratch = (values: number[]) => { setScratch(values); emit({ ...activity, scratchRewards: values }); };
  const emitGameConfigs = (values: ActivityGameConfig[]) => {
    const normalized = normalizeGameConfigs(values, activity);
    const slot = normalized.find((item) => normalizeGameMode(item.game) === 'slot');
    setGameConfigs(normalized);
    emit(stripComputedExpectedValues({
      ...activity,
      gameConfigs: normalized,
      targetExpectedValue: slot?.targetExpectedValue
    }));
  };
  const emitGameRoutes = (values: ActivityGameRoute[]) => {
    const routes = normalizeGameRoutes(values);
    setGameRoutes(routes);
    emit({ ...activity, gameRoutes: routes, scratchTiers: scratchTiersFromGameRoutes(routes) });
  };
  const emitDynamicPool = (value: DynamicPrizePoolConfig) => {
    const normalized = normalizeDynamicPrizePool(value);
    setDynamicPool(normalized);
    emit({ ...activity, dynamicPrizePool: normalized });
  };
  const patchDynamicPool = (patch: Partial<DynamicPrizePoolConfig>) => emitDynamicPool({ ...dynamicPool, ...patch });
  const updateGameForTier = (quota: number, game: GameMode) => emitGameRoutes(upsertGameRoute(gameRoutes, quota, { game }));
  const updateDrawCountForTier = (quota: number, drawCount: number) => emitGameRoutes(upsertGameRoute(gameRoutes, quota, { drawCount }));
  const routeForTier = (quota: number): ActivityGameRoute | undefined => gameRoutes.find((route) => numberValue(route.dollars) === quota);
  const gameForTier = (quota: number): GameMode => normalizeGameMode(routeForTier(quota)?.game);
  const drawCountForTier = (quota: number): number => {
    const route = routeForTier(quota);
    const explicit = numberValue(route?.drawCount);
    if (explicit > 0) return explicit;
    const game = normalizeGameMode(route?.game);
    const slotCount = spinMapDrawCount(activity, quota);
    if (game === 'slot' && slotCount > 0) return slotCount;
    return 1;
  };
  const updateWindow = (patch: ActivityConfig) => emit({ ...activity, ...patch });
  const startDateValue = activityDateTimeValue(activity.startText, activity.startTS);
  const endDateValue = activityDateTimeValue(activity.endText, activity.endTS);
  const metricConfig = gameConfigs.find((item) => normalizeGameMode(item.game) === 'slot') ?? gameConfigs[0];
  const actualExpectedValue = stats?.expectedValue;
  const awardRateTotal = numberValue(dynamicPool.jackpotRate) + numberValue(dynamicPool.secondRate) + numberValue(dynamicPool.thirdRate);

  return (
    <div className="business-stack">
      <div className="metrics">
        <Metric label="目标期望值" value={metricConfig?.targetExpectedValue} />
        <Metric label="实际期望值" value={actualExpectedValue} />
      </div>
      <div className="config-subhead">活动窗口</div>
      <div className="field-grid activity-window-grid">
        <label className="field">开始时间<Input className="blueprint-input blueprint-date-input" type="datetime-local" step={1} value={startDateValue} onChange={(event) => updateWindow(activityDatePatch('start', event.target.value))} /></label>
        <label className="field">结束时间<Input className="blueprint-input blueprint-date-input" type="datetime-local" step={1} value={endDateValue} onChange={(event) => updateWindow(activityDatePatch('end', event.target.value))} /></label>
      </div>

      <div className="config-subhead">玩法参数</div>
      <div className="game-route-editor">
        <div className="game-route-row game-route-row--head game-config-row"><span>玩法</span><span>目标期望值</span></div>
        {gameConfigs.map((item) => {
          const game = normalizeGameMode(item.game);
          const update = (patch: Partial<ActivityGameConfig>) => emitGameConfigs(patchGameConfig(gameConfigs, game, patch));
          return (
            <div className="game-route-row game-config-row" key={game}>
              <strong>{gameModeLabel(game)}</strong>
              <Input className="mini-input blueprint-input" type="number" step="0.0001" min={0} value={String(item.targetExpectedValue ?? '')} aria-label={`${gameModeLabel(game)}目标期望值`} onChange={(event) => update({ targetExpectedValue: Number(event.target.value) })} />
            </div>
          );
        })}
      </div>

      <div className="config-subhead">卡档配置（来自 MCY 上架配置）</div>
      <p className="inline-help">卡档读取自动补卡里的 MCY 套餐配置；这里维护每个卡档的玩法、抽奖次数、售价和成本。</p>
      <div className="game-route-editor">
        <div className="game-route-row game-route-row--head"><span>MCY 卡档</span><span>玩法</span><span>抽奖次数</span><span>售价</span><span>成本</span><span>净利润</span><span>入池</span></div>
        {saleTierOptions.length === 0 ? <p className="inline-help">未加载到 MCY 卡档配置，先检查自动补卡配置。</p> : null}
        {saleTierOptions.map((option) => {
          const tier = dynamicTierForQuota(dynamicPool, option.quota);
          const revenue = numberValue(tier.revenue);
          const cost = numberValue(tier.cost);
          const profit = Math.max(0, revenue - cost);
          const contribution = profit * numberValue(dynamicPool.contributionRate);
          const updatePoolTier = (patch: Partial<DynamicPrizePoolTier>) => emitDynamicPool(upsertDynamicTier(dynamicPool, option.quota, patch));
          return (
            <div className="game-route-row" key={option.quota}>
              <div className="game-route-main">
                <strong>{option.label}</strong>
                <span>{option.planText}</span>
              </div>
              <select className="native-select game-mode-select" value={gameForTier(option.quota)} onChange={(event) => updateGameForTier(option.quota, normalizeGameMode(event.target.value))}>
                {GAME_MODE_OPTIONS.map((mode) => <option key={mode.value} value={mode.value}>{mode.label}</option>)}
              </select>
              <Input className="mini-input blueprint-input game-route-spin" type="number" min={1} value={String(drawCountForTier(option.quota))} aria-label={`${option.label}抽奖次数`} onChange={(event) => updateDrawCountForTier(option.quota, Number(event.target.value))} />
              <Input className="mini-input blueprint-input game-route-money" type="number" step="0.01" min={0} value={String(revenue || '')} aria-label={`${option.label}售价`} onChange={(event) => updatePoolTier({ revenue: Number(event.target.value) })} />
              <Input className="mini-input blueprint-input game-route-money" type="number" step="0.01" min={0} value={String(cost || '')} aria-label={`${option.label}成本`} onChange={(event) => updatePoolTier({ cost: Number(event.target.value) })} />
              <span className="game-route-meta">{profit.toFixed(2)}</span>
              <span className="game-route-meta">{contribution.toFixed(2)}</span>
            </div>
          );
        })}
      </div>

      <div className="config-subhead">动态奖池</div>
      <div className="dynamic-pool-panel">
        <p className="inline-help dynamic-pool-help">比例按百分比填写，支持 0.01%；0.10 表示 0.10%。卡密核销时按净利润入池，当前奖池余额按奖项分配比例展示。</p>
        <div className="dynamic-pool-controls">
          <label className="field dynamic-pool-toggle">
            <span>奖池开关</span>
            <span className="dynamic-toggle-control"><input type="checkbox" checked={Boolean(dynamicPool.enabled)} onChange={(event) => patchDynamicPool({ enabled: event.target.checked })} /><span>启用动态奖池</span></span>
          </label>
          <label className="field dynamic-rate-field">入池比例 (%)<Input className="mini-input blueprint-input" type="number" step="0.01" min={0} max={100} value={rateToPercentInput(dynamicPool.contributionRate)} onChange={(event) => patchDynamicPool({ contributionRate: percentInputToRate(event.target.value) })} /></label>
          <label className="field dynamic-rate-field">大奖分配 (%)<Input className="mini-input blueprint-input" type="number" step="0.01" min={0} max={100} value={rateToPercentInput(dynamicPool.jackpotRate)} onChange={(event) => patchDynamicPool({ jackpotRate: percentInputToRate(event.target.value) })} /></label>
          <label className="field dynamic-rate-field">二奖分配 (%)<Input className="mini-input blueprint-input" type="number" step="0.01" min={0} max={100} value={rateToPercentInput(dynamicPool.secondRate)} onChange={(event) => patchDynamicPool({ secondRate: percentInputToRate(event.target.value) })} /></label>
          <label className="field dynamic-rate-field">三奖分配 (%)<Input className="mini-input blueprint-input" type="number" step="0.01" min={0} max={100} value={rateToPercentInput(dynamicPool.thirdRate)} onChange={(event) => patchDynamicPool({ thirdRate: percentInputToRate(event.target.value) })} /></label>
        </div>
        <div className="dynamic-pool-note">奖项分配合计 {rateToPercentInput(awardRateTotal)}%；入池比例用于计算每张卡贡献多少利润。</div>
      </div>

      <div className="config-subhead">刮刮卡奖励（$）</div>
      <div className="scratch-editor">
        {scratch.map((value, index) => (
          <div className="scratch-chip" key={index}>
            <Input className="mini-input blueprint-input" type="number" min={0} value={String(value)} onChange={(event) => emitScratch(scratch.map((v, i) => (i === index ? Number(event.target.value) : v)))} />
            <Button className="blueprint-danger-button" onPress={() => emitScratch(scratch.filter((_, i) => i !== index))}>×</Button>
          </div>
        ))}
        <Button className="blueprint-button" onPress={() => emitScratch([...scratch, 0])}>新增奖励</Button>
      </div>
    </div>
  );
}
