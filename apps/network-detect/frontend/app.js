import {
  escapeHtml,
  idFragment,
  modelScopeTabId,
  modelSiteDisplayName,
  compactNumber,
  formatTime,
  formatServerTime,
  average,
  formatMs,
  formatRate,
  modelStatusMeta,
  formatNullableRate,
  formatPriceValue,
  formatShortTime,
  formatWindow,
  formatCooldown,
  statusFromCounts,
  statusFromSummary,
  successRate
} from './utils.js';

const app = document.getElementById('app');
let filterRenderTimer = null;
let renderMotion = '';

const SAMPLE_COUNT = 4;
const TIMEOUT_MS = 8000;

const DEFAULT_TARGET_GROUPS = [
  {
    id: 'api',
    name: 'API 次数站',
    urls: [
      'https://api.fufuapi.top',
      'https://api.fufuapi.online',
      'https://api.fufuflower.top'
    ]
  },
  {
    id: 'token',
    name: 'Token 站',
    urls: [
      'https://token.fufuapi.top',
      'https://token.fufuapi.online',
      'https://token.fufuflower.top'
    ]
  }
];

const state = {
  loading: false,
  initialized: false,
  modelFilter: '',
  selectedModelSite: '次数fufu',
  selectedTokenGroup: '',
  groupSelectOpen: false,
  modelTestMessage: '',
  activePanel: 'url',
  modelStatus: null,
  testingCells: new Set(),
  client: null,
  targets: [],
  connectivity: {
    running: false,
    mode: 'pending',
    tone: '',
    icon: '?',
    title: '等待测试',
    text: '页面会从当前用户浏览器发起请求，因此结果代表当前用户网络环境。测试会自动访问全部固定 Base URL。',
    progress: 0,
    progressText: '尚未开始测试',
    currentUrl: '尚未开始测试',
    success: '-',
    testedAt: '-',
    results: []
  },
  error: ''
};

function targetGroups() {
  const groups = state.targets?.length ? state.targets : DEFAULT_TARGET_GROUPS;
  return groups
    .map((group) => ({
      id: String(group.id || group.name || ''),
      name: String(group.name || group.id || 'URL 组'),
      urls: Array.isArray(group.urls) ? group.urls.map(String).filter(Boolean) : []
    }))
    .filter((group) => group.urls.length);
}

function addCacheBust(url) {
  const next = new URL(url, window.location.href);
  next.searchParams.set('_fufu_connect_test', `${Date.now()}_${Math.random().toString(16).slice(2)}`);
  return next.toString();
}

function fetchWithTimeout(url, options, timeoutMs) {
  const controller = new AbortController();
  const timer = window.setTimeout(() => controller.abort(), timeoutMs);
  return fetch(url, { ...options, signal: controller.signal })
    .finally(() => window.clearTimeout(timer));
}

function fetchErrorText(error) {
  if (error?.name === 'AbortError') return '请求超时';
  return '请求失败或被浏览器拦截';
}

async function fetchJson(path, options = {}) {
  const headers = {
    ...(options.headers || {})
  };
  const response = await fetch(path, { ...options, headers, cache: 'no-store' });
  const data = await response.json().catch(() => ({}));
  if (!response.ok) {
    const error = new Error(data.error || data.configError || `HTTP ${response.status}`);
    error.status = response.status;
    error.data = data;
    throw error;
  }
  return data;
}

function postJson(path, body) {
  return fetchJson(path, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body)
  });
}

async function loadStaticContext() {
  const [client, targets] = await Promise.all([
    fetchJson('/api/client').catch(() => null),
    fetchJson('/api/connectivity/targets').catch(() => ({ groups: [] }))
  ]);
  state.client = client;
  state.targets = targets.groups || [];
}

async function loadModelStatus(refresh = false, options = {}) {
  state.loading = true;
  state.error = '';
  state.modelTestMessage = '';
  if (options.renderStart !== false) render();

  try {
    state.modelStatus = await fetchJson(`/api/newapi/model-status${refresh ? '?refresh=1' : ''}`);
    state.initialized = true;
  } catch (error) {
    state.error = error.message;
    if (error.data && typeof error.data === 'object' && Number.isFinite(Number(error.data.generatedAt))) {
      state.modelStatus = error.data;
    }
  } finally {
    state.loading = false;
    render();
  }
}

function renderMetric(label, value, sub = '', tone = '') {
  return `
    <div class="metric ${tone ? `metric-${tone}` : ''}" data-slot="card">
      <div class="metric-head" data-slot="card-header">
        <span data-slot="card-title">${escapeHtml(label)}</span>
        ${sub ? `<small data-slot="card-description">${escapeHtml(sub)}</small>` : '<small class="sr-only" data-slot="card-description">当前指标</small>'}
      </div>
      <div class="metric-content" data-slot="card-content">
        <b>${escapeHtml(value)}</b>
      </div>
    </div>
  `;
}

function renderStateCard(title, description, content = '') {
  return `
    <div class="model-state-card" data-slot="card">
      <div class="model-state-copy" data-slot="card-header">
        <h3 data-slot="card-title">${escapeHtml(title)}</h3>
        <p data-slot="card-description">${escapeHtml(description)}</p>
      </div>
      <span data-slot="card-content">${escapeHtml(content || title)}</span>
    </div>
  `;
}

function renderChip(label, tone = 'muted', className = '') {
  return `
    <span class="chip ${className} ${tone}" data-slot="chip">
      <span class="chip-label" data-slot="chip-label">${escapeHtml(label)}</span>
    </span>
  `;
}

function renderAlert(message, tone = 'danger', className = '') {
  const titles = {
    danger: '错误',
    info: '提示',
    success: '完成',
    warning: '注意'
  };
  const title = titles[tone] || '提示';
  const role = tone === 'danger' ? 'alert' : 'status';
  return `
    <div class="notice alert ${className}" role="${role}" data-slot="alert" data-status="${escapeHtml(tone)}">
      <span class="alert-indicator" data-slot="alert-indicator" aria-hidden="true"></span>
      <span class="alert-content" data-slot="alert-content">
        <span class="alert-title sr-only" data-slot="alert-title">${title}</span>
        <span class="alert-description" data-slot="alert-description">${escapeHtml(message)}</span>
      </span>
    </div>
  `;
}

function renderHeader() {
  const generatedAt = state.modelStatus?.generatedAt
    ? formatTime(state.modelStatus.generatedAt)
    : formatServerTime(state.client?.serverTime);
  return `
    <header class="app-header">
      <div class="brand-block">
        <div class="brand-mark">API</div>
        <div>
          <h1>fufu API 状态面板</h1>
          <p>固定展示 Base URL 连通性和管理站模型可用性</p>
        </div>
      </div>
      <div class="header-meta">
        <span>更新时间</span>
        <b>${escapeHtml(generatedAt)}</b>
      </div>
    </header>
  `;
}

function connectivityTagClass(status) {
  if (status === 'ok') return 'ok';
  if (status === 'warn') return 'warn';
  if (status === 'bad') return 'bad';
  return 'idle';
}

function renderConnectivityRow(result) {
  const best = result.starred ? '<span class="url-star" aria-label="最优">⭐</span>' : '';
  const bestClass = result.starred ? ' is-best' : '';
  const safeUrl = escapeHtml(result.url);
  return `
    <div class="result-row${bestClass}">
      <div class="url-cell">
        <button class="url-copy" type="button" value="${safeUrl}" data-copy-value="${safeUrl}" title="点击复制 URL" aria-label="复制 ${safeUrl}">
          <span class="url-text">${safeUrl}</span>${best}
          <span class="copy-tip" aria-hidden="true">已复制</span>
        </button>
      </div>
      <div class="result-field">
        <span class="result-label">状态</span>
        <b>${renderChip(result.label, connectivityTagClass(result.status), 'tag')}</b>
      </div>
      <div class="result-field">
        <span class="result-label">成功率</span>
        <b>${escapeHtml(result.rate)}</b>
      </div>
      <div class="result-field">
        <span class="result-label">平均延迟</span>
        <b>${escapeHtml(result.latency)}</b>
      </div>
    </div>
  `;
}

function renderConnectivityResults() {
  const results = state.connectivity.results || [];
  const groups = targetGroups();
  return groups.map((group) => {
    const groupResults = results.filter((item) => item.groupId === group.id);
    const okCount = groupResults.filter((item) => item.reachable).length;
    const resultMap = Object.fromEntries(groupResults.map((item) => [item.url, item]));
    const bestInGroup = groupResults
      .filter((item) => item.reachable && item.averageMs != null)
      .sort((a, b) => a.averageMs - b.averageMs)[0] || null;

    return `
      <div class="connectivity-group" data-slot="card">
        <div class="group-head" data-slot="card-header">
          <h3 data-slot="card-title">${escapeHtml(group.name)}</h3>
          <span data-slot="card-description">${results.length ? `可达 ${okCount}/${group.urls.length}` : `${group.urls.length} 个站点`}</span>
        </div>
        <div class="result-list" data-slot="card-content">
          ${group.urls.map((url) => {
            const item = resultMap[url];
            if (!item) {
              return renderConnectivityRow({
                url,
                status: 'idle',
                label: '等待',
                rate: '-',
                latency: '-',
                starred: false
              });
            }
            return renderConnectivityRow({
              url: item.url,
              status: item.reachable ? 'ok' : 'bad',
              label: item.reachable ? '可达' : '失败',
              rate: formatRate(item.successRate),
              latency: formatMs(item.averageMs),
              starred: !!bestInGroup && item.url === bestInGroup.url
            });
          }).join('')}
        </div>
      </div>
    `;
  }).join('');
}

function renderPanelToggle() {
  const options = [
    ['url', 'URL 检测'],
    ['models', '模型状态']
  ];
  const activeIndex = Math.max(0, options.findIndex(([value]) => state.activePanel === value));

  return `
    <div
      class="panel-toggle tabs__list"
      role="tablist"
      aria-label="状态视图"
      data-slot="tab-list"
      data-tab-motion-key="panel"
      data-orientation="horizontal"
      style="--tab-count: ${options.length}; --active-tab-index: ${activeIndex};"
    >
      <span class="tab-indicator tabs__indicator" data-slot="tab-indicator" aria-hidden="true"></span>
      ${options.map(([value, label]) => {
        const active = state.activePanel === value;
        return `
        <button
          class="toggle-button tabs__tab ${active ? 'active' : ''}"
          type="button"
          role="tab"
          aria-selected="${active ? 'true' : 'false'}"
          aria-controls="${value}Panel"
          id="${value}Tab"
          tabindex="${active ? '0' : '-1'}"
          data-slot="tab"
          data-selected="${active ? 'true' : 'false'}"
          data-panel="${escapeHtml(value)}"
        >
          <span class="tab-label" data-slot="tab-label">${escapeHtml(label)}</span>
        </button>
      `;
      }).join('')}
    </div>
  `;
}

function motionClass(...types) {
  return types.includes(renderMotion) ? ' motion-enter' : '';
}

function captureTabIndicatorRects() {
  const rects = new Map();
  document.querySelectorAll('.tabs__list > .tab-indicator').forEach((indicator) => {
    const tabList = indicator.closest('[role="tablist"]');
    const key = tabList?.dataset.tabMotionKey || tabList?.getAttribute('aria-label') || '';
    if (key) rects.set(key, indicator.getBoundingClientRect());
  });
  return rects;
}

function animateTabIndicators(previousRects) {
  if (!previousRects?.size) return;
  if (window.matchMedia?.('(prefers-reduced-motion: reduce)').matches) return;

  document.querySelectorAll('.tabs__list > .tab-indicator').forEach((indicator) => {
    const tabList = indicator.closest('[role="tablist"]');
    const key = tabList?.dataset.tabMotionKey || tabList?.getAttribute('aria-label') || '';
    const previous = previousRects.get(key);
    if (!previous) return;

    const next = indicator.getBoundingClientRect();
    const horizontal = tabList?.dataset.orientation !== 'vertical';
    const dx = previous.left - next.left;
    const dy = horizontal ? 0 : previous.top - next.top;
    if (Math.abs(dx) < 0.5 && Math.abs(dy) < 0.5) return;

    indicator.style.transition = 'none';
    indicator.style.transform = `translate(calc(var(--indicator-x) + ${dx}px), calc(var(--indicator-y) + ${dy}px))`;
    indicator.getBoundingClientRect();

    requestAnimationFrame(() => {
      indicator.style.transition = 'transform 260ms var(--ease-out-fluid), width 260ms var(--ease-out-fluid), height 260ms var(--ease-out-fluid)';
      indicator.style.transform = '';
      window.setTimeout(() => {
        indicator.style.transition = '';
      }, 280);
    });
  });
}

function renderUrlStatus() {
  const current = state.connectivity;
  return `
    <div class="url-monitor-grid${motionClass('panel')}" id="urlPanel" role="tabpanel" aria-labelledby="urlTab" data-slot="tab-panel">
      <article class="verdict-card ${escapeHtml(current.mode)}" id="verdictCard" data-slot="card">
        <div class="verdict-icon ${escapeHtml(current.tone)}" id="verdictIcon" data-slot="card-media">${escapeHtml(current.icon)}</div>
        <div class="verdict-body">
          <div class="verdict-copy" data-slot="card-header">
            <h2 id="verdictTitle" data-slot="card-title">${escapeHtml(current.title)}</h2>
            <p id="verdictText" data-slot="card-description">${escapeHtml(current.text)}</p>
          </div>
          <div class="verdict-main" data-slot="card-content">
          <div class="connectivity-metrics">
            ${renderMetric('可达率', current.success, '固定 Base URL', '')}
            ${renderMetric('最后测试', current.testedAt, '当前浏览器网络', '')}
          </div>
          <div class="verdict-actions">
            <button class="button primary" id="runConnectivityBtn" type="button" ${current.running ? 'disabled' : ''}>${current.running ? '测试中' : (current.results.length ? '重新测试' : '开始测试')}</button>
          </div>
          <div class="progress-panel">
            <div class="progress-line">
              <span id="progressText">${escapeHtml(current.progressText)}</span>
              <b id="progressPct">${escapeHtml(`${current.progress}%`)}</b>
            </div>
            <div
              class="progress-bar"
              role="progressbar"
              aria-valuemin="0"
              aria-valuemax="100"
              aria-valuenow="${escapeHtml(current.progress)}"
              aria-labelledby="progressText"
              data-slot="progressbar"
            >
              <div class="track progress-bar-track" data-slot="progress-track">
                <div class="bar progress-bar-fill" id="progressBar" data-slot="progress-fill" style="width: ${escapeHtml(`${current.progress}%`)}"></div>
              </div>
            </div>
            <div class="current-url" id="currentUrl">${escapeHtml(current.currentUrl)}</div>
          </div>
          </div>
        </div>
      </article>
      <div class="results-block" data-slot="card">
        <div class="section-head" data-slot="card-header">
          <h2 data-slot="card-title">检测结果</h2>
          <span data-slot="card-description">浏览器直接访问</span>
        </div>
        <div class="groups" id="connectivityResultGroups" data-slot="card-content">
          ${renderConnectivityResults()}
        </div>
      </div>
    </div>
  `;
}

function renderConnectivity() {
  return `
    <section class="monitor-panel" data-slot="card">
      <div class="tabs panel-tabs" data-slot="tabs" data-orientation="horizontal">
        <div class="monitor-head" data-slot="card-header">
          <div>
            <h2 data-slot="card-title">状态面板</h2>
            <p data-slot="card-description">${state.activePanel === 'url' ? '当前浏览器网络的 URL 连通性' : '已配置管理站点的模型可用性'}</p>
          </div>
          ${renderPanelToggle()}
        </div>
        <div class="monitor-content" data-slot="card-content">
          ${state.activePanel === 'url' ? renderUrlStatus() : renderModelAvailability()}
        </div>
      </div>
    </section>
  `;
}

function modelSortRank(row) {
  const rank = { down: 0, degraded: 1, operational: 2, unknown: 3 };
  return rank[row.status] ?? 4;
}

function renderStatusPill(status, configured = true) {
  const meta = modelStatusMeta(status, configured);
  return renderChip(meta.label, meta.tone, 'status');
}

function renderPriceCell(pricing) {
  if (!pricing?.available) return '<span class="price-empty">-</span>';
  if (pricing.type === 'dynamic') {
    return `
      <div class="price-cell">
        <b>阶梯计费</b>
        <small>按规则计算</small>
      </div>
    `;
  }
  if (pricing.type === 'request') {
    return `
      <div class="price-cell">
        <b>${escapeHtml(formatPriceValue(pricing.request, pricing.currency))}</b>
        <small>每次请求</small>
      </div>
    `;
  }
  return `
    <div class="price-cell">
      <b>入 ${escapeHtml(formatPriceValue(pricing.input, pricing.currency))}</b>
      <small>出 ${escapeHtml(formatPriceValue(pricing.output, pricing.currency))} / 1M</small>
    </div>
  `;
}

function activeModelScope(modelStatus) {
  const sites = modelStatus?.sites || [];
  const preferredSite = sites.find((item) => item.site.name === state.selectedModelSite) || sites[0] || null;
  const siteName = preferredSite?.site.name || '';
  const groups = preferredSite?.groups || [];
  const group = siteName === '次数fufu'
    ? 'mix'
    : (state.selectedTokenGroup && groups.includes(state.selectedTokenGroup) ? state.selectedTokenGroup : groups[0] || '');
  return { site: preferredSite, siteName, group, groups };
}

function groupCellFor(row, siteName, group) {
  const cell = row.perSite?.[siteName];
  if (!cell?.configured || !group) return null;
  const groupCell = cell.groupStats?.[group];
  if (!groupCell?.configured) return null;
  return applyManualTestDisplay({
    ...cell,
    ...groupCell,
    siteName,
    model: row.model,
    configured: true,
    groups: [group],
    manualTest: cell.manualTest,
    nextTestAllowedAt: cell.nextTestAllowedAt
  });
}

function applyManualTestDisplay(cell) {
  const manual = cell.manualTest;
  if (!manual?.testedAt) return cell;

  const passed = manual.ok === true || manual.status === 'operational';
  const testedAt = Number(manual.testedAt) || 0;
  const successCount = Number(cell.successCount) || 0;
  const failureCount = Number(cell.failureCount) || 0;
  const hasLogData = successCount + failureCount > 0;

  if (!passed && hasLogData) return cell;

  const nextSuccessCount = successCount + (passed ? 1 : 0);
  const nextFailureCount = failureCount + (passed ? 0 : 1);
  return {
    ...cell,
    manualTestTone: passed ? 'ok' : 'bad',
    status: statusFromCounts(nextSuccessCount, nextFailureCount),
    successRate: nextSuccessCount / (nextSuccessCount + nextFailureCount),
    requestCount: nextSuccessCount + nextFailureCount,
    successCount: nextSuccessCount,
    failureCount: nextFailureCount,
    lastSuccessAt: passed ? Math.max(Number(cell.lastSuccessAt) || 0, testedAt) : cell.lastSuccessAt,
    lastFailureAt: passed ? cell.lastFailureAt : Math.max(Number(cell.lastFailureAt) || 0, testedAt),
    lastSeenAt: testedAt || cell.lastSeenAt
  };
}

function scopedModelRows(modelStatus, scope, applyTextFilter = true) {
  const filter = state.modelFilter.trim().toLowerCase();
  return (modelStatus?.models || [])
    .map((row) => ({ row, cell: groupCellFor(row, scope.siteName, scope.group) }))
    .filter((item) => item.cell)
    .filter((item) => item.cell.enabledChannelCount > 0)
    .filter((item) => !applyTextFilter || !filter || item.row.model.toLowerCase().includes(filter))
    .sort((a, b) => {
      const statusRank = modelSortRank({ status: a.cell.status }) - modelSortRank({ status: b.cell.status });
      if (statusRank !== 0) return statusRank;
      return a.row.model.localeCompare(b.row.model);
    });
}

function scopedSummary(rows) {
  const summary = rows.reduce(
    (acc, item) => {
      acc.modelCount += 1;
      acc.requestCount += item.cell.requestCount;
      acc.successCount += item.cell.successCount;
      acc.failureCount += item.cell.failureCount;
      acc[item.cell.status] += 1;
      return acc;
    },
    { modelCount: 0, requestCount: 0, successCount: 0, failureCount: 0, operational: 0, degraded: 0, down: 0, unknown: 0 }
  );
  summary.successRate = successRate(summary.successCount, summary.failureCount);
  summary.modelAvailabilityRate = summary.modelCount > 0 ? summary.operational / summary.modelCount : null;
  summary.status = statusFromSummary(summary);
  return summary;
}

function renderSiteStatusCard(site, group, summary, windowLabel) {
  const meta = modelStatusMeta(summary.status, true);
  return `
    <article class="model-site-card" data-slot="card">
      <div class="model-site-head" data-slot="card-header">
        <div>
          <h3 data-slot="card-title">${escapeHtml(modelSiteDisplayName(site.site.name))}</h3>
          <p data-slot="card-description">${escapeHtml(site.site.url)} · ${escapeHtml(group || '-')}</p>
        </div>
        ${renderChip(meta.label, meta.tone, 'status')}
      </div>
      <div class="model-site-stats" data-slot="card-content">
        ${renderMetric('请求成功率', formatNullableRate(summary.successRate), `${compactNumber(summary.requestCount)} 次请求`, '')}
        ${renderMetric('模型可用率', formatNullableRate(summary.modelAvailabilityRate), `${summary.operational}/${summary.modelCount} 正常`, 'success')}
        ${renderMetric('失败数', compactNumber(summary.failureCount), `最近 ${windowLabel}`, summary.failureCount ? 'danger' : '')}
        ${renderMetric('未知模型', compactNumber(summary.unknown), '无调用记录', '')}
      </div>
      ${site.logError || site.channelsError ? `<p class="site-error" data-slot="card-footer">${escapeHtml(site.logError || site.channelsError)}</p>` : ''}
    </article>
  `;
}

function renderModelTestAction(cell) {
  const key = `${cell.siteName}\u0000${cell.model}`;
  const testing = state.testingCells.has(key);
  const cooldown = formatCooldown(cell.nextTestAllowedAt);

  return `
    <div class="model-action">
      <button
        class="button small model-test-button"
        type="button"
        data-model-test="1"
        data-site="${escapeHtml(cell.siteName)}"
        data-model="${escapeHtml(cell.model)}"
        data-group="${escapeHtml(cell.groups?.[0] || '')}"
        ${testing || Boolean(cooldown) ? 'disabled' : ''}
      >${testing ? '测试中' : (cooldown || '测试')}</button>
    </div>
  `;
}

function manualTestRowClass(cell) {
  if (cell.manualTestTone === 'ok') return 'is-manual-ok';
  if (cell.manualTestTone === 'bad') return 'is-manual-bad';
  return '';
}

function renderTokenGroupSelect(groups, selectedGroup) {
  const isOpen = state.groupSelectOpen;
  const current = selectedGroup || groups[0] || '';

  return `
    <div class="field group-select heroui-select" data-slot="select" data-token-group-select>
      <span id="tokenGroupSelectLabel" data-slot="select-label">分组</span>
      <button
        class="heroui-select-trigger"
        id="tokenGroupSelect"
        type="button"
        data-slot="select-trigger"
        aria-haspopup="listbox"
        aria-expanded="${isOpen ? 'true' : 'false'}"
        ${isOpen ? 'aria-controls="tokenGroupListbox"' : ''}
        aria-labelledby="tokenGroupSelectLabel tokenGroupSelectValue"
        data-token-group-trigger
      >
        <span class="heroui-select-value" id="tokenGroupSelectValue" data-slot="select-value">${escapeHtml(current || '选择分组')}</span>
        <span class="heroui-select-indicator" data-slot="select-indicator" aria-hidden="true">
          <svg viewBox="0 0 20 20" focusable="false">
            <path d="M5.5 7.5 10 12l4.5-4.5" />
          </svg>
        </span>
      </button>
      ${isOpen ? `
        <div class="heroui-select-popover" data-slot="select-popover">
          <div class="heroui-select-listbox" id="tokenGroupListbox" role="listbox" data-slot="listbox" aria-labelledby="tokenGroupSelectLabel">
            ${groups.map((group) => `
              <button
                class="heroui-select-item"
                type="button"
                role="option"
                data-slot="listbox-item"
                data-selected="${group === current ? 'true' : 'false'}"
                aria-selected="${group === current ? 'true' : 'false'}"
                data-token-group-option="${escapeHtml(group)}"
              >
                <span>${escapeHtml(group)}</span>
                <span class="heroui-select-item-indicator" data-slot="select-item-indicator" aria-hidden="true">
                  ${group === current ? `
                    <svg viewBox="0 0 20 20" focusable="false">
                      <path d="m4.75 10.25 3.25 3.25 7.25-7.25" />
                    </svg>
                  ` : ''}
                </span>
              </button>
            `).join('')}
          </div>
        </div>
      ` : ''}
    </div>
  `;
}

function renderModelScopeControls(modelStatus, scope) {
  const sites = modelStatus.sites || [];
  const activeIndex = Math.max(0, sites.findIndex((item) => item.site.name === scope.siteName));
  const hasTokenGroups = scope.siteName === 'token-fufu';
  return `
    <div class="model-scope-bar">
      <div
        class="model-scope-tabs tabs__list"
        role="tablist"
        aria-label="模型站点"
        data-slot="tab-list"
        data-tab-motion-key="model-site"
        data-orientation="horizontal"
        style="--tab-count: ${Math.max(1, sites.length)}; --active-tab-index: ${activeIndex};"
      >
        <span class="tab-indicator tabs__indicator" data-slot="tab-indicator" aria-hidden="true"></span>
        ${sites.map((item) => {
          const active = item.site.name === scope.siteName;
          return `
          <button
            class="scope-button tabs__tab ${active ? 'active' : ''}"
            type="button"
            role="tab"
            aria-selected="${active ? 'true' : 'false'}"
            aria-controls="modelScopePanel"
            id="${modelScopeTabId(item.site.name)}"
            tabindex="${active ? '0' : '-1'}"
            data-slot="tab"
            data-selected="${active ? 'true' : 'false'}"
            data-model-site="${escapeHtml(item.site.name)}"
          >
            <span class="tab-label" data-slot="tab-label">${escapeHtml(modelSiteDisplayName(item.site.name))}</span>
          </button>
        `;
        }).join('')}
      </div>
      <div class="model-scope-group-slot${hasTokenGroups ? '' : ' is-placeholder'}" ${hasTokenGroups ? '' : 'aria-hidden="true"'}>
        ${hasTokenGroups ? renderTokenGroupSelect(scope.groups || [], scope.group) : ''}
      </div>
    </div>
  `;
}

function renderModelAvailability() {
  const modelStatus = state.modelStatus;
  const sites = modelStatus?.sites || [];

  if (state.loading && !modelStatus) {
    return `
      <div class="model-state-empty${motionClass('panel', 'scope')}" id="modelsPanel" role="tabpanel" aria-labelledby="modelsTab" data-slot="tab-panel">
        ${renderStateCard('正在读取模型状态', '等待服务端返回管理站点模型数据', '加载中')}
      </div>
    `;
  }

  if (!modelStatus?.configured || !sites.length) {
    const reason = state.error || modelStatus?.configError || '当前没有可展示的管理站点或模型统计';
    return `
      <div class="model-state-empty${motionClass('panel', 'scope')}" id="modelsPanel" role="tabpanel" aria-labelledby="modelsTab" data-slot="tab-panel">
        ${renderStateCard('暂无模型状态数据', reason, '未配置')}
      </div>
    `;
  }

  const scope = activeModelScope(modelStatus);
  const allScopedRows = scopedModelRows(modelStatus, scope, false);
  const models = scopedModelRows(modelStatus, scope, true);
  const summary = scopedSummary(allScopedRows);
  const windowLabel = formatWindow(modelStatus.windowSeconds);
  const scopeTabId = modelScopeTabId(scope.siteName);

  return `
    <div class="model-status-panel${motionClass('panel')}" id="modelsPanel" role="tabpanel" aria-labelledby="modelsTab" data-slot="tab-panel">
      <div class="section-head model-status-head">
        <h2>模型状态</h2>
        <span>最近 ${escapeHtml(windowLabel)} · 下次刷新 ${escapeHtml(formatShortTime(modelStatus.expiresAt))}</span>
      </div>
      <div class="tabs model-scope-tabs-wrap" data-slot="tabs" data-orientation="horizontal">
        ${renderModelScopeControls(modelStatus, scope)}
        <div class="model-scope-content${motionClass('scope')}" id="modelScopePanel" role="tabpanel" aria-labelledby="${scopeTabId}" data-slot="tab-panel">
          ${scope.group ? renderSiteStatusCard(scope.site, scope.group, summary, windowLabel) : renderStateCard('请选择分组', '选择一个分组后查看模型状态', '等待选择')}
          <div class="model-toolbar">
            <label class="field model-filter">
              <span>模型筛选</span>
              <input id="modelStatusFilter" data-slot="input" value="${escapeHtml(state.modelFilter)}" placeholder="输入模型名过滤状态列表" />
            </label>
            <div class="model-status-counts">
              ${renderChip(`不可用 ${compactNumber(summary.down)}`, 'bad', 'status')}
              ${renderChip(`部分异常 ${compactNumber(summary.degraded)}`, 'warn', 'status')}
              ${renderChip(`未知 ${compactNumber(summary.unknown)}`, 'muted', 'status')}
              ${renderChip(`正常 ${compactNumber(summary.operational)}`, 'ok', 'status')}
            </div>
          </div>
          ${state.modelTestMessage ? renderAlert(state.modelTestMessage, 'info', 'model-test-notice') : ''}
          ${state.error ? renderAlert(state.error, 'danger') : ''}
          <div class="table-wrap availability-wrap">
            <table class="data-table availability-table">
              <thead>
                <tr>
                  <th class="sticky-col">模型</th>
                  <th>价格</th>
                  <th>状态</th>
                  <th>成功率</th>
                  <th>最近</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                ${models.map(({ row, cell }) => `
                  <tr class="${manualTestRowClass(cell)}">
                    <td class="sticky-col model-name" data-label="模型">${escapeHtml(row.model)}</td>
                    <td data-label="价格">${renderPriceCell(cell.pricing)}</td>
                    <td data-label="状态">${renderStatusPill(cell.status, true)}</td>
                    <td data-label="成功率">${escapeHtml(formatNullableRate(cell.successRate))}</td>
                    <td data-label="最近">${escapeHtml(formatShortTime(cell.lastSuccessAt || cell.lastFailureAt || cell.lastSeenAt))}</td>
                    <td data-label="操作">${renderModelTestAction(cell)}</td>
                  </tr>
                `).join('')}
              </tbody>
            </table>
          </div>
          ${models.length === 0 ? '<div class="empty-inline">没有匹配的模型</div>' : ''}
        </div>
      </div>
    </div>
  `;
}

function renderEnvironment() {
  const client = state.client || {};
  return `
    <section class="section environment-section">
      <div class="section-head">
        <h2>访问环境</h2>
        <span>客户端与服务端</span>
      </div>
      <div class="info-list">
        <div><span>客户端 IP</span><b>${escapeHtml(client.ip || '-')}</b></div>
        <div><span>服务器时间</span><b>${escapeHtml(formatServerTime(client.serverTime))}</b></div>
        <div><span>浏览器时间</span><b>${escapeHtml(new Date().toLocaleString('zh-CN', { hour12: false }))}</b></div>
        <div><span>浏览器在线</span><b>${navigator.onLine ? '是' : '否'}</b></div>
        <div><span>时区</span><b>${escapeHtml(Intl.DateTimeFormat().resolvedOptions().timeZone || '未知')}</b></div>
        <div><span>网络类型</span><b>${escapeHtml(formatNetworkType())}</b></div>
      </div>
    </section>
  `;
}

function captureRenderScroll() {
  const tableWrap = document.querySelector('.availability-wrap');
  return {
    activePanel: state.activePanel,
    windowX: window.scrollX,
    windowY: window.scrollY,
    tableTop: tableWrap?.scrollTop || 0,
    tableLeft: tableWrap?.scrollLeft || 0
  };
}

function restoreRenderScroll(snapshot) {
  if (!snapshot || snapshot.activePanel !== state.activePanel) return;
  const tableWrap = document.querySelector('.availability-wrap');
  if (tableWrap) {
    tableWrap.scrollTop = snapshot.tableTop;
    tableWrap.scrollLeft = snapshot.tableLeft;
  }
  window.scrollTo(snapshot.windowX, snapshot.windowY);
}

function render() {
  const scrollSnapshot = captureRenderScroll();
  document.title = 'fufu API 状态面板';
  app.innerHTML = `
    ${renderHeader()}
    ${renderConnectivity()}
    ${state.loading && !state.modelStatus ? '<div class="boot-state">正在读取模型状态</div>' : ''}
    ${renderEnvironment()}
  `;

  bindEvents();
  restoreRenderScroll(scrollSnapshot);
}

function renderWithMotion(motion) {
  const previousRects = ['panel', 'scope'].includes(motion) ? captureTabIndicatorRects() : null;
  renderMotion = '';
  render();
  animateTabIndicators(previousRects);
}

function setConnectivityState(partial) {
  Object.assign(state.connectivity, partial);
  render();
}

async function probeNoCors(url, onSample) {
  const attempts = [];

  for (let i = 0; i < SAMPLE_COUNT; i++) {
    onSample(i + 1, SAMPLE_COUNT);
    const started = performance.now();
    try {
      await fetchWithTimeout(addCacheBust(url), {
        method: 'GET',
        mode: 'no-cors',
        cache: 'no-store',
        credentials: 'omit',
        redirect: 'follow'
      }, TIMEOUT_MS);

      attempts.push({
        ok: true,
        ms: performance.now() - started,
        error: ''
      });
    } catch (error) {
      attempts.push({
        ok: false,
        ms: performance.now() - started,
        error: fetchErrorText(error)
      });
    }
  }

  const okAttempts = attempts.filter((item) => item.ok);
  return {
    ok: okAttempts.length > 0,
    successRate: okAttempts.length / attempts.length,
    averageMs: average(okAttempts.map((item) => item.ms)),
    lastError: attempts.at(-1)?.error || ''
  };
}

async function testTarget(group, url, index, total) {
  const baseProgress = (index / total) * 90;
  setConnectivityState({
    currentUrl: url,
    progress: Math.round(baseProgress),
    progressText: `测试 ${group.name}: ${url}`
  });

  const reach = await probeNoCors(url, (current, samples) => {
    const sampleProgress = (current / samples) * (90 / total);
    setConnectivityState({
      progress: Math.round(baseProgress + sampleProgress),
      progressText: `${group.name} 采样 ${current}/${samples}`,
      currentUrl: url
    });
  });

  return {
    groupId: group.id,
    groupName: group.name,
    url,
    reachable: reach.ok,
    successRate: reach.successRate,
    averageMs: reach.averageMs,
    lastError: reach.lastError
  };
}

async function runConnectivityTests() {
  if (state.connectivity.running) return;

  const allTargets = targetGroups().flatMap((group) => group.urls.map((url) => ({ group, url })));
  if (!allTargets.length) {
    setConnectivityState({
      mode: 'complete',
      tone: 'bad',
      icon: 'x',
      title: '没有测试目标',
      text: '后端没有返回固定 Base URL 目标。',
      progress: 100,
      progressText: '没有目标',
      currentUrl: '-',
      success: '-',
      testedAt: new Date().toLocaleString('zh-CN', { hour12: false }),
      results: []
    });
    return;
  }

  setConnectivityState({
    running: true,
    mode: 'running',
    tone: 'warn',
    icon: '...',
    title: '测试中',
    text: '正在从当前浏览器逐个访问固定 Base URL。',
    progress: 0,
    progressText: '准备测试',
    currentUrl: '正在准备测试目标',
    success: '-',
    testedAt: '-',
    results: []
  });

  const results = [];

  try {
    for (let i = 0; i < allTargets.length; i++) {
      const { group, url } = allTargets[i];
      const result = await testTarget(group, url, i, allTargets.length);
      results.push(result);
      state.connectivity.results = [...results];
      render();
    }

    const totalReachable = results.filter((item) => item.reachable).length;
    const total = results.length;
    const testedAt = new Date().toLocaleString('zh-CN', { hour12: false });

    if (totalReachable === total) {
      setConnectivityState({
        running: false,
        mode: 'complete',
        tone: 'ok',
        icon: 'OK',
        title: '全部可达',
        text: '当前用户浏览器可以访问全部 API 次数站和 Token 站 Base URL。',
        progress: 100,
        progressText: '测试完成',
        currentUrl: '全部目标测试完成',
        success: formatRate(totalReachable / total),
        testedAt,
        results
      });
    } else if (totalReachable > 0) {
      setConnectivityState({
        running: false,
        mode: 'complete',
        tone: 'warn',
        icon: '!',
        title: '部分可达',
        text: '当前用户网络只能访问部分 fufu Base URL，请优先使用可达且延迟较低的站点。',
        progress: 100,
        progressText: '测试完成',
        currentUrl: '全部目标测试完成',
        success: formatRate(totalReachable / total),
        testedAt,
        results
      });
    } else {
      setConnectivityState({
        running: false,
        mode: 'complete',
        tone: 'bad',
        icon: 'x',
        title: '全部不可达',
        text: '当前用户浏览器无法访问这些 fufu Base URL。可能是 DNS、证书、网络阻断、代理或目标服务异常。',
        progress: 100,
        progressText: '测试完成',
        currentUrl: '全部目标测试完成',
        success: '0%',
        testedAt,
        results
      });
    }
  } catch (error) {
    setConnectivityState({
      running: false,
      mode: 'complete',
      tone: 'bad',
      icon: 'x',
      title: '测试异常',
      text: error.message || '浏览器执行检测时发生异常。',
      progress: 100,
      progressText: '测试异常',
      currentUrl: '-',
      testedAt: new Date().toLocaleString('zh-CN', { hour12: false }),
      results
    });
  }
}

function getFixedTargetUrls() {
  return new Set(targetGroups().flatMap((group) => group.urls));
}

function getCopyUrl(button) {
  const candidates = [
    button.value,
    button.getAttribute('data-copy-value'),
    button.querySelector('.url-text')?.textContent?.trim()
  ];
  const allowed = getFixedTargetUrls();
  return candidates.find((value) => allowed.has(value)) || '';
}

async function copyText(value) {
  if (!value) throw new Error('没有可复制的 URL');

  if (navigator.clipboard && window.isSecureContext) {
    await navigator.clipboard.writeText(value);
    return;
  }

  const selection = window.getSelection();
  if (selection) selection.removeAllRanges();

  const textarea = document.createElement('textarea');
  textarea.value = value;
  textarea.setAttribute('readonly', '');
  textarea.style.position = 'fixed';
  textarea.style.left = '-9999px';
  document.body.appendChild(textarea);
  textarea.focus();
  textarea.select();
  textarea.setSelectionRange(0, value.length);
  const copied = document.execCommand('copy');
  textarea.remove();
  if (!copied) throw new Error('复制失败');
}

function showCopiedFeedback(button, text = '已复制') {
  const tip = button.querySelector('.copy-tip');
  if (tip) tip.textContent = text;
  document.querySelectorAll('.url-copy.copied').forEach((item) => item.classList.remove('copied'));
  button.classList.add('copied');
  window.clearTimeout(button._copiedTimer);
  button._copiedTimer = window.setTimeout(() => button.classList.remove('copied'), 1200);
}

function formatNetworkType() {
  const connection = navigator.connection || navigator.mozConnection || navigator.webkitConnection;
  if (!connection) return '浏览器未提供';

  const parts = [
    connection.type,
    connection.effectiveType,
    Number.isFinite(connection.downlink) ? `${connection.downlink} Mbps` : '',
    Number.isFinite(connection.rtt) ? `${connection.rtt} ms RTT` : ''
  ].filter(Boolean);

  return parts.length ? [...new Set(parts)].join(' / ') : '未知';
}

function updateModelCell(siteName, model, cell) {
  const row = state.modelStatus?.models?.find((item) => item.model === model);
  if (!row?.perSite || !cell) return;
  row.perSite[siteName] = cell;
}

async function testModelCell(siteName, model, group = '') {
  if (!siteName || !model) return;
  const key = `${siteName}\u0000${model}`;
  if (state.testingCells.has(key)) return;

  state.testingCells.add(key);
  state.modelTestMessage = '';
  render();

  try {
    const result = await postJson('/api/newapi/model-status/test', { siteName, model, group });
    updateModelCell(siteName, model, result.cell);
    state.modelTestMessage = `${siteName} / ${model} 测试完成：${result.test?.message || '测试完成'}`;
  } catch (error) {
    const row = state.modelStatus?.models?.find((item) => item.model === model);
    const cell = row?.perSite?.[siteName];
    if (cell && error.data?.nextAllowedAt) cell.nextTestAllowedAt = error.data.nextAllowedAt;
    state.modelTestMessage = `${siteName} / ${model} 测试失败：${error.message}`;
  } finally {
    state.testingCells.delete(key);
    render();
  }
}

function activatePanelTab(nextPanel, focusAfterRender = false) {
  if (state.activePanel === nextPanel) return;
  const shouldLoadModelStatus = nextPanel === 'models' && !state.modelStatus && !state.loading;
  state.activePanel = nextPanel || 'url';
  state.groupSelectOpen = false;
  if (shouldLoadModelStatus) {
    state.loading = true;
    state.error = '';
    state.modelTestMessage = '';
  }
  renderWithMotion('panel');
  if (focusAfterRender) {
    requestAnimationFrame(() => document.querySelector(`[data-panel="${nextPanel}"]`)?.focus());
  }
  if (shouldLoadModelStatus) loadModelStatus(false, { renderStart: false });
}

function activateModelSiteTab(siteName, focusAfterRender = false) {
  if (!siteName || state.selectedModelSite === siteName) return;
  state.selectedModelSite = siteName;
  state.modelFilter = '';
  state.modelTestMessage = '';
  state.groupSelectOpen = false;
  renderWithMotion('scope');
  if (focusAfterRender) {
    requestAnimationFrame(() => document.querySelector(`[data-model-site="${CSS.escape(siteName)}"]`)?.focus());
  }
}

function bindTabKeyboard(button, selector, activate) {
  button.addEventListener('keydown', (event) => {
    if (!['ArrowRight', 'ArrowDown', 'ArrowLeft', 'ArrowUp', 'Home', 'End'].includes(event.key)) return;
    event.preventDefault();
    const tabList = button.closest('[role="tablist"]') || document;
    const tabs = [...tabList.querySelectorAll(selector)];
    if (!tabs.length) return;
    const currentIndex = Math.max(0, tabs.indexOf(button));
    const nextIndex = event.key === 'Home'
      ? 0
      : event.key === 'End'
        ? tabs.length - 1
        : (currentIndex + (['ArrowRight', 'ArrowDown'].includes(event.key) ? 1 : -1) + tabs.length) % tabs.length;
    const nextTab = tabs[nextIndex];
    nextTab?.focus();
    activate(nextTab);
  });
}

function bindEvents() {
  const modelStatusFilter = document.getElementById('modelStatusFilter');
  const tokenGroupTrigger = document.querySelector('[data-token-group-trigger]');
  const runConnectivityBtn = document.getElementById('runConnectivityBtn');
  const connectivityResultGroups = document.getElementById('connectivityResultGroups');
  const toggleButtons = document.querySelectorAll('[data-panel]');

  modelStatusFilter?.addEventListener('input', (event) => {
    state.modelFilter = event.target.value;
    const cursor = event.target.selectionStart;
    window.clearTimeout(filterRenderTimer);
    filterRenderTimer = window.setTimeout(() => {
      render();
      const nextInput = document.getElementById('modelStatusFilter');
      nextInput?.focus();
      if (Number.isFinite(cursor)) nextInput?.setSelectionRange(cursor, cursor);
    }, 120);
  });

  runConnectivityBtn?.addEventListener('click', runConnectivityTests);

  document.querySelectorAll('[data-model-site]').forEach((button) => {
    button.addEventListener('click', () => {
      activateModelSiteTab(button.dataset.modelSite || '次数fufu');
    });
    bindTabKeyboard(button, '[data-model-site]', (nextTab) => {
      activateModelSiteTab(nextTab.dataset.modelSite || '次数fufu', true);
    });
  });

  tokenGroupTrigger?.addEventListener('click', () => {
    state.groupSelectOpen = !state.groupSelectOpen;
    renderWithMotion('select');
  });

  tokenGroupTrigger?.addEventListener('keydown', (event) => {
    if (!['Enter', ' ', 'ArrowDown'].includes(event.key)) return;
    event.preventDefault();
    state.groupSelectOpen = true;
    renderWithMotion('select');
    requestAnimationFrame(() => {
      const selected = document.querySelector('[data-token-group-option][aria-selected="true"]');
      (selected || document.querySelector('[data-token-group-option]'))?.focus();
    });
  });

  document.querySelectorAll('[data-token-group-option]').forEach((button) => {
    button.addEventListener('click', () => {
      state.selectedTokenGroup = button.dataset.tokenGroupOption || '';
      state.modelFilter = '';
      state.modelTestMessage = '';
      state.groupSelectOpen = false;
      renderWithMotion('scope');
    });
  });

  document.querySelectorAll('[data-token-group-option]').forEach((button) => {
    button.addEventListener('keydown', (event) => {
      if (!['ArrowDown', 'ArrowUp', 'Home', 'End', 'Escape'].includes(event.key)) return;
      event.preventDefault();
      const options = [...document.querySelectorAll('[data-token-group-option]')];
      const currentIndex = options.indexOf(button);
      if (event.key === 'Escape') {
        state.groupSelectOpen = false;
        renderWithMotion('select');
        requestAnimationFrame(() => document.querySelector('[data-token-group-trigger]')?.focus());
        return;
      }
      const nextIndex = event.key === 'Home'
        ? 0
        : event.key === 'End'
          ? options.length - 1
          : Math.max(0, Math.min(options.length - 1, currentIndex + (event.key === 'ArrowDown' ? 1 : -1)));
      options[nextIndex]?.focus();
    });
  });

  toggleButtons.forEach((button) => {
    button.addEventListener('click', () => {
      activatePanelTab(button.dataset.panel || 'url');
    });
    bindTabKeyboard(button, '[data-panel]', (nextTab) => {
      activatePanelTab(nextTab.dataset.panel || 'url', true);
    });
  });

  app.querySelectorAll('[data-model-test]').forEach((button) => {
    button.addEventListener('click', () => {
      testModelCell(button.dataset.site || '', button.dataset.model || '', button.dataset.group || '');
    });
  });

  connectivityResultGroups?.addEventListener('click', async (event) => {
    const button = event.target.closest('.url-copy');
    if (!button) return;

    try {
      await copyText(getCopyUrl(button));
      showCopiedFeedback(button);
    } catch {
      showCopiedFeedback(button, '复制失败');
    }
  });
}

document.addEventListener('pointerdown', (event) => {
  if (!state.groupSelectOpen) return;
  if (event.target.closest?.('[data-token-group-select]')) return;
  state.groupSelectOpen = false;
  renderWithMotion('select');
});

document.addEventListener('keydown', (event) => {
  if (!state.groupSelectOpen || event.key !== 'Escape') return;
  state.groupSelectOpen = false;
  renderWithMotion('select');
  requestAnimationFrame(() => document.querySelector('[data-token-group-trigger]')?.focus());
});

async function boot() {
  render();
  await loadStaticContext();
  render();
  window.setTimeout(runConnectivityTests, 180);
  await loadModelStatus();
  window.setInterval(() => loadModelStatus(), 10 * 60 * 1000);
  window.setInterval(() => {
    if (state.activePanel === 'models') render();
  }, 60 * 1000);
}

boot();
