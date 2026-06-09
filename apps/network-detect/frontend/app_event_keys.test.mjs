import test from 'node:test';
import assert from 'node:assert/strict';

import {
  isTokenGroupOptionKey,
  isTokenGroupTriggerOpenKey,
  nextTokenGroupOptionIndex
} from './app_event_keys.js';

test('token group event key helpers classify supported keys', () => {
  assert.equal(isTokenGroupTriggerOpenKey('Enter'), true);
  assert.equal(isTokenGroupTriggerOpenKey(' '), true);
  assert.equal(isTokenGroupTriggerOpenKey('ArrowDown'), true);
  assert.equal(isTokenGroupTriggerOpenKey('Escape'), false);

  assert.equal(isTokenGroupOptionKey('ArrowUp'), true);
  assert.equal(isTokenGroupOptionKey('End'), true);
  assert.equal(isTokenGroupOptionKey('Escape'), true);
  assert.equal(isTokenGroupOptionKey('Enter'), false);
});

test('nextTokenGroupOptionIndex maps keyboard navigation within bounds', () => {
  assert.equal(nextTokenGroupOptionIndex('ArrowDown', 0, 3), 1);
  assert.equal(nextTokenGroupOptionIndex('ArrowUp', 0, 3), 0);
  assert.equal(nextTokenGroupOptionIndex('End', 0, 3), 2);
  assert.equal(nextTokenGroupOptionIndex('Home', 2, 3), 0);
  assert.equal(nextTokenGroupOptionIndex('Enter', 1, 3), null);
  assert.equal(nextTokenGroupOptionIndex('ArrowDown', 0, 0), null);
});
