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
  PrizeConfigResponse,
  RuntimeSitesResponse,
  SaleCardConfig,
  SaleCardJob,
  SaleCardPlan
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
  url: '',
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
    <div className="admin-metric">
      <span>{label}</span>
      <b>{formatNumber(value)}</b>
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
    <Card className="admin-panel">
      <Card.Header className="admin-panel-head">
        <div className="admin-panel-title">
          <Card.Title>{title}</Card.Title>
          {description ? <Card.Description>{description}</Card.Description> : null}
        </div>
        {action}
      </Card.Header>
      <Card.Content>{children}</Card.Content>
    </Card>
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

function SiteEditor({
  sites,
  onChange
}: {
  sites: ManagedSite[];
  onChange: (sites: ManagedSite[]) => void;
}) {
  function updateSite(index: number, patch: Partial<ManagedSite>) {
    onChange(sites.map((site, siteIndex) => siteIndex === index ? { ...site, ...patch } : site));
  }

  return (
    <div className="site-list">
      {sites.map((site, index) => (
        <Card key={`${site.name}-${index}`} className="site-card">
          <Card.Header className="site-card-head">
            <div>
              <Card.Title>{index === 0 ? '主站点（合卡复用）' : '备用站点'} #{index + 1}</Card.Title>
              <Card.Description>{site.tokenSet ? `当前 token：${site.tokenMasked || '已保存'}` : '新站点必须填写 token'}</Card.Description>
            </div>
            <Button className="blueprint-danger-button" onPress={() => onChange(sites.filter((_, siteIndex) => siteIndex !== index))}>
              删除
            </Button>
          </Card.Header>
          <Card.Content>
            <div className="field-grid">
              <label className="field">名称<Input className="blueprint-input" value={site.name || ''} onChange={(event) => updateSite(index, { name: event.target.value })} /></label>
              <label className="field">base_url<Input className="blueprint-input" value={site.url || ''} placeholder="https://api.example.com" onChange={(event) => updateSite(index, { url: event.target.value })} /></label>
              <label className="field">Token<Input className="blueprint-input" type="password" value={site.token || ''} placeholder={site.tokenMasked || '留空不变'} onChange={(event) => updateSite(index, { token: event.target.value })} /></label>
              <label className="field">User ID<Input className="blueprint-input" value={site.userId || '1'} onChange={(event) => updateSite(index, { userId: event.target.value })} /></label>
              <label className="field">Kind<Input className="blueprint-input" value={site.kind || 'api'} onChange={(event) => updateSite(index, { kind: event.target.value })} /></label>
              <label className="field">Quota Unit<Input className="blueprint-input" type="number" value={String(site.quotaUnit || 500000)} onChange={(event) => updateSite(index, { quotaUnit: Number(event.target.value) })} /></label>
              <label className="field">Currency<Input className="blueprint-input" value={site.currency || '$'} onChange={(event) => updateSite(index, { currency: event.target.value })} /></label>
              <label className="field">Recharge Ratio<Input className="blueprint-input" type="number" step="0.0001" value={String(site.rechargeRatio || 1)} onChange={(event) => updateSite(index, { rechargeRatio: Number(event.target.value) })} /></label>
              <label className="field">Channel Endpoint<Input className="blueprint-input" value={site.channelListEndpoint || ''} onChange={(event) => updateSite(index, { channelListEndpoint: event.target.value })} /></label>
              <label className="field">Note<Input className="blueprint-input" value={site.note || ''} onChange={(event) => updateSite(index, { note: event.target.value })} /></label>
              <label className="field field--inline">
                <input type="checkbox" checked={Boolean(site.skipUserHeader)} onChange={(event) => updateSite(index, { skipUserHeader: event.target.checked })} />
                Skip User Header
              </label>
            </div>
          </Card.Content>
        </Card>
      ))}
      <p className="inline-help">Token 字段加载时只显示掩码；保存时留空表示沿用原 token，新站点必须填写 token。</p>
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

function SaleCardManager({
  config,
  onSave,
  onRun
}: {
  config?: SaleCardConfig;
  onSave: (schedule: NonNullable<SaleCardConfig['schedule']>) => Promise<void>;
  onRun: (plan: string, count: number) => Promise<void>;
}) {
  const plans = config?.plans ?? [];
  const initialSchedule = config?.schedule ?? { enabled: false, time: '09:00', timezone: 'Asia/Shanghai', jobs: [] };
  const [enabled, setEnabled] = useState(Boolean(initialSchedule.enabled));
  const [time, setTime] = useState(initialSchedule.time || '09:00');
  const [timezone, setTimezone] = useState(initialSchedule.timezone || 'Asia/Shanghai');
  const [jobs, setJobs] = useState<SaleCardJob[]>(initialSchedule.jobs ?? []);
  const [runPlan, setRunPlan] = useState(plans[0]?.id ?? '');
  const [runCount, setRunCount] = useState(1);

  useEffect(() => {
    setEnabled(Boolean(initialSchedule.enabled));
    setTime(initialSchedule.time || '09:00');
    setTimezone(initialSchedule.timezone || 'Asia/Shanghai');
    setJobs(initialSchedule.jobs ?? []);
    setRunPlan(plans[0]?.id ?? '');
  }, [config]);

  function jobFor(plan: SaleCardPlan) {
    return jobs.find((job) => job.plan === plan.id) ?? { plan: plan.id, count: 1, enabled: false };
  }

  function updateJob(plan: SaleCardPlan, patch: Partial<SaleCardJob>) {
    const next = jobFor(plan);
    const merged = { ...next, ...patch };
    setJobs((current) => {
      const without = current.filter((job) => job.plan !== plan.id);
      return [...without, merged];
    });
  }

  return (
    <div className="business-stack">
      <div className="action-row">
        <label className="field--inline"><input type="checkbox" checked={enabled} onChange={(event) => setEnabled(event.target.checked)} /> 启用每日自动上架</label>
        <Input className="mini-input blueprint-input" value={time} onChange={(event) => setTime(event.target.value)} aria-label="上架时间" />
        <Input className="mini-input blueprint-input" value={timezone} onChange={(event) => setTimezone(event.target.value)} aria-label="时区" />
        <Button className="blueprint-button" onPress={() => onSave({ enabled, time, timezone, jobs })}>保存上架计划</Button>
      </div>
      <DataTable
        columns={['计划', '额度', '周期', '每日数量', '启用']}
        rows={plans.map((plan) => {
          const job = jobFor(plan);
          return [
            `${plan.name || plan.id} / ITEM ${plan.itemId || '-'} / SKU ${plan.skuId || '-'}`,
            `$${plan.quota ?? '-'}`,
            plan.intervalUnit || '-',
            job.count ?? '',
            job.enabled ? '是' : '否'
          ];
        })}
        empty="暂无销售卡计划"
      />
      <div className="sale-job-list">
        {plans.map((plan) => {
          const job = jobFor(plan);
          return (
            <div className="sale-job-row" key={plan.id}>
              <span>{plan.name || plan.id}</span>
              <Input className="mini-input blueprint-input" type="number" min={1} max={100} value={String(job.count || '')} onChange={(event) => updateJob(plan, { count: Number(event.target.value) })} />
              <label className="field--inline"><input type="checkbox" checked={Boolean(job.enabled)} onChange={(event) => updateJob(plan, { enabled: event.target.checked })} /> 启用</label>
            </div>
          );
        })}
      </div>
      <div className="action-row">
        <select className="native-select" value={runPlan} onChange={(event) => setRunPlan(event.target.value)}>
          {plans.length ? plans.map((plan) => <option key={plan.id} value={plan.id}>{plan.name || plan.id}</option>) : <option value="">暂无计划</option>}
        </select>
        <Input className="mini-input blueprint-input" type="number" min={1} max={100} value={String(runCount)} onChange={(event) => setRunCount(Number(event.target.value))} aria-label="上架数量" />
        <Button className="blueprint-primary-button" onPress={() => onRun(runPlan, runCount)} isDisabled={!runPlan}>立即上架</Button>
      </div>
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
      const next = await sendJSON<AdminConfig>('/api/admin/config', 'PUT', {
        newapi: { sites: config.newapi.sites },
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
    setMessage({ text: '正在保存上架计划…' });
    try {
      const next = await sendJSON<SaleCardConfig>('/api/admin/sale-cards/config', 'POST', schedule);
      setSaleCards(next);
      setMessage({ text: '上架计划已保存', tone: 'ok' });
    } catch (error) {
      setMessage({ text: messageFromError(error), tone: 'error' });
    }
  }

  async function runSalePlan(plan: string, count: number) {
    setMessage({ text: '正在执行销售卡上架…' });
    try {
      await sendJSON('/api/admin/sale-cards/run', 'POST', { plan, count });
      setMessage({ text: '销售卡上架执行完成', tone: 'ok' });
    } catch (error) {
      setMessage({ text: messageFromError(error), tone: 'error' });
    }
  }

  const siteCount = config.newapi.sites.length;
  const configSites = useMemo(() => config.newapi.sites ?? [], [config]);

  return (
    <>
      <TopActions>
        <ThemeToggle />
        <NavPill href="/">首页</NavPill>
      </TopActions>
      <main className="blueprint-page admin-page">
        <BlueprintHeader title="fufu 管 理 后 台" subtitle="站 点 · 补 货 · 活 动" label="ADMIN CONSOLE" compact />
        <div className="admin-doc-meta" aria-hidden="true">
          <span><b>ROUTE</b><span className="blueprint-route-badge">/admin</span></span>
          <span><b>SOURCE</b>SQLite · tool-config.db</span>
          <span><b>ACTOR</b>admin</span>
        </div>
        {checking ? <MessageLine>正在检查登录态…</MessageLine> : null}
        {!authenticated && !checking ? <LoginPanel onLogin={login} busy={busy} /> : null}
        <MessageLine tone={message.tone}>{message.text}</MessageLine>

        {authenticated ? (
          <section className="admin-dashboard fade-in">
            <Tabs className="admin-tabs" defaultSelectedKey="site-replenish">
              <div className="admin-toolbar-row">
                <Tabs.List className="admin-tab-list">
                  <Tabs.Tab id="site-replenish" className="admin-tab-card">
                    <span className="admin-tab-title">状态页 / 合卡 / 自动补货</span>
                  </Tabs.Tab>
                  <Tabs.Tab id="activity" className="admin-tab-card">
                    <span className="admin-tab-title">活动统计 / 奖池 / 期望值</span>
                  </Tabs.Tab>
                </Tabs.List>

                <div className="admin-utility-bar">
                  <span className="admin-utility-note">配置操作</span>
                  <div className="admin-action-group">
                    <Button className="blueprint-button" onPress={loadAll} isDisabled={busy}>重新加载配置</Button>
                    <Button className="blueprint-primary-button" onPress={saveConfig} isDisabled={busy}>保存全部配置</Button>
                    <Button className="blueprint-button" onPress={loadBusinessData} isDisabled={busy}>刷新业务数据</Button>
                    <Button className="blueprint-danger-button" onPress={logout}>退出登录</Button>
                  </div>
                </div>
              </div>

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
                  description="第一个站点作为合卡主站复用；状态页会显示脱敏后的站点信息。"
                  action={<Button className="blueprint-button" onPress={() => setConfig({ ...config, newapi: { sites: [...configSites, { ...defaultSite }] } })}>新增站点</Button>}
                >
                  <SiteEditor
                    sites={configSites}
                    onChange={(sites) => setConfig({ ...config, newapi: { sites } })}
                  />
                </ConfigCard>
                <ConfigCard title="销售卡上架" description="自动补货计划复用当前 NewAPI 主站生成卡密，再推送到活动商城。">
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
        ) : null}
      </main>
    </>
  );
}
