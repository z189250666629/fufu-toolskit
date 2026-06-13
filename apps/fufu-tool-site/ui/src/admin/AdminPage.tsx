import { FormEvent, useEffect, useMemo, useState } from 'react';
import {
  Button,
  Card,
  Input,
  Table,
  TextArea as Textarea,
  Tabs
} from '@heroui/react';
import { fetchJSON, messageFromError, sendJSON } from '../api';
import { formatNumber, MessageLine, NavPill, ThemeToggle, TopActions, BlueprintHeader } from '../blueprint';
import type {
  ActivityConfig,
  ActivityStats,
  AdminConfig,
  AdminSession,
  ManagedSite,
  ManagedSiteURL,
  PrizeConfigResponse,
  RuntimeSitesResponse,
  SaleCardConfig,
  SaleCardRunResult,
  SaleCardStockResponse
} from './types';

type MessageState = {
  text: string;
  tone?: 'ok' | 'error' | '';
};

const emptyConfig: AdminConfig = {
  newapi: { sites: [] },
  activity: {}
};

const defaultSite: ManagedSite = {
  name: '',
  category: 'api',
  urls: [],
  token: '',
  userId: '1',
  kind: 'api',
  skipUserHeader: false,
  quotaUnit: 500000,
  currency: '$',
  rechargeRatio: 1,
  channelListEndpoint: '',
  note: ''
};

function jsonPretty(value: unknown) {
  return JSON.stringify(value ?? {}, null, 2);
}

function parseJSONField<T>(value: string, fallback: T, label: string): T {
  const trimmed = value.trim();
  if (!trimmed) return fallback;
  try {
    return JSON.parse(trimmed) as T;
  } catch (error) {
    throw new Error(`${label} 不是有效 JSON：${error instanceof Error ? error.message : '格式错误'}`);
  }
}

function Metric({ label, value }: { label: string; value: unknown }) {
  return (
    <div className="bp-kpi">
      <span className="bp-kpi-label">{label}</span>
      <span className="bp-kpi-num">{formatNumber(value)}</span>
    </div>
  );
}

function DataTable({
  columns,
  rows,
  empty = '暂无数据'
}: {
  columns: string[];
  rows: Array<Array<unknown>>;
  empty?: string;
}) {
  return (
    <Table className="blueprint-table">
      <div className="table-wrap">
        <table>
          <thead>
            <tr>{columns.map((column) => <th key={column}>{column}</th>)}</tr>
          </thead>
          <tbody>
            {rows.length ? rows.map((row, rowIndex) => (
              <tr key={rowIndex}>
                {row.map((cell, cellIndex) => <td key={cellIndex}>{String(cell ?? '-')}</td>)}
              </tr>
            )) : (
              <tr><td colSpan={columns.length}><span className="inline-help">{empty}</span></td></tr>
            )}
          </tbody>
        </table>
      </div>
    </Table>
  );
}

function ConfigCard({
  title,
  description,
  action,
  children
}: {
  title: string;
  description?: string;
  action?: React.ReactNode;
  children: React.ReactNode;
}) {
  return (
    <section className="bp-card admin-panel">
      <header className="bp-card-titlebar">
        <span>{title}</span>
        {action}
      </header>
      <div className="bp-card-body">
        {description ? <p className="bp-card-desc">{description}</p> : null}
        {children}
      </div>
    </section>
  );
}

function LoginPanel({ onLogin, busy }: { onLogin: (token: string) => Promise<void>; busy: boolean }) {
  const [token, setToken] = useState('');

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    await onLogin(token);
  }

  return (
    <Card className="auth-panel fade-in">
      <Card.Header>
        <div>
          <Card.Title>管理员登录</Card.Title>
          <Card.Description>先完成登录验证，再展示完整管理面板。口令只提交给本机后台验证。</Card.Description>
        </div>
      </Card.Header>
      <Card.Content>
        <form className="auth-form" onSubmit={submit}>
          <Input
            type="password"
            autoComplete="current-password"
            placeholder="输入管理员口令"
            value={token}
            onChange={(event) => setToken(event.target.value)}
            className="blueprint-input"
          />
          <Button type="submit" className="blueprint-primary-button" isDisabled={busy}>
            {busy ? '验证中' : '登录后台'}
          </Button>
        </form>
      </Card.Content>
    </Card>
  );
}

const SITE_GROUPS: { category: string; title: string; desc: string }[] = [
  { category: 'api', title: 'API 次数站', desc: '一个站点：配置一次 access token，可加多条 base_url。合卡默认复用第一条线路。' },
  { category: 'token', title: 'Token 站', desc: '一个站点：配置一次 access token，可加多条 base_url。' }
];

function SiteEditor({
  sites,
  onChange
}: {
  sites: ManagedSite[];
  onChange: (sites: ManagedSite[]) => void;
}) {
  const categoryOf = (site: ManagedSite) => site.category || 'api';
  const indexOf = (category: string) => sites.findIndex((site) => categoryOf(site) === category);
  const siteFor = (category: string) => sites.find((site) => categoryOf(site) === category);

  // Ensure exactly one site exists for a category, returning the working list +
  // its index. A site = one token + a list of base_url lines.
  function ensureSite(category: string): [ManagedSite[], number] {
    const idx = indexOf(category);
    if (idx >= 0) return [sites, idx];
    const name = category === 'token' ? 'token-fufu' : '次数fufu';
    const next = [...sites, { ...defaultSite, category, name, urls: [] }];
    return [next, next.length - 1];
  }
  function setCategoryToken(category: string, value: string) {
    const [base, idx] = ensureSite(category);
    onChange(base.map((site, i) => (i === idx ? { ...site, token: value } : site)));
  }
  function addUrl(category: string) {
    const [base, idx] = ensureSite(category);
    const urls = base[idx].urls ?? [];
    onChange(base.map((site, i) => (i === idx ? { ...site, urls: [...urls, { name: `线路 ${urls.length + 1}`, url: '' }] } : site)));
  }
  function updateUrl(category: string, urlIndex: number, patch: Partial<ManagedSiteURL>) {
    const idx = indexOf(category);
    if (idx < 0) return;
    onChange(sites.map((site, i) => (i === idx ? { ...site, urls: (site.urls ?? []).map((u, ui) => (ui === urlIndex ? { ...u, ...patch } : u)) } : site)));
  }
  function removeUrl(category: string, urlIndex: number) {
    const idx = indexOf(category);
    if (idx < 0) return;
    onChange(sites.map((site, i) => (i === idx ? { ...site, urls: (site.urls ?? []).filter((_, ui) => ui !== urlIndex) } : site)));
  }

  return (
    <div className="site-groups">
      {SITE_GROUPS.map((group) => {
        const site = siteFor(group.category);
        const urls = site?.urls ?? [];
        return (
          <section key={group.category} className="bp-card">
            <header className="bp-card-titlebar">
              <span>{group.title}</span>
              <Button className="blueprint-button" onPress={() => addUrl(group.category)}>新增 URL</Button>
            </header>
            <div className="bp-card-body">
              <p className="bp-card-desc">{group.desc}</p>
              <label className="field site-token-field">
                access token（整站配置一次）
                <Input
                  className="blueprint-input"
                  type="password"
                  value={site?.token || ''}
                  placeholder={site?.tokenMasked || '留空表示沿用原 token'}
                  onChange={(event) => setCategoryToken(group.category, event.target.value)}
                />
              </label>
              <div className="config-subhead">base_url 线路</div>
              {urls.length === 0 ? <p className="inline-help">还没有 base_url，点“新增 URL”添加。</p> : null}
              <div className="site-url-list">
                {urls.map((line, urlIndex) => (
                  <div key={urlIndex} className="site-url-row">
                    <span className="site-url-tag">
                      {urlIndex + 1}
                      {group.category === 'api' && urlIndex === 0 ? ' · 合卡' : ''}
                    </span>
                    <Input
                      className="blueprint-input site-url-name"
                      value={line.name || ''}
                      placeholder={`线路 ${urlIndex + 1}`}
                      onChange={(event) => updateUrl(group.category, urlIndex, { name: event.target.value })}
                    />
                    <Input
                      className="blueprint-input"
                      value={line.url || ''}
                      placeholder="https://api.example.com"
                      onChange={(event) => updateUrl(group.category, urlIndex, { url: event.target.value })}
                    />
                    <Button className="blueprint-danger-button" onPress={() => removeUrl(group.category, urlIndex)}>删除</Button>
                  </div>
                ))}
              </div>
            </div>
          </section>
        );
      })}
      <p className="inline-help">每个站点仅配置一次 access token，可添加多条 base_url（线路名用于首页展示）；首页只明文展示 URL，token 仅后台可见。Token 留空表示沿用原值。</p>
    </div>
  );
}

function RuntimeSites({ sites }: { sites?: RuntimeSitesResponse }) {
  const rows = (sites?.sites ?? []).map((site, index) => [
    site.name || '-',
    site.displayUrl || site.url || '地址已隐藏',
    site.userId || '-',
    index === 0 ? '是' : '否'
  ]);
  return <DataTable columns={['站点', '显示地址', 'User ID', '合卡复用']} rows={rows} empty="登录后加载状态页站点。" />;
}

const SALE_SLOTS: { group: string; label: string; defaultTime: string }[] = [
  { group: 'special55', label: '55 次混合特惠卡', defaultTime: '09:00' },
  { group: 'month', label: '月次卡', defaultTime: '09:30' }
];

type SlotState = { time: string; enabled: boolean; jobs: Record<string, { targetStock: number; enabled: boolean }> };

function buildSlotState(config?: SaleCardConfig): Record<string, SlotState> {
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

function SaleCardManager({
  config,
  onSave,
  onRun
}: {
  config?: SaleCardConfig;
  onSave: (schedule: NonNullable<SaleCardConfig['schedule']>) => Promise<void>;
  onRun: (plan: string, targetStock: number) => Promise<SaleCardRunResult | undefined>;
}) {
  const plans = config?.plans ?? [];
  const [enabled, setEnabled] = useState(Boolean(config?.schedule?.enabled));
  const [timezone, setTimezone] = useState(config?.schedule?.timezone || 'Asia/Shanghai');
  const [slotState, setSlotState] = useState<Record<string, SlotState>>(() => buildSlotState(config));
  const [runPlan, setRunPlan] = useState(plans[0]?.id ?? '');
  const [runTarget, setRunTarget] = useState(50);
  const [runResult, setRunResult] = useState<SaleCardRunResult>();
  const [stock, setStock] = useState<Record<string, number>>({});
  const [refreshing, setRefreshing] = useState(false);

  useEffect(() => {
    setEnabled(Boolean(config?.schedule?.enabled));
    setTimezone(config?.schedule?.timezone || 'Asia/Shanghai');
    setSlotState(buildSlotState(config));
    setRunPlan((config?.plans ?? [])[0]?.id ?? '');
  }, [config]);

  async function refreshStock() {
    setRefreshing(true);
    try {
      const data = await fetchJSON<SaleCardStockResponse>('/api/admin/sale-cards/stock');
      const map: Record<string, number> = {};
      for (const entry of data.stock ?? []) map[entry.planId] = entry.currentStock;
      setStock(map);
    } catch {
      // leave previous counts; admin can retry
    } finally {
      setRefreshing(false);
    }
  }
  const stockText = (planId: string) => (planId in stock ? String(stock[planId]) : '—');

  function patchSlot(group: string, patch: Partial<Omit<SlotState, 'jobs'>>) {
    setSlotState((current) => ({ ...current, [group]: { ...current[group], ...patch } }));
  }
  function patchJob(group: string, planId: string, patch: Partial<{ targetStock: number; enabled: boolean }>) {
    setSlotState((current) => {
      const slot = current[group];
      const job = slot.jobs[planId] ?? { targetStock: 0, enabled: false };
      return { ...current, [group]: { ...slot, jobs: { ...slot.jobs, [planId]: { ...job, ...patch } } } };
    });
  }
  function buildSchedule(): NonNullable<SaleCardConfig['schedule']> {
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

  async function runNow() {
    const result = await onRun(runPlan, runTarget);
    if (result) setRunResult(result);
  }

  return (
    <div className="business-stack">
      <div className="action-row">
        <label className="field--inline"><input type="checkbox" checked={enabled} onChange={(event) => setEnabled(event.target.checked)} /> 启用自动补卡</label>
        <label className="field--inline field--inline-tz">时区<Input className="mini-input blueprint-input" value={timezone} onChange={(event) => setTimezone(event.target.value)} aria-label="时区" /></label>
        <Button className="blueprint-button" onPress={refreshStock} isDisabled={refreshing}>{refreshing ? '刷新中…' : '刷新当前库存'}</Button>
        <Button className="blueprint-primary-button" onPress={() => onSave(buildSchedule())}>保存补卡计划</Button>
      </div>
      <p className="inline-help">月次卡与 55 次混合特惠卡各占一个独立时段；到点按目标库存补齐（补 目标-当前）。点“刷新当前库存”查询 NewAPI 实时卡量。</p>
      {SALE_SLOTS.map((def) => {
        const slot = slotState[def.group];
        const slotPlans = plans.filter((plan) => (plan.slot || '') === def.group);
        if (!slot) return null;
        return (
          <section key={def.group} className="bp-card sale-slot-card">
            <header className="bp-card-titlebar">
              <span>{def.label} 时段</span>
              <div className="sale-slot-controls">
                <Input className="mini-input blueprint-input sale-time-input" type="time" value={slot.time} onChange={(event) => patchSlot(def.group, { time: event.target.value })} aria-label={`${def.label}补卡时间`} />
                <label className="field--inline"><input type="checkbox" checked={slot.enabled} onChange={(event) => patchSlot(def.group, { enabled: event.target.checked })} /> 启用时段</label>
              </div>
            </header>
            <div className="bp-card-body">
              <div className="sale-job-list">
                {slotPlans.length ? slotPlans.map((plan) => {
                  const job = slot.jobs[plan.id] ?? { targetStock: 0, enabled: false };
                  return (
                    <div className="sale-job-row" key={plan.id}>
                      <span className="sale-job-name">{plan.name || plan.id}<span className="sale-job-meta">SKU {plan.skuId || '-'} · ${plan.quota ?? '-'}</span></span>
                      <span className="sale-job-current">当前 <b>{stockText(plan.id)}</b></span>
                      <label className="field--inline sale-job-target">补齐到<Input className="mini-input blueprint-input" type="number" min={0} max={2000} value={String(job.targetStock || '')} onChange={(event) => patchJob(def.group, plan.id, { targetStock: Number(event.target.value) })} aria-label={`${plan.name || plan.id}目标库存`} />张</label>
                      <label className="field--inline"><input type="checkbox" checked={job.enabled} onChange={(event) => patchJob(def.group, plan.id, { enabled: event.target.checked })} /> 启用</label>
                    </div>
                  );
                }) : <p className="inline-help">该时段暂无计划。</p>}
              </div>
            </div>
          </section>
        );
      })}
      <div className="config-subhead">立即补卡（手动）</div>
      <div className="action-row">
        <select className="native-select" value={runPlan} onChange={(event) => setRunPlan(event.target.value)}>
          {plans.length ? plans.map((plan) => <option key={plan.id} value={plan.id}>{plan.name || plan.id}</option>) : <option value="">暂无计划</option>}
        </select>
        <span className="sale-job-current">当前 <b>{stockText(runPlan)}</b></span>
        <label className="field--inline">补齐到<Input className="mini-input blueprint-input" type="number" min={0} max={2000} value={String(runTarget)} onChange={(event) => setRunTarget(Number(event.target.value))} aria-label="目标库存" />张</label>
        <Button className="blueprint-primary-button" onPress={runNow} isDisabled={!runPlan}>立即补卡</Button>
      </div>
      {runResult ? (
        <div className="metrics sale-run-result">
          <Metric label="当前库存" value={runResult.currentStock} />
          <Metric label="目标库存" value={runResult.targetStock} />
          <Metric label="本次补卡" value={runResult.toUpload} />
          <Metric label="已上架" value={runResult.uploaded} />
        </div>
      ) : null}
    </div>
  );
}

function ActivityStatsPanel({ stats }: { stats?: ActivityStats }) {
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
        empty="登录后加载活动统计。"
      />
      <DataTable
        columns={['面额', '卡数', '已用/总次', '总中奖']}
        rows={tierRows.map((row) => [row.tier || row.quota || '-', row.cards || row.count, `${row.used ?? '-'}/${row.spins ?? row.totalSpins ?? '-'}`, row.totalWon ?? '-'])}
        empty="暂无分面额统计。"
      />
    </div>
  );
}

function CurrentPrizePanel({ prizes }: { prizes?: PrizeConfigResponse }) {
  const pool = prizes?.prizePool ?? [];
  const rows = pool.map((row) => [
    row.type === 'miss' ? '未中奖' : `$${row.dollars ?? 0}`,
    row.weight ?? 0,
    pool.reduce((sum, item) => sum + Number(item.weight || 0), 0),
    `${Math.round(Number(row.weight || 0) / Math.max(1, pool.reduce((sum, item) => sum + Number(item.weight || 0), 0)) * 10000) / 100}%`
  ]);
  const tierPools = prizes?.tierPools ?? {};
  return (
    <div className="business-stack">
      <DataTable columns={['普通奖项', '权重', '总权重', '理论概率']} rows={rows} empty="等待加载当前奖池。" />
      <DataTable
        columns={['额度', '中奖项数量', '详情']}
        rows={Object.entries(tierPools).map(([tier, poolRows]) => [
          `$${tier}`,
          Array.isArray(poolRows) ? poolRows.length : 0,
          Array.isArray(poolRows) ? poolRows.map((row) => `$${row.dollars ?? 0}:${row.weight ?? 0}`).join(' / ') : '-'
        ])}
        empty="暂无分额度奖池"
      />
    </div>
  );
}

function ActivityConfigEditor({
  activity,
  onChange
}: {
  activity: ActivityConfig;
  onChange: (activity: ActivityConfig) => void;
}) {
  const [spinMap, setSpinMap] = useState(jsonPretty(activity.spinMap ?? {}));
  const [prizePool, setPrizePool] = useState(jsonPretty(activity.prizePool ?? []));
  const [tierPools, setTierPools] = useState(jsonPretty(activity.tierPools ?? {}));
  const [postJackpotPrizes, setPostJackpotPrizes] = useState(jsonPretty(activity.postJackpotPrizes ?? []));
  const [scratchRewards, setScratchRewards] = useState(jsonPretty(activity.scratchRewards ?? []));

  useEffect(() => {
    setSpinMap(jsonPretty(activity.spinMap ?? {}));
    setPrizePool(jsonPretty(activity.prizePool ?? []));
    setTierPools(jsonPretty(activity.tierPools ?? {}));
    setPostJackpotPrizes(jsonPretty(activity.postJackpotPrizes ?? []));
    setScratchRewards(jsonPretty(activity.scratchRewards ?? []));
  }, [activity]);

  function update(patch: ActivityConfig) {
    onChange({ ...activity, ...patch });
  }

  function applyJSON() {
    update({
      spinMap: parseJSONField<Record<string, number>>(spinMap, {}, '额度 → 抽奖次数'),
      prizePool: parseJSONField(prizePool, [], '普通奖池权重（调整中奖率）'),
      tierPools: parseJSONField(tierPools, {}, '分额度奖池权重'),
      postJackpotPrizes: parseJSONField(postJackpotPrizes, [], '中大奖后奖池权重'),
      scratchRewards: parseJSONField(scratchRewards, [], '刮刮卡奖励')
    });
  }

  return (
    <div className="business-stack">
      <div className="metrics">
        <Metric label="目标期望值" value={activity.targetExpectedValue} />
        <Metric label="实际期望值" value={activity.actualExpectedValue} />
      </div>
      <div className="config-subhead">活动窗口与整体期望值</div>
      <div className="field-grid">
        <label className="field">开始时间文本<Input className="blueprint-input" value={activity.startText || ''} placeholder="2026-06-01 00:00:00" onChange={(event) => update({ startText: event.target.value })} /></label>
        <label className="field">结束时间文本<Input className="blueprint-input" value={activity.endText || ''} placeholder="2026-06-30 23:59:59" onChange={(event) => update({ endText: event.target.value })} /></label>
        <label className="field">开始时间戳<Input className="blueprint-input" type="number" min={1} value={String(activity.startTS ?? '')} onChange={(event) => update({ startTS: Number(event.target.value) })} /></label>
        <label className="field">结束时间戳<Input className="blueprint-input" type="number" min={1} value={String(activity.endTS ?? '')} onChange={(event) => update({ endTS: Number(event.target.value) })} /></label>
        <label className="field">整体数学期望值<Input className="blueprint-input" type="number" step="0.0001" min={0} value={String(activity.targetExpectedValue ?? '')} onChange={(event) => update({ targetExpectedValue: Number(event.target.value) })} /></label>
      </div>
      <div className="config-subhead">奖池权重配置 · JSON</div>
      <div className="json-grid">
        <label className="field">额度 → 抽奖次数<Textarea className="blueprint-textarea" value={spinMap} onChange={(event) => setSpinMap(event.target.value)} /></label>
        <label className="field">普通奖池权重（调整中奖率）<Textarea className="blueprint-textarea" value={prizePool} onChange={(event) => setPrizePool(event.target.value)} /></label>
        <label className="field">分额度奖池权重<Textarea className="blueprint-textarea" value={tierPools} onChange={(event) => setTierPools(event.target.value)} /></label>
        <label className="field">中大奖后奖池权重<Textarea className="blueprint-textarea" value={postJackpotPrizes} onChange={(event) => setPostJackpotPrizes(event.target.value)} /></label>
        <label className="field">刮刮卡奖励<Textarea className="blueprint-textarea" value={scratchRewards} onChange={(event) => setScratchRewards(event.target.value)} /></label>
      </div>
      <Button className="blueprint-button" onPress={applyJSON}>应用 JSON 到待保存配置</Button>
    </div>
  );
}

export function AdminPage() {
  const [authenticated, setAuthenticated] = useState(false);
  const [checking, setChecking] = useState(true);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState<MessageState>({ text: '' });
  const [config, setConfig] = useState<AdminConfig>(emptyConfig);
  const [runtimeSites, setRuntimeSites] = useState<RuntimeSitesResponse>();
  const [activityStats, setActivityStats] = useState<ActivityStats>();
  const [saleCards, setSaleCards] = useState<SaleCardConfig>();
  const [prizes, setPrizes] = useState<PrizeConfigResponse>();

  async function loadConfig() {
    const next = await fetchJSON<AdminConfig>('/api/admin/config');
    setConfig(next);
  }

  async function loadBusinessData() {
    const [stats, saleConfig, prizeConfig, sites] = await Promise.all([
      fetchJSON<ActivityStats>('/api/admin/stats').catch(() => undefined),
      fetchJSON<SaleCardConfig>('/api/admin/sale-cards/config').catch(() => undefined),
      fetchJSON<PrizeConfigResponse>('/api/prizes').catch(() => undefined),
      fetchJSON<RuntimeSitesResponse>('/api/newapi/sites').catch(() => undefined)
    ]);
    if (stats) setActivityStats(stats);
    if (saleConfig) setSaleCards(saleConfig);
    if (prizeConfig) setPrizes(prizeConfig);
    if (sites) setRuntimeSites(sites);
  }

  async function loadAll() {
    setBusy(true);
    setMessage({ text: '正在加载配置…' });
    try {
      await loadConfig();
      await loadBusinessData();
      setMessage({ text: '配置已加载', tone: 'ok' });
    } catch (error) {
      setMessage({ text: messageFromError(error), tone: 'error' });
    } finally {
      setBusy(false);
    }
  }

  useEffect(() => {
    fetchJSON<AdminSession>('/api/admin/session')
      .then((session) => {
        setAuthenticated(Boolean(session.authenticated));
        if (session.authenticated) void loadAll();
      })
      .catch(() => setAuthenticated(false))
      .finally(() => setChecking(false));
  }, []);

  async function login(token: string) {
    setBusy(true);
    setMessage({ text: '正在验证管理员登录…' });
    try {
      const session = await sendJSON<AdminSession>('/api/admin/session', 'POST', { token });
      setAuthenticated(Boolean(session.authenticated));
      setMessage({ text: '登录成功', tone: 'ok' });
      await loadAll();
    } catch (error) {
      setMessage({ text: messageFromError(error), tone: 'error' });
    } finally {
      setBusy(false);
    }
  }

  async function logout() {
    await sendJSON<AdminSession>('/api/admin/session', 'DELETE');
    setAuthenticated(false);
    setConfig(emptyConfig);
    setMessage({ text: '已退出登录', tone: 'ok' });
  }

  async function saveConfig() {
    setBusy(true);
    setMessage({ text: '正在保存全部配置…' });
    try {
      // Drop incomplete sites (no base_url) and blank url lines so emptying one
      // category never blocks saving the other (backend requires ≥1 base_url).
      const sites = config.newapi.sites
        .map((site) => ({ ...site, urls: (site.urls ?? []).filter((u) => (u.url ?? '').trim() !== '') }))
        .filter((site) => site.urls.length > 0);
      const next = await sendJSON<AdminConfig>('/api/admin/config', 'PUT', {
        newapi: { sites },
        activity: config.activity
      });
      setConfig(next);
      await loadBusinessData();
      setMessage({ text: '配置已保存并应用', tone: 'ok' });
    } catch (error) {
      setMessage({ text: messageFromError(error), tone: 'error' });
    } finally {
      setBusy(false);
    }
  }

  async function saveSaleSchedule(schedule: NonNullable<SaleCardConfig['schedule']>) {
    setMessage({ text: '正在保存补卡计划…' });
    try {
      // Backend expects the schedule wrapped: {"schedule": {...}}.
      const next = await sendJSON<SaleCardConfig>('/api/admin/sale-cards/config', 'POST', { schedule });
      setSaleCards(next);
      setMessage({ text: '补卡计划已保存', tone: 'ok' });
    } catch (error) {
      setMessage({ text: messageFromError(error), tone: 'error' });
    }
  }

  async function runSalePlan(plan: string, targetStock: number): Promise<SaleCardRunResult | undefined> {
    setMessage({ text: '正在执行补卡…' });
    try {
      const result = await sendJSON<SaleCardRunResult>('/api/admin/sale-cards/run', 'POST', { plan, targetStock });
      setMessage({ text: `补卡完成：当前 ${result.currentStock ?? 0} → 目标 ${result.targetStock ?? 0}，补 ${result.toUpload ?? 0} 张`, tone: 'ok' });
      return result;
    } catch (error) {
      setMessage({ text: messageFromError(error), tone: 'error' });
      return undefined;
    }
  }

  const siteCount = config.newapi.sites.length;
  const configSites = useMemo(() => config.newapi.sites ?? [], [config]);

  return (
    <main className="blueprint-page admin-page">
        {authenticated ? (
          <section className="admin-dashboard fade-in">
            <Tabs className="admin-tabs" defaultSelectedKey="site-replenish">
              <header className="blueprint-header blueprint-header--compact admin-header" data-label="ADMIN CONSOLE">
                <div className="admin-header-top">
                  <div className="admin-header-titles">
                    <h1 className="blueprint-title admin-title">fufu 管理后台</h1>
                    <div className="admin-doc-meta" aria-hidden="true">
                      <span className="admin-meta-actor">admin</span>
                      <span className="admin-meta-dot">·</span>
                      <span>/admin</span>
                      <span className="admin-meta-dot">·</span>
                      <span>tool-config.db</span>
                    </div>
                  </div>
                  <div className="admin-utility-bar">
                    <div className="admin-header-nav">
                      <ThemeToggle />
                      <NavPill href="/">首页</NavPill>
                    </div>
                    <span className="admin-utility-note">配置操作</span>
                    <div className="admin-action-group">
                      <Button className="blueprint-button" onPress={loadAll} isDisabled={busy}>重新加载配置</Button>
                      <Button className="blueprint-primary-button" onPress={saveConfig} isDisabled={busy}>保存全部配置</Button>
                      <Button className="blueprint-button" onPress={loadBusinessData} isDisabled={busy}>刷新业务数据</Button>
                      <Button className="blueprint-danger-button" onPress={logout}>退出登录</Button>
                    </div>
                  </div>
                </div>
                <Tabs.List className="admin-tab-list">
                  <Tabs.Tab id="site-replenish" className="admin-tab-card">
                    <span className="admin-tab-title">状态页 / 合卡 / 自动补货</span>
                  </Tabs.Tab>
                  <Tabs.Tab id="activity" className="admin-tab-card">
                    <span className="admin-tab-title">活动统计 / 奖池 / 期望值</span>
                  </Tabs.Tab>
                </Tabs.List>
              </header>
              <MessageLine tone={message.tone}>{message.text}</MessageLine>

              <Tabs.Panel id="site-replenish" className="admin-tab-panel">
                <ConfigCard
                  title="状态页实际站点"
                  description="状态页、连通性检测、模型状态和合卡都会读取这组站点。"
                  action={<Metric label="已配置站点" value={siteCount} />}
                >
                  <RuntimeSites sites={runtimeSites} />
                </ConfigCard>
                <ConfigCard
                  title="NewAPI 站点配置"
                  description="分 2 类站点（次数站 / token 站）。每个站点只配置一次 access token，可添加多条 base_url；首页仅明文展示 URL。第一条次数站 URL 作为合卡主站。"
                >
                  <SiteEditor
                    sites={configSites}
                    onChange={(sites) => setConfig({ ...config, newapi: { sites } })}
                  />
                </ConfigCard>
                <ConfigCard title="自动补卡" description="按时段把库存补齐到目标：查询 NewAPI 当前库存，补 目标-当前 张并推送到活动商城。月次卡与 55 次混合特惠卡各一个独立时段。">
                  <SaleCardManager config={saleCards} onSave={saveSaleSchedule} onRun={runSalePlan} />
                </ConfigCard>
              </Tabs.Panel>

              <Tabs.Panel id="activity" className="admin-tab-panel">
                <ConfigCard title="活动统计" description="统计来自活动模块后台接口，登录态通过统一后台转发。">
                  <ActivityStatsPanel stats={activityStats} />
                </ConfigCard>
                <ConfigCard title="当前奖池中奖率" description="读取当前 /api/prizes，可用于核对中奖率权重。">
                  <CurrentPrizePanel prizes={prizes} />
                </ConfigCard>
                <ConfigCard title="活动配置" description="配置活动日期、整体数学期望值、抽奖次数映射和中奖率权重。">
                  <ActivityConfigEditor
                    activity={config.activity}
                    onChange={(activity) => setConfig({ ...config, activity })}
                  />
                </ConfigCard>
              </Tabs.Panel>
            </Tabs>
          </section>
        ) : (
          <>
            <TopActions>
              <ThemeToggle />
              <NavPill href="/">首页</NavPill>
            </TopActions>
            <BlueprintHeader title="fufu 管理后台" subtitle="站点 · 补货 · 活动" label="ADMIN CONSOLE" compact />
            {checking ? <MessageLine>正在检查登录态…</MessageLine> : <LoginPanel onLogin={login} busy={busy} />}
            <MessageLine tone={message.tone}>{message.text}</MessageLine>
          </>
        )}
    </main>
  );
}
