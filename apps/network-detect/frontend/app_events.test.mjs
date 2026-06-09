import test from 'node:test';
import assert from 'node:assert/strict';

import {
  bindAppEvents,
  nextTokenGroupOptionIndex as nextTokenGroupOptionIndexFromAppEvents
} from './app_events.js';
import {
  nextTokenGroupOptionIndex
} from './app_event_keys.js';

test('app events re-exports token group option navigation helper', () => {
  assert.equal(nextTokenGroupOptionIndexFromAppEvents, nextTokenGroupOptionIndex);
});

test('bindAppEvents lets token group options move focus with arrow keys', () => {
  const focused = [];
  const prevented = [];
  const listeners = new Map();
  const options = ['vip', 'default'].map((name) => ({
    dataset: { tokenGroupOption: name },
    addEventListener: (type, handler) => listeners.set(`${name}:${type}`, handler),
    focus: () => focused.push(name)
  }));
  const documentRef = {
    getElementById: () => null,
    querySelector: () => null,
    querySelectorAll: (selector) => selector === '[data-token-group-option]' ? options : []
  };

  bindAppEvents({
    documentRef,
    windowRef: { clearTimeout: () => {}, setTimeout: () => 1 },
    appElement: { querySelectorAll: () => [] },
    state: { groupSelectOpen: true },
    targetGroups: () => [],
    render: () => {},
    renderWithMotion: () => {},
    runConnectivityTests: () => {},
    activateModelSiteTab: () => {},
    activatePanelTab: () => {},
    testModelCell: () => {}
  });

  listeners.get('vip:keydown')({
    key: 'ArrowDown',
    preventDefault: () => prevented.push('prevented')
  });

  assert.deepEqual(prevented, ['prevented']);
  assert.deepEqual(focused, ['default']);
});
