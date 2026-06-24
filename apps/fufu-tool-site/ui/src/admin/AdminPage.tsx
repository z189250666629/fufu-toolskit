import { useEffect, useMemo, useState } from 'react';
import { Button, Tabs } from '@heroui/react';
import { APIError, fetchJSON, messageFromError, sendJSON } from '../api';
import { BlueprintHeader, MessageLine, NavPill, ThemeToggle, TopActions } from '../blueprint';
import { materializeActivityGameRoutes } from './activityConfigCore';
import { ActivityConfigEditor, ActivityStatsPanel, CurrentPrizePanel } from './activityPanels';
import { ConfigCard, LoginPanel, Metric, type MessageState } from './adminShared';
import { MCYConfigEditor } from './mcyConfigPanel';
import { SaleCardManager } from './saleCardPanel';
import { HomeNavLines, NavigationToolsEditor, RuntimeSites, SiteEditor } from './siteNavigationPanels';
import { buildAdminNavigationSavePayload } from './siteNavigationConfigCore';
import type {
  ActivityStats,
  AdminConfig,
  AdminSession,
  MCYConfig,
  NavLinesResponse,
  PrizeConfigResponse,
  RuntimeSitesResponse,
  SaleCardConfig,
  SaleCardTestKeyResult
} from './types';

const emptyConfig: AdminConfig = {
  newapi: { sites: [] },
  navigation: { cards: [] },
  activity: {},
  mcy: {}
};

async function sendAdminActionJSON<T>(path: string, method: string, payload?: unknown): Promise<T> {
  const response = await fetch(path, {
    method,
    credentials: 'same-origin',
    cache: 'no-store',
    headers: {
      Accept: 'application/json',
      'Content-Type': 'application/json'
    },
    body: payload === undefined ? undefined : JSON.stringify(payload)
  });
  let body: unknown = {};
  try {
    body = await response.json();
  } catch {
    body = {};
  }
  if (!response.ok) {
    const maybeMessage = body && typeof body === 'object' && 'error' in body
      ? String((body as { error?: unknown }).error ?? '')
      : '';
    throw new APIError(maybeMessage || `请求失败（${response.status}）`, response.status, body);
  }
  return body as T;
}

export function AdminPage() {
  const [authenticated, setAuthenticated] = useState(false);
  const [checking, setChecking] = useState(true);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState<MessageState>({ text: '' });
  const [config, setConfig] = useState<AdminConfig>(emptyConfig);
  const [runtimeSites, setRuntimeSites] = useState<RuntimeSitesResponse>();
  const [navLines, setNavLines] = useState<NavLinesResponse>();
  const [activityStats, setActivityStats] = useState<ActivityStats>();
  const [saleCards, setSaleCards] = useState<SaleCardConfig>();
  const [prizes, setPrizes] = useState<PrizeConfigResponse>();

  async function loadConfig() {
    const next = await fetchJSON<AdminConfig>('/api/admin/config');
    setConfig(next);
  }

  async function loadBusinessData(): Promise<string[]> {
    async function optional<T>(label: string, path: string): Promise<{ label: string; value?: T; error?: string }> {
      try {
        return { label, value: await fetchJSON<T>(path) };
      } catch (error) {
        return { label, error: messageFromError(error, `${label}加载失败`) };
      }
    }
    const [stats, saleConfig, prizeConfig, sites, lines] = await Promise.all([
      optional<ActivityStats>('活动统计', '/api/admin/stats'),
      optional<SaleCardConfig>('补卡配置', '/api/admin/sale-cards/config'),
      optional<PrizeConfigResponse>('奖池配置', '/api/prizes'),
      optional<RuntimeSitesResponse>('运行站点', '/api/newapi/sites'),
      optional<NavLinesResponse>('首页线路', '/api/nav/lines')
    ]);
    if (stats.value) setActivityStats(stats.value);
    if (saleConfig.value) setSaleCards(saleConfig.value);
    if (prizeConfig.value) setPrizes(prizeConfig.value);
    if (sites.value) setRuntimeSites(sites.value);
    if (lines.value) setNavLines(lines.value);
    return [stats, saleConfig, prizeConfig, sites, lines]
      .filter((result) => result.error)
      .map((result) => `${result.label}: ${result.error}`);
  }

  async function loadAll() {
    setBusy(true);
    setMessage({ text: '正在加载配置…' });
    try {
      await loadConfig();
      const failures = await loadBusinessData();
      setMessage(failures.length
        ? { text: `配置已加载，部分业务数据加载失败：${failures.join('；')}`, tone: 'error' }
        : { text: '配置与业务数据已加载', tone: 'ok' });
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
      const navigationPayload = buildAdminNavigationSavePayload(config);
      const next = await sendJSON<AdminConfig>('/api/admin/config', 'PUT', {
        ...navigationPayload,
        activity: materializeActivityGameRoutes(config.activity, saleCards?.plans ?? []),
        mcy: config.mcy
      });
      setConfig(next);
      const failures = await loadBusinessData();
      setMessage(failures.length
        ? { text: `配置已保存并应用，部分业务数据加载失败：${failures.join('；')}`, tone: 'error' }
        : { text: '配置已保存并应用', tone: 'ok' });
    } catch (error) {
      setMessage({ text: messageFromError(error), tone: 'error' });
    } finally {
      setBusy(false);
    }
  }

  async function refreshBusinessData() {
    setBusy(true);
    setMessage({ text: '正在刷新业务数据…' });
    try {
      const failures = await loadBusinessData();
      setMessage(failures.length
        ? { text: `部分业务数据加载失败：${failures.join('；')}`, tone: 'error' }
        : { text: '业务数据已刷新', tone: 'ok' });
    } finally {
      setBusy(false);
    }
  }

  async function saveMCY(mcy: MCYConfig) {
    const next = await sendJSON<AdminConfig>('/api/admin/config', 'PUT', { mcy });
    setConfig((current) => ({ ...current, mcy: next.mcy ?? current.mcy }));
  }

  async function generateSaleCardTestKey(plan: string, count: number): Promise<SaleCardTestKeyResult> {
    return sendAdminActionJSON<SaleCardTestKeyResult>('/api/admin/sale-cards/test-key', 'POST', { plan, count });
  }

  const siteCount = config.newapi.sites.length;
  const navLineCount = (navLines?.categories ?? []).reduce((sum, category) => sum + (category.lines?.length ?? 0), 0);
  const navCardCount = config.navigation?.cards?.length ?? 0;
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
                    <Button className="blueprint-button" onPress={refreshBusinessData} isDisabled={busy}>刷新业务数据</Button>
                    <Button className="blueprint-danger-button" onPress={logout}>退出登录</Button>
                  </div>
                </div>
              </div>
              <Tabs.List className="admin-tab-list">
                <Tabs.Tab id="site-replenish" className="admin-tab-card">
                  <span className="admin-tab-title">状态页 / 合卡 / 活动卡档</span>
                </Tabs.Tab>
                <Tabs.Tab id="activity" className="admin-tab-card">
                  <span className="admin-tab-title">活动统计 / 奖池 / 期望值</span>
                </Tabs.Tab>
              </Tabs.List>
            </header>
            <MessageLine tone={message.tone}>{message.text}</MessageLine>

            <Tabs.Panel id="site-replenish" className="admin-tab-panel">
              <ConfigCard
                title="首页导航线路"
                description="首页实际展示的 API/Token 链接；未配置 NewAPI 时显示默认导航链接。"
                action={<Metric label="首页线路" value={navLineCount} />}
              >
                <HomeNavLines lines={navLines} />
              </ConfigCard>
              <ConfigCard
                title="首页卡片"
                description="首页展示哪些卡片由这里配置；API/Token 卡片使用当前站点线路，其余卡片可配置单入口或多线路。"
                action={<Metric label="首页卡片" value={navCardCount} />}
              >
                <NavigationToolsEditor
                  cards={config.navigation?.cards ?? []}
                  onChange={(cards) => setConfig({ ...config, navigation: { cards } })}
                />
              </ConfigCard>
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
              <ConfigCard title="MCY 商城登录" description="商城账号仍用于活动核销等既有流程；自动补卡与库存检测暂时下线，不从后台触发对接。">
                <MCYConfigEditor mcy={config.mcy ?? {}} onChange={(mcy) => setConfig({ ...config, mcy })} onSave={saveMCY} />
              </ConfigCard>
              <ConfigCard title="自动补卡任务状态" description="按配置时段补卡；后台记录任务、超时重试和失败原因，不做全天库存监控。">
                <SaleCardManager config={saleCards} />
              </ConfigCard>
            </Tabs.Panel>

            <Tabs.Panel id="activity" className="admin-tab-panel">
              <ConfigCard title="活动配置" description="配置活动总开关、活动日期、玩法参数和卡档玩法。">
                <ActivityConfigEditor
                  activity={config.activity}
                  stats={activityStats}
                  salePlans={saleCards?.plans ?? []}
                  onGenerateTestKey={generateSaleCardTestKey}
                  onChange={(activity) => setConfig((current) => ({ ...current, activity }))}
                />
              </ConfigCard>
              <ConfigCard title="活动统计" description="统计来自活动模块后台接口，登录态通过统一后台转发。">
                <ActivityStatsPanel stats={activityStats} />
              </ConfigCard>
              <ConfigCard title="当前奖池中奖率" description="读取当前 /api/prizes，可用于核对目标期望值平衡后的概率。">
                <CurrentPrizePanel prizes={prizes} />
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
