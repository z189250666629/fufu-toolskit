import test from 'node:test';
import assert from 'node:assert/strict';

import { fetchJson, postJson } from './api.js';

test('fetchJson requests no-store JSON and returns decoded body', async () => {
  const calls = [];
  const data = await fetchJson('/api/test', {
    headers: { 'X-Test': 'yes' },
    fetchImpl: async (path, options) => {
      calls.push([path, options.cache, options.headers['X-Test']]);
      return {
        ok: true,
        json: async () => ({ ok: true })
      };
    }
  });

  assert.deepEqual(data, { ok: true });
  assert.deepEqual(calls, [['/api/test', 'no-store', 'yes']]);
});

test('fetchJson throws API error with status and data', async () => {
  await assert.rejects(
    () => fetchJson('/api/test', {
      fetchImpl: async () => ({
        ok: false,
        status: 503,
        json: async () => ({ error: 'bad gateway' })
      })
    }),
    (error) => {
      assert.equal(error.message, 'bad gateway');
      assert.equal(error.status, 503);
      assert.deepEqual(error.data, { error: 'bad gateway' });
      return true;
    }
  );
});

test('postJson sends JSON body and content type', async () => {
  const calls = [];
  await postJson('/api/test', { hello: 'world' }, {
    fetchImpl: async (path, options) => {
      calls.push([path, options.method, options.headers['Content-Type'], options.body]);
      return {
        ok: true,
        json: async () => ({ ok: true })
      };
    }
  });

  assert.deepEqual(calls, [['/api/test', 'POST', 'application/json', '{"hello":"world"}']]);
});
