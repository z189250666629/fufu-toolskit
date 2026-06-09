import test from 'node:test';
import assert from 'node:assert/strict';

import {
  bindTabKeyboard,
  formatNetworkType,
  getCopyUrl
} from './dom.js';

test('getCopyUrl only returns fixed allowed URLs', () => {
  const button = {
    value: '',
    getAttribute: (name) => (name === 'data-copy-value' ? 'https://api.example.test' : ''),
    querySelector: () => ({ textContent: 'https://fallback.example.test' })
  };
  const allowed = new Set(['https://api.example.test']);

  assert.equal(getCopyUrl(button, allowed), 'https://api.example.test');
  assert.equal(getCopyUrl({ ...button, getAttribute: () => 'https://blocked.example.test' }, allowed), '');
});

test('formatNetworkType formats browser network connection details', () => {
  const navigatorLike = {
    connection: {
      type: 'wifi',
      effectiveType: '4g',
      downlink: 42,
      rtt: 30
    }
  };

  assert.equal(formatNetworkType(navigatorLike), 'wifi / 4g / 42 Mbps / 30 ms RTT');
  assert.equal(formatNetworkType({}), '浏览器未提供');
  assert.equal(formatNetworkType({ connection: {} }), '未知');
});

test('bindTabKeyboard moves focus and activates the next tab', () => {
  const focused = [];
  const activated = [];
  const tabs = ['first', 'second', 'third'].map((name) => ({
    name,
    focus: () => focused.push(name)
  }));
  const button = tabs[1];
  button.closest = () => ({
    querySelectorAll: () => tabs
  });
  const listeners = {};
  button.addEventListener = (type, handler) => {
    listeners[type] = handler;
  };

  bindTabKeyboard(button, '[role="tab"]', (nextTab) => activated.push(nextTab.name));
  listeners.keydown({
    key: 'ArrowRight',
    preventDefault: () => focused.push('prevented')
  });

  assert.deepEqual(focused, ['prevented', 'third']);
  assert.deepEqual(activated, ['third']);
});

test('bindTabKeyboard wraps focus and handles home and end keys', () => {
  const focused = [];
  const activated = [];
  const tabs = ['first', 'second', 'third'].map((name) => ({
    name,
    focus: () => focused.push(name)
  }));
  const button = tabs[0];
  button.closest = () => ({
    querySelectorAll: () => tabs
  });
  const listeners = {};
  button.addEventListener = (type, handler) => {
    listeners[type] = handler;
  };

  bindTabKeyboard(button, '[role="tab"]', (nextTab) => activated.push(nextTab.name));
  listeners.keydown({
    key: 'ArrowLeft',
    preventDefault: () => focused.push('prevented-left')
  });
  listeners.keydown({
    key: 'Home',
    preventDefault: () => focused.push('prevented-home')
  });
  listeners.keydown({
    key: 'End',
    preventDefault: () => focused.push('prevented-end')
  });

  assert.deepEqual(focused, ['prevented-left', 'third', 'prevented-home', 'first', 'prevented-end', 'third']);
  assert.deepEqual(activated, ['third', 'first', 'third']);
});

test('bindTabKeyboard ignores unsupported keys', () => {
  const focused = [];
  const activated = [];
  const button = {
    addEventListener: (type, handler) => {
      button[type] = handler;
    },
    closest: () => ({
      querySelectorAll: () => [button]
    }),
    focus: () => focused.push('focused')
  };

  bindTabKeyboard(button, '[role="tab"]', (nextTab) => activated.push(nextTab));
  button.keydown({
    key: 'Enter',
    preventDefault: () => focused.push('prevented')
  });

  assert.deepEqual(focused, []);
  assert.deepEqual(activated, []);
});
