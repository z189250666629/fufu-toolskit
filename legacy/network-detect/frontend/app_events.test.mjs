import test from 'node:test';
import assert from 'node:assert/strict';

import {
  bindActivatableTabs,
  bindAppEvents,
  nextTokenGroupOptionIndex as nextTokenGroupOptionIndexFromAppEvents
} from './app_events.js';
import {
  nextTokenGroupOptionIndex
} from './app_event_keys.js';

test('app events re-exports token group option navigation helper', () => {
  assert.equal(nextTokenGroupOptionIndexFromAppEvents, nextTokenGroupOptionIndex);
});

test('bindActivatableTabs wires click activation and keyboard activation', () => {
  const calls = [];
  const focused = [];
  const prevented = [];
  const listeners = new Map();
  const buttons = [
    { dataset: { panel: '' }, name: 'fallback' },
    { dataset: { panel: 'models' }, name: 'models' }
  ].map((button) => ({
    ...button,
    addEventListener: (type, handler) => listeners.set(`${button.name}:${type}`, handler),
    closest: () => ({ querySelectorAll: () => buttons }),
    focus: () => focused.push(button.name)
  }));

  bindActivatableTabs({
    buttons,
    selector: '[data-panel]',
    fallbackValue: 'url',
    getValue: (button) => button.dataset.panel,
    activate: (...args) => calls.push(args)
  });

  listeners.get('fallback:click')();
  listeners.get('fallback:keydown')({
    key: 'ArrowRight',
    preventDefault: () => prevented.push('prevented')
  });

  assert.deepEqual(calls, [['url'], ['models', true]]);
  assert.deepEqual(prevented, ['prevented']);
  assert.deepEqual(focused, ['models']);
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
