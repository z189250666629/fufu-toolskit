import test from 'node:test';
import assert from 'node:assert/strict';

import {
  bindAppEvents,
  nextTokenGroupOptionIndex
} from './app_events.js';

test('nextTokenGroupOptionIndex maps keyboard navigation within bounds', () => {
  assert.equal(nextTokenGroupOptionIndex('ArrowDown', 0, 3), 1);
  assert.equal(nextTokenGroupOptionIndex('ArrowUp', 0, 3), 0);
  assert.equal(nextTokenGroupOptionIndex('End', 0, 3), 2);
  assert.equal(nextTokenGroupOptionIndex('Home', 2, 3), 0);
  assert.equal(nextTokenGroupOptionIndex('Enter', 1, 3), null);
  assert.equal(nextTokenGroupOptionIndex('ArrowDown', 0, 0), null);
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