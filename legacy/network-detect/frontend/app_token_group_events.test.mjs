import test from 'node:test';
import assert from 'node:assert/strict';

import { bindTokenGroupSelectEvents } from './app_token_group_events.js';

test('bindTokenGroupSelectEvents selects options and restores trigger focus on Escape', () => {
  const motions = [];
  const focused = [];
  const prevented = [];
  const listeners = new Map();
  const trigger = {
    addEventListener: (type, handler) => listeners.set(`trigger:${type}`, handler),
    focus: () => focused.push('trigger')
  };
  const options = ['vip', 'default'].map((name) => ({
    dataset: { tokenGroupOption: name },
    addEventListener: (type, handler) => listeners.set(`${name}:${type}`, handler),
    focus: () => focused.push(name)
  }));
  const documentRef = {
    querySelector: (selector) => selector === '[data-token-group-trigger]' ? trigger : null,
    querySelectorAll: (selector) => selector === '[data-token-group-option]' ? options : []
  };
  const state = { groupSelectOpen: true, selectedTokenGroup: '', modelFilter: 'model', modelTestMessage: 'msg' };

  bindTokenGroupSelectEvents({
    documentRef,
    windowRef: { requestAnimationFrame: (callback) => callback() },
    state,
    renderWithMotion: (motion) => motions.push(motion)
  });

  listeners.get('vip:click')();
  assert.equal(state.selectedTokenGroup, 'vip');
  assert.equal(state.groupSelectOpen, false);
  assert.deepEqual(motions, ['scope']);

  state.groupSelectOpen = true;
  listeners.get('default:keydown')({
    key: 'Escape',
    preventDefault: () => prevented.push('escape')
  });

  assert.equal(state.groupSelectOpen, false);
  assert.deepEqual(prevented, ['escape']);
  assert.deepEqual(focused, ['trigger']);
  assert.deepEqual(motions, ['scope', 'select']);
});
