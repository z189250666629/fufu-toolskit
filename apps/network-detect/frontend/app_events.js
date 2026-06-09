import {
  bindTabKeyboard,
  copyText,
  getCopyUrl,
  showCopiedFeedback
} from './dom.js';
import {
  bindTokenGroupSelectEvents
} from './app_token_group_events.js';

export { nextTokenGroupOptionIndex } from './app_event_keys.js';

let filterRenderTimer = null;

export function bindAppEvents({
  documentRef = document,
  windowRef = window,
  appElement,
  state,
  targetGroups,
  render,
  renderWithMotion,
  runConnectivityTests,
  activateModelSiteTab,
  activatePanelTab,
  testModelCell
}) {
  const modelStatusFilter = documentRef.getElementById('modelStatusFilter');
  const runConnectivityBtn = documentRef.getElementById('runConnectivityBtn');
  const connectivityResultGroups = documentRef.getElementById('connectivityResultGroups');
  const toggleButtons = documentRef.querySelectorAll('[data-panel]');

  const fixedTargetUrls = () => new Set(targetGroups().flatMap((group) => group.urls));

  modelStatusFilter?.addEventListener('input', (event) => {
    state.modelFilter = event.target.value;
    const cursor = event.target.selectionStart;
    windowRef.clearTimeout(filterRenderTimer);
    filterRenderTimer = windowRef.setTimeout(() => {
      render();
      const nextInput = documentRef.getElementById('modelStatusFilter');
      nextInput?.focus();
      if (Number.isFinite(cursor)) nextInput?.setSelectionRange(cursor, cursor);
    }, 120);
  });

  runConnectivityBtn?.addEventListener('click', runConnectivityTests);

  documentRef.querySelectorAll('[data-model-site]').forEach((button) => {
    button.addEventListener('click', () => {
      activateModelSiteTab(button.dataset.modelSite || '次数fufu');
    });
    bindTabKeyboard(button, '[data-model-site]', (nextTab) => {
      activateModelSiteTab(nextTab.dataset.modelSite || '次数fufu', true);
    });
  });

  bindTokenGroupSelectEvents({
    documentRef,
    windowRef,
    state,
    renderWithMotion
  });

  toggleButtons.forEach((button) => {
    button.addEventListener('click', () => {
      activatePanelTab(button.dataset.panel || 'url');
    });
    bindTabKeyboard(button, '[data-panel]', (nextTab) => {
      activatePanelTab(nextTab.dataset.panel || 'url', true);
    });
  });

  appElement.querySelectorAll('[data-model-test]').forEach((button) => {
    button.addEventListener('click', () => {
      testModelCell(button.dataset.site || '', button.dataset.model || '', button.dataset.group || '');
    });
  });

  connectivityResultGroups?.addEventListener('click', async (event) => {
    const button = event.target.closest('.url-copy');
    if (!button) return;

    try {
      await copyText(getCopyUrl(button, fixedTargetUrls()));
      showCopiedFeedback(button);
    } catch {
      showCopiedFeedback(button, '复制失败');
    }
  });
}

export function bindGlobalAppEvents({
  documentRef = document,
  windowRef = window,
  state,
  renderWithMotion
}) {
  const requestFrame = windowRef.requestAnimationFrame?.bind(windowRef) || ((callback) => callback());

  documentRef.addEventListener('pointerdown', (event) => {
    if (!state.groupSelectOpen) return;
    if (event.target.closest?.('[data-token-group-select]')) return;
    state.groupSelectOpen = false;
    renderWithMotion('select');
  });

  documentRef.addEventListener('keydown', (event) => {
    if (!state.groupSelectOpen || event.key !== 'Escape') return;
    state.groupSelectOpen = false;
    renderWithMotion('select');
    requestFrame(() => documentRef.querySelector('[data-token-group-trigger]')?.focus());
  });
}
