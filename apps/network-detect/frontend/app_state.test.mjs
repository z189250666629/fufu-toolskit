import test from 'node:test';
import assert from 'node:assert/strict';

import {
  activateModelSiteState,
  activatePanelState,
  createInitialAppState,
  selectTokenGroupState
} from './app_state.js';

test('createInitialAppState returns independent default app state', () => {
  const first = createInitialAppState();
  const second = createInitialAppState();

  assert.equal(first.activePanel, 'url');
  assert.equal(first.selectedModelSite, '次数fufu');
  assert.equal(first.connectivityTargetError, '');
  assert.equal(first.connectivity.title, '等待测试');
  assert.equal(first.connectivity.results.length, 0);
  assert.ok(first.testingCells instanceof Set);

  first.testingCells.add('site\u0000model');
  first.connectivity.results.push({ url: 'https://api.test' });

  assert.equal(second.testingCells.size, 0);
  assert.equal(second.connectivity.results.length, 0);
});

test('activatePanelState switches panel and prepares lazy model loading', () => {
  const state = {
    activePanel: 'url',
    modelStatus: null,
    loading: false,
    error: 'old',
    modelTestMessage: 'old',
    groupSelectOpen: true
  };

  const result = activatePanelState(state, 'models');

  assert.deepEqual(result, { changed: true, shouldLoadModelStatus: true, panel: 'models' });
  assert.equal(state.activePanel, 'models');
  assert.equal(state.loading, true);
  assert.equal(state.error, '');
  assert.equal(state.modelTestMessage, '');
  assert.equal(state.groupSelectOpen, false);
});

test('activatePanelState no-ops when panel is unchanged', () => {
  const state = { activePanel: 'url', groupSelectOpen: true };
  const result = activatePanelState(state, 'url');

  assert.deepEqual(result, { changed: false, shouldLoadModelStatus: false, panel: 'url' });
  assert.equal(state.groupSelectOpen, true);
});

test('activatePanelState falls back to url panel', () => {
  const state = {
    activePanel: 'models',
    modelStatus: {},
    loading: false,
    error: 'old',
    modelTestMessage: 'old',
    groupSelectOpen: true
  };
  const result = activatePanelState(state, '');

  assert.deepEqual(result, { changed: true, shouldLoadModelStatus: false, panel: 'url' });
  assert.equal(state.activePanel, 'url');
  assert.equal(state.groupSelectOpen, false);
  assert.equal(state.loading, false);
  assert.equal(state.error, 'old');
});

test('activateModelSiteState resets scoped filters when site changes', () => {
  const state = {
    selectedModelSite: '次数fufu',
    modelFilter: 'gpt',
    modelTestMessage: 'done',
    groupSelectOpen: true
  };

  assert.equal(activateModelSiteState(state, 'token-fufu'), true);
  assert.equal(state.selectedModelSite, 'token-fufu');
  assert.equal(state.modelFilter, '');
  assert.equal(state.modelTestMessage, '');
  assert.equal(state.groupSelectOpen, false);
  assert.equal(activateModelSiteState(state, 'token-fufu'), false);
  assert.equal(activateModelSiteState(state, ''), false);
});

test('selectTokenGroupState resets group-scoped model state', () => {
  const state = {
    selectedTokenGroup: 'old',
    modelFilter: 'gpt',
    modelTestMessage: 'done',
    groupSelectOpen: true
  };

  selectTokenGroupState(state, 'vip');

  assert.equal(state.selectedTokenGroup, 'vip');
  assert.equal(state.modelFilter, '');
  assert.equal(state.modelTestMessage, '');
  assert.equal(state.groupSelectOpen, false);
});
