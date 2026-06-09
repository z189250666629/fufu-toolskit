import {
  normalizeTargetGroups
} from './connectivity.js';
import {
  fetchJson,
  postJson
} from './api.js';
import {
  loadModelStatusState,
  loadStaticContextState
} from './app_data.js';
import {
  formatNetworkType,
} from './dom.js';
import {
  renderEnvironment,
  renderHeader,
  renderMonitorPanel
} from './app_shell.js';
import {
  bindAppEvents,
  bindGlobalAppEvents
} from './app_events.js';
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
  createInitialAppState,
} from './app_state.js';

const app = document.getElementById('app');
let renderMotion = '';

const state = createInitialAppState();

function targetGroups() {
  return normalizeTargetGroups(state.targets);
}

async function loadStaticContext() {
  await loadStaticContextState(state, fetchJson);
}

async function loadModelStatus(refresh = false, options = {}) {
  await loadModelStatusState(state, {
    refresh,
    renderStart: options.renderStart !== false,
    fetchJsonImpl: fetchJson,
    render
  });
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
  bindAppEvents({
    appElement: app,
    state,
    targetGroups,
    render,
    renderWithMotion,
    runConnectivityTests,
    activateModelSiteTab,
    activatePanelTab,
    testModelCell
  });
}

bindGlobalAppEvents({ state, renderWithMotion });

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
