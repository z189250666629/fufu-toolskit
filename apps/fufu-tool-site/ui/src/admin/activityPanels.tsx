import { useEffect, useRef, useState } from 'react';
import { Button, Input } from '@heroui/react';
import { messageFromError } from '../api';
import { MessageLine } from '../blueprint';
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
import type { ActivityConfig, ActivityGameConfig, ActivityGameRoute, ActivityStats, DynamicPrizePoolConfig, DynamicPrizePoolTier, PrizeConfigResponse, SaleCardPlan, SaleCardTestKeyResult } from './types';

type LocalMessage = {
  text: string;
  tone?: 'ok' | 'error';
};

const SCRATCH_FIXED_STEPS = 6;

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
  const scratchRewards = (prizes?.scratchRewards ?? []).map(Number);
  const rows = pool.map((row) => [
    row.label || (row.rank === 'jackpot' ? '大奖' : row.rank === 'second' ? '二等奖' : row.rank === 'third' ? '三等奖' : row.type === 'miss' ? '未中奖' : `$${row.dollars ?? 0}`),
    row.weight ?? 0,
    totalWeight,
    `${Math.round(Number(row.weight || 0) / Math.max(1, totalWeight) * 10000) / 100}%`
  ]);
  return (
    <div className="business-stack">
      <div className="metrics">
        <Metric label="老虎机奖池" value={prizes?.poolBalance ?? 0} />
        <Metric label="刮刮乐奖池" value={prizes?.scratchPoolBalance ?? 0} />
      </div>
      <DataTable columns={['统一奖项', '权重', '总权重', '理论概率']} rows={rows} empty={prizes ? '暂无普通奖项。' : '正在加载当前奖池。'} />
      <DataTable
        columns={['刮开步数', '动态奖励']}
        rows={scratchRewards.map((value, index) => [`第 ${index + 1} 步`, `$${value}`])}
        empty={prizes ? '暂无刮刮乐动态奖励。' : '正在加载刮刮乐奖池。'}
      />
    </div>
  );
}

export function ActivityConfigEditor({
  activity,
  stats,
  salePlans = [],
  onGenerateTestKey,
  onChange
}: {
  activity: ActivityConfig;
  stats?: ActivityStats;
  salePlans?: SaleCardPlan[];
  onGenerateTestKey?: (plan: string, count: number) => Promise<SaleCardTestKeyResult>;
  onChange: (activity: ActivityConfig) => void;
}) {
  const [gameConfigs, setGameConfigs] = useState<ActivityGameConfig[]>(() => normalizeGameConfigs(activity.gameConfigs, activity));
  const [gameRoutes, setGameRoutes] = useState<ActivityGameRoute[]>(() => gameRoutesFromActivity(activity));
  const [dynamicPool, setDynamicPool] = useState<DynamicPrizePoolConfig>(() => normalizeDynamicPrizePool(activity.dynamicPrizePool));
  const [testKeyCount, setTestKeyCount] = useState(1);
  const [generatingTestPlan, setGeneratingTestPlan] = useState('');
  const [testKeyResult, setTestKeyResult] = useState<SaleCardTestKeyResult>();
  const [testKeyMessage, setTestKeyMessage] = useState<LocalMessage>({ text: '' });
  const pushedRef = useRef<ActivityConfig | null>(null);
  const saleTierOptions = buildSaleCardTierOptions(salePlans);

  useEffect(() => {
    if (activity === pushedRef.current) return;
    setGameConfigs(normalizeGameConfigs(activity.gameConfigs, activity));
    setGameRoutes(gameRoutesFromActivity(activity));
    setDynamicPool(normalizeDynamicPrizePool(activity.dynamicPrizePool));
  }, [activity]);

  function emit(next: ActivityConfig) {
    pushedRef.current = next;
    onChange(next);
  }
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
  const updateTestKeyCount = (value: string) => {
    const next = Math.max(1, Math.min(20, Math.floor(Number(value) || 1)));
    setTestKeyCount(next);
  };
  const generateTestKeyForTier = async (planID?: string) => {
    const plan = String(planID ?? '').trim();
    if (!plan || !onGenerateTestKey) return;
    setGeneratingTestPlan(plan);
    setTestKeyMessage({ text: '正在生成测试 key…' });
    try {
      const result = await onGenerateTestKey(plan, testKeyCount);
      setTestKeyResult(result);
      setTestKeyMessage({ text: `已生成 ${result.generated ?? result.keys?.length ?? 0} 个测试 key`, tone: 'ok' });
    } catch (error) {
      setTestKeyMessage({ text: messageFromError(error, '测试 key 生成失败'), tone: 'error' });
    } finally {
      setGeneratingTestPlan('');
    }
  };
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
      <p className="inline-help">卡档读取补卡卡档配置；这里维护每个卡档的玩法、抽奖次数、售价和成本。</p>
      <div className="sale-test-key-toolbar">
        <label className="field field--inline">测试数量<Input className="mini-input blueprint-input sale-test-key-count" type="number" min={1} max={20} value={String(testKeyCount)} onChange={(event) => updateTestKeyCount(event.target.value)} /></label>
        <span className="inline-help">测试 key 只创建 NewAPI token，不上传 MCY；token 名会带活动测试标记。</span>
      </div>
      <div className="game-route-editor">
        <div className="game-route-row game-route-row--head"><span>MCY 卡档</span><span>玩法</span><span>抽奖次数</span><span>售价</span><span>成本</span><span>净利润</span><span>入池</span><span>测试 key</span></div>
        {saleTierOptions.length === 0 ? <p className="inline-help">未加载到 MCY 卡档配置，先检查补卡卡档配置。</p> : null}
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
              <Button className="blueprint-button game-route-test-button" isDisabled={!option.primaryPlanId || !onGenerateTestKey || generatingTestPlan === option.primaryPlanId} onPress={() => { void generateTestKeyForTier(option.primaryPlanId); }}>
                {generatingTestPlan === option.primaryPlanId ? '生成中…' : '一键生成 key'}
              </Button>
            </div>
          );
        })}
      </div>
      {testKeyMessage.text ? <MessageLine tone={testKeyMessage.tone}>{testKeyMessage.text}</MessageLine> : null}
      {testKeyResult ? (
        <div className="sale-test-key-result">
          <div className="sale-test-key-result-head">
            <strong>{testKeyResult.planName || testKeyResult.planId || '测试 key'}</strong>
            <span>{gameModeLabel(testKeyResult.game || 'slot')} · {testKeyResult.drawCount ?? '-'} 次</span>
          </div>
          <div className="sale-test-key-list">
            {(testKeyResult.keys ?? []).map((key, index) => <code key={`${key}-${index}`}>{key}</code>)}
          </div>
        </div>
      ) : null}

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

      <div className="config-subhead">刮刮乐动态奖励</div>
      <div className="scratch-editor scratch-editor--readonly">
        <p className="inline-help">
          刮刮乐使用独立奖池实时计算奖励，后台只固定 {SCRATCH_FIXED_STEPS} 个安全步数；具体金额请在“当前奖池中奖率”里查看。
        </p>
        <div className="scratch-step-list" aria-label="刮刮乐固定步数">
          {Array.from({ length: SCRATCH_FIXED_STEPS }, (_, index) => (
            <span className="scratch-chip scratch-chip--static" key={index}>第 {index + 1} 步</span>
          ))}
        </div>
      </div>
    </div>
  );
}
