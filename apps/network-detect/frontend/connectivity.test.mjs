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
    { id: 'api', name: 'API', urls: ['https://a.example', '123'] }
  ]);

  const fallback = normalizeTargetGroups([]);
  assert.equal(fallback.length, DEFAULT_TARGET_GROUPS.length);
  assert.equal(fallback[0].id, 'api');
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
