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
import {
  runModelCellTest
} from './model_test_runner.js';
import {
  animateTabIndicators,
  captureRenderScroll,
  captureTabIndicatorRects,
  motionClass as tabMotionClass,
  restoreRenderScroll
} from './ui_motion.js';
import {
  activateModelSiteState,
  activatePanelState,
  selectTokenGroupState
} from './app_state.js';

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
  return tabMotionClass(renderMotion, ...types);
}

function render() {
  const scrollSnapshot = captureRenderScroll(state.activePanel);
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
  restoreRenderScroll(scrollSnapshot, state.activePanel);
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

async function testModelCell(siteName, model, group = '') {
  await runModelCellTest({
    state,
    siteName,
    model,
    group,
    postJsonImpl: postJson,
    render
  });
}

function activatePanelTab(nextPanel, focusAfterRender = false) {
  const { changed, shouldLoadModelStatus, panel } = activatePanelState(state, nextPanel);
  if (!changed) return;
  renderWithMotion('panel');
  if (focusAfterRender) {
    requestAnimationFrame(() => document.querySelector(`[data-panel="${panel}"]`)?.focus());
  }
  if (shouldLoadModelStatus) loadModelStatus(false, { renderStart: false });
}

function activateModelSiteTab(siteName, focusAfterRender = false) {
  if (!activateModelSiteState(state, siteName)) return;
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
      selectTokenGroupState(state, button.dataset.tokenGroupOption || '');
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
