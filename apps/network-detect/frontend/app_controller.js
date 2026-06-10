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

function defaultCssEscape(value) {
  if (globalThis.CSS?.escape) return globalThis.CSS.escape(value);
  return String(value).replace(/\\/g, '\\\\').replace(/"/g, '\\"');
}

export function createDashboardApp(deps = {}) {
  const {
    documentRef = globalThis.document,
    windowRef = globalThis.window,
    navigatorRef = globalThis.navigator,
    intlRef = globalThis.Intl,
    appElement,
    fetchJsonImpl = fetchJson,
    postJsonImpl = postJson,
    loadStaticContextStateImpl = loadStaticContextState,
    loadModelStatusStateImpl = loadModelStatusState,
    runConnectivityTestSequenceImpl = runConnectivityTestSequence,
    runModelCellTestImpl = runModelCellTest,
    createState = createInitialAppState,
    now = () => new Date(),
    cssEscape = defaultCssEscape
  } = deps;

  const app = appElement || documentRef.getElementById('app');
  const state = createState();
  let renderMotion = '';
  let globalEventsBound = false;

  function targetGroups() {
    return normalizeTargetGroups(state.targets);
  }

  async function loadStaticContext() {
    await loadStaticContextStateImpl(state, fetchJsonImpl);
  }

  async function loadModelStatus(refresh = false, options = {}) {
    await loadModelStatusStateImpl(state, {
      refresh,
      renderStart: options.renderStart !== false,
      fetchJsonImpl,
      render
    });
  }

  function motionClass(...types) {
    return tabMotionClass(renderMotion, ...types);
  }

  function render() {
    const scrollSnapshot = captureRenderScroll(state.activePanel, {
      document: documentRef,
      window: windowRef
    });
    documentRef.title = 'fufu API 状态面板';
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
      browserTime: now().toLocaleString('zh-CN', { hour12: false }),
      online: navigatorRef.onLine,
      timezone: intlRef.DateTimeFormat().resolvedOptions().timeZone,
      networkType: formatNetworkType(navigatorRef)
    })}
  `;

    bindEvents();
    restoreRenderScroll(scrollSnapshot, state.activePanel, {
      document: documentRef,
      window: windowRef
    });
  }

  function renderWithMotion(motion) {
    const previousRects = ['panel', 'scope'].includes(motion) ? captureTabIndicatorRects(documentRef) : null;
    renderMotion = '';
    render();
    animateTabIndicators(previousRects, {
      document: documentRef,
      window: windowRef
    });
  }

  function setConnectivityState(partial) {
    Object.assign(state.connectivity, partial);
    render();
  }

  function requestFrame(callback) {
    const requestAnimationFrameImpl = windowRef.requestAnimationFrame?.bind(windowRef) || ((frameCallback) => frameCallback());
    return requestAnimationFrameImpl(callback);
  }

  async function runConnectivityTests() {
    await runConnectivityTestSequenceImpl({
      connectivity: state.connectivity,
      targetGroups: targetGroups(),
      setConnectivityState
    });
  }

  async function testModelCell(siteName, model, group = '') {
    await runModelCellTestImpl({
      state,
      siteName,
      model,
      group,
      postJsonImpl,
      render
    });
  }

  function activatePanelTab(nextPanel, focusAfterRender = false) {
    const { changed, shouldLoadModelStatus, panel } = activatePanelState(state, nextPanel);
    if (!changed) return;
    renderWithMotion('panel');
    if (focusAfterRender) {
      requestFrame(() => documentRef.querySelector(`[data-panel="${panel}"]`)?.focus());
    }
    if (shouldLoadModelStatus) loadModelStatus(false, { renderStart: false });
  }

  function activateModelSiteTab(siteName, focusAfterRender = false) {
    if (!activateModelSiteState(state, siteName)) return;
    renderWithMotion('scope');
    if (focusAfterRender) {
      requestFrame(() => documentRef.querySelector(`[data-model-site="${cssEscape(siteName)}"]`)?.focus());
    }
  }

  function bindEvents() {
    bindAppEvents({
      documentRef,
      windowRef,
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

  function bindGlobalEvents() {
    if (globalEventsBound) return;
    bindGlobalAppEvents({ documentRef, windowRef, state, renderWithMotion });
    globalEventsBound = true;
  }

  async function boot() {
    bindGlobalEvents();
    render();
    await loadStaticContext();
    render();
    windowRef.setTimeout(runConnectivityTests, 180);
    await loadModelStatus();
    windowRef.setInterval(() => loadModelStatus(), 10 * 60 * 1000);
    windowRef.setInterval(() => {
      if (state.activePanel === 'models') render();
    }, 60 * 1000);
  }

  return {
    activateModelSiteTab,
    activatePanelTab,
    boot,
    loadModelStatus,
    loadStaticContext,
    render,
    runConnectivityTests,
    state,
    targetGroups,
    testModelCell
  };
}
