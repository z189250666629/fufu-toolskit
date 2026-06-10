import test from 'node:test';
import assert from 'node:assert/strict';

import { initTheme } from './theme.mjs';

function createThemeHarness({ storedTheme = null, systemDark = false } = {}) {
  const html = {
    attrs: {},
    setAttribute(name, value) {
      this.attrs[name] = value;
    },
    getAttribute(name) {
      return this.attrs[name] ?? null;
    }
  };
  const icon = { textContent: '' };
  const listeners = {};
  const btn = {
    addEventListener(type, handler) {
      listeners[type] = handler;
    }
  };
  const storageWrites = [];
  const storage = {
    value: storedTheme,
    getItem(key) {
      assert.equal(key, 'theme');
      return this.value;
    },
    setItem(key, value) {
      assert.equal(key, 'theme');
      this.value = value;
      storageWrites.push(value);
    }
  };
  const mediaListeners = {};
  const media = {
    matches: systemDark,
    addEventListener(type, handler) {
      mediaListeners[type] = handler;
    }
  };

  initTheme({
    html,
    button: btn,
    icon,
    storage,
    matchMedia: () => media
  });

  return { html, icon, listeners, mediaListeners, storage, storageWrites };
}

test('y2k theme follows system preference until user toggles', () => {
  const harness = createThemeHarness({ systemDark: true });

  assert.equal(harness.html.getAttribute('data-theme'), 'dark');
  assert.equal(harness.icon.textContent, '☀');
  assert.deepEqual(harness.storageWrites, []);

  harness.mediaListeners.change({ matches: false });

  assert.equal(harness.html.getAttribute('data-theme'), 'light');
  assert.equal(harness.icon.textContent, '☽');
  assert.deepEqual(harness.storageWrites, []);

  harness.listeners.click();

  assert.equal(harness.html.getAttribute('data-theme'), 'dark');
  assert.deepEqual(harness.storageWrites, ['dark']);

  harness.mediaListeners.change({ matches: false });

  assert.equal(harness.html.getAttribute('data-theme'), 'dark');
  assert.deepEqual(harness.storageWrites, ['dark']);
});
