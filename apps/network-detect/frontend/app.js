import {
  escapeHtml,
  idFragment,
  formatTime,
  formatServerTime,
  formatMs
} from './utils.js';
import {
  normalizeTargetGroups
} from './connectivity.js';
import {
  fetchJson,
  postJson
} from './api.js';
import {
  bindTabKeyboard,
  copyText,
  formatNetworkType,
  getCopyUrl,
  showCopiedFeedback
} from './dom.js';
import {
  renderUrlStatus
} from './render_connectivity.js';
import {
  renderModelAvailability
} from './render_models.js';
import {
  runConnectivityTests as runConnectivityTestSequence
} from './connectivity_runner.js';

const app = document.getElementById('app');
let filterRenderTimer = null;
let renderMotion = '';

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
  return normalizeTargetGroups(state.targets);
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
          ${state.activePanel === 'url' ? renderUrlStatus({ connectivity: state.connectivity, groups: targetGroups(), panelMotionClass: motionClass('panel') }) : renderModelAvailability({ state, panelMotionClass: motionClass('panel'), scopeMotionClass: motionClass('scope') })}
        </div>
      </div>
    </section>
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

async function runConnectivityTests() {
  await runConnectivityTestSequence({
    connectivity: state.connectivity,
    targetGroups: targetGroups(),
    setConnectivityState
  });
}

function getFixedTargetUrls() {
  return new Set(targetGroups().flatMap((group) => group.urls));
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
      await copyText(getCopyUrl(button, getFixedTargetUrls()));
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
