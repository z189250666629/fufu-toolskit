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

test('fetchJson throws client API error with status and data', async () => {
  await assert.rejects(
    () => fetchJson('/api/test', {
      fetchImpl: async () => ({
        ok: false,
        status: 400,
        json: async () => ({ error: '请求参数不正确' })
      })
    }),
    (error) => {
      assert.equal(error.message, '请求参数不正确');
      assert.equal(error.status, 400);
      assert.deepEqual(error.data, { error: '请求参数不正确' });
      return true;
    }
  );
});

test('fetchJson masks server API error payload details', async () => {
  await assert.rejects(
    () => fetchJson('/api/test', {
      fetchImpl: async () => ({
        ok: false,
        status: 502,
        json: async () => ({ error: 'sql: no such table tokens; stack=internal' })
      })
    }),
    (error) => {
      assert.equal(error.message, '服务暂时不可用，请稍后重试');
      assert.equal(error.status, 502);
      assert.deepEqual(error.data, {});
      assert.doesNotMatch(error.message, /sql|table|stack|tokens/i);
      assert.doesNotMatch(JSON.stringify(error.data), /sql|table|stack|tokens/i);
      return true;
    }
  );
});

test('fetchJson throws when successful API response is not valid JSON', async () => {
  await assert.rejects(
    () => fetchJson('/api/test', {
      fetchImpl: async () => ({
        ok: true,
        status: 200,
        json: async () => {
          throw new SyntaxError('Unexpected token <');
        }
      })
    }),
    (error) => {
      assert.equal(error.message, '响应不是有效 JSON');
      assert.equal(error.status, 200);
      assert.deepEqual(error.data, {});
      return true;
    }
  );
});

test('fetchJson masks server error status when error body is not JSON', async () => {
  await assert.rejects(
    () => fetchJson('/api/test', {
      fetchImpl: async () => ({
        ok: false,
        status: 502,
        json: async () => {
          throw new SyntaxError('Unexpected token <');
        }
      })
    }),
    (error) => {
      assert.equal(error.message, '服务暂时不可用，请稍后重试');
      assert.equal(error.status, 502);
      assert.deepEqual(error.data, {});
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
