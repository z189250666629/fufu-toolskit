import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { Script, createContext } from 'node:vm';

async function loadActivityApi() {
  const source = await readFile(new URL('./activity-api.js', import.meta.url), 'utf8');
  const context = createContext({});
  new Script(source).runInContext(context);
  return context.activityApi;
}

function makeResponse(status, contentType, body) {
  return {
    ok: status >= 200 && status < 300,
    status,
    headers: {
      get(name) {
        return name.toLowerCase() === 'content-type' ? contentType : null;
      },
    },
    async json() {
      return JSON.parse(body);
    },
  };
}

test('spin API errors use safe fallback for proxy HTML and server JSON raw errors', async () => {
  const api = await loadActivityApi();
  const fallback = '抽奖失败，请稍后重试';
  const options = {
    serverErrorMessage: fallback,
    clientErrorMessage: '抽奖失败',
  };

  await assert.rejects(
    () => api.readApiJson(makeResponse(502, 'text/html', '<html>sql: down</html>'), options),
    (err) => {
      assert.equal(api.safeErrorMessage(err, fallback), fallback);
      assert.doesNotMatch(err.message, /<html|sql|Unexpected token/i);
      return true;
    },
  );

  await assert.rejects(
    () => api.readApiJson(makeResponse(500, 'application/json', '{"error":"sql: no such table credit_queue"}'), options),
    (err) => {
      assert.equal(api.safeErrorMessage(err, fallback), fallback);
      assert.doesNotMatch(err.message, /sql|credit_queue/i);
      return true;
    },
  );

  await assert.rejects(
    () => api.readApiJson(makeResponse(403, 'application/json', '{"error":"次数不足"}'), options),
    (err) => {
      assert.equal(api.safeErrorMessage(err, fallback), '次数不足');
      return true;
    },
  );

  assert.equal(api.safeErrorMessage(new Error('Unexpected token < in JSON'), fallback), fallback);
});
