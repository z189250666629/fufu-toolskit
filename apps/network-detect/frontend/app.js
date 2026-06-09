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
  renderEnvironment,
  renderHeader,
  renderMonitorPanel
} from './app_shell.js';
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
    ${renderHeader({ modelStatus: state.modelStatus, client: state.client })}
    ${renderMonitorPanel({
      state,
      groups: targetGroups(),
      panelMotionClass: motionClass('panel'),
      scopeMotionClass: motionClass('scope')
    })}
    ${state.loading && !state.modelStatus ? '<div class="boot-state">正在读取模型状态</div>' : ''}
    ${renderEnvironment({
      client: state.client,
      browserTime: new Date().toLocaleString('zh-CN', { hour12: false }),
      online: navigator.onLine,
      timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
      networkType: formatNetworkType()
    })}
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
