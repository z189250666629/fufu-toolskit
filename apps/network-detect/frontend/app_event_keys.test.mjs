import test from 'node:test';
import assert from 'node:assert/strict';

import {
  isTokenGroupOptionKey,
  isTokenGroupTriggerOpenKey,
  nextKeyboardNavigationIndex,
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

test('nextKeyboardNavigationIndex supports wrapping and clamped keyboard navigation', () => {
  const horizontalKeys = {
    forwardKeys: ['ArrowRight'],
    backwardKeys: ['ArrowLeft'],
    wrap: true
  };
  assert.equal(nextKeyboardNavigationIndex('ArrowRight', 2, 3, horizontalKeys), 0);
  assert.equal(nextKeyboardNavigationIndex('ArrowLeft', 0, 3, horizontalKeys), 2);
  assert.equal(nextKeyboardNavigationIndex('Home', 2, 3, horizontalKeys), 0);
  assert.equal(nextKeyboardNavigationIndex('End', 0, 3, horizontalKeys), 2);

  const verticalKeys = {
    forwardKeys: ['ArrowDown'],
    backwardKeys: ['ArrowUp'],
    wrap: false
  };
  assert.equal(nextKeyboardNavigationIndex('ArrowDown', 2, 3, verticalKeys), 2);
  assert.equal(nextKeyboardNavigationIndex('ArrowUp', 0, 3, verticalKeys), 0);
  assert.equal(nextKeyboardNavigationIndex('Enter', 1, 3, verticalKeys), null);
  assert.equal(nextKeyboardNavigationIndex('ArrowDown', 0, 0, verticalKeys), null);
});
