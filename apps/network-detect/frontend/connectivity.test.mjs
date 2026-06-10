import test from 'node:test';
import assert from 'node:assert/strict';

import {
  DEFAULT_TARGET_GROUPS,
  addCacheBust,
  fetchErrorText,
  fetchWithTimeout,
  normalizeTargetGroups
} from './connectivity.js';

test('normalizes target groups and falls back to defaults', () => {
  const custom = normalizeTargetGroups([
    { id: 'api', name: 'API', urls: ['https://a.example', '', 123] },
    { id: '', name: '', urls: [] }
  ]);

  assert.deepEqual(custom, [
    { id: 'api', name: 'API', urls: ['https://a.example'] }
  ]);

  const fallback = normalizeTargetGroups([]);
  assert.equal(fallback.length, DEFAULT_TARGET_GROUPS.length);
  assert.equal(fallback[0].id, 'api');
});

test('normalizes target groups falls back to defaults after dropping groups without usable URLs', () => {
  const fallback = normalizeTargetGroups([
    { id: 'empty', name: 'Empty', urls: [] },
    { id: 'blank', name: 'Blank', urls: [' ', '\n\t'] }
  ]);

  assert.equal(fallback.length, DEFAULT_TARGET_GROUPS.length);
  assert.deepEqual(fallback.map((group) => group.id), DEFAULT_TARGET_GROUPS.map((group) => group.id));
});

test('normalizeTargetGroups trims target URLs before filtering', () => {
  const custom = normalizeTargetGroups([
    { id: 'api', name: 'API', urls: [' https://a.example ', '  '] }
  ]);

  assert.deepEqual(custom, [
    { id: 'api', name: 'API', urls: ['https://a.example'] }
  ]);
});

test('normalizeTargetGroups drops unsafe targets and strips URL details', () => {
  const custom = normalizeTargetGroups([
    {
      id: 'api',
      name: 'API',
      urls: [
        'http://10.0.0.5:3000',
        'http://127.0.0.1:8080',
        'http://localhost:8080',
        'http://0.0.0.0:8080',
        'http://169.254.1.1',
        'http://[::1]:8080',
        'C:\\secret\\config.json',
        'https://api.example.test/path?token=sk-secret#debug'
      ]
    }
  ]);

  assert.deepEqual(custom, [
    { id: 'api', name: 'API', urls: ['https://api.example.test'] }
  ]);
});

test('adds cache bust query without dropping existing URL parts', () => {
  const got = new URL(addCacheBust('https://api.example.test/v1?x=1', 'https://panel.example.test/'));
  assert.equal(got.origin, 'https://api.example.test');
  assert.equal(got.pathname, '/v1');
  assert.equal(got.searchParams.get('x'), '1');
  assert.match(got.searchParams.get('_fufu_connect_test'), /^\d+_[a-f0-9.]+$/);
});

test('maps fetch errors to user-facing text', () => {
  assert.equal(fetchErrorText({ name: 'AbortError' }), '请求超时');
  assert.equal(fetchErrorText(new Error('network')), '请求失败或被浏览器拦截');
});

test('fetchWithTimeout passes an abort signal and clears timer', async () => {
  const calls = [];
  const response = await fetchWithTimeout(
    'https://api.example.test',
    { method: 'GET' },
    100,
    {
      fetchImpl: async (url, options) => {
        calls.push(['fetch', url, Boolean(options.signal), options.method]);
        return { ok: true };
      },
      setTimeoutImpl: () => 7,
      clearTimeoutImpl: (timer) => calls.push(['clear', timer])
    }
  );

  assert.deepEqual(response, { ok: true });
  assert.deepEqual(calls, [
    ['fetch', 'https://api.example.test', true, 'GET'],
    ['clear', 7]
  ]);
});
