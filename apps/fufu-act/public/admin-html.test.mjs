import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import vm from 'node:vm';

test('admin stats load uses safe API parsing instead of exposing proxy HTML or server 5xx errors', async () => {
  const source = await readFile(new URL('./admin.html', import.meta.url), 'utf8');
  const loadStart = source.indexOf('async function load()');
  const loadEnd = source.indexOf('function render(d)');
  const loadSource = source.slice(loadStart, loadEnd);

  assert.match(source, /<script src="activity-api\.js"><\/script>/);
  assert.match(loadSource, /activityApi\.readApiJson\(r,\s*\{/);
  assert.match(loadSource, /serverErrorMessage: '统计加载失败，请稍后重试'/);
  assert.match(loadSource, /activityApi\.safeErrorMessage\(e,\s*'统计加载失败，请稍后重试'\)/);
  assert.doesNotMatch(loadSource, /const j = await r\.json\(\); throw new Error\(j\.error \|\| r\.status\)/);
  assert.doesNotMatch(loadSource, /render\(await r\.json\(\)\)/);
  assert.doesNotMatch(loadSource, /errEl\.textContent = 'ERROR: ' \+ e\.message/);
});

test('admin stats uses Authorization header instead of query token', async () => {
  const source = await readFile(new URL('./admin.html', import.meta.url), 'utf8');
  const loadStart = source.indexOf('async function load()');
  const loadEnd = source.indexOf('function render(d)');
  const loadSource = source.slice(loadStart, loadEnd);

  assert.match(loadSource, /fetch\('\/api\/admin\/stats',\s*\{/);
  assert.match(loadSource, /Authorization:\s*'Bearer '\s*\+\s*tok/);
  assert.doesNotMatch(loadSource, /\/api\/admin\/stats\?token=/);
});

test('admin stats ignores stale overlapping load failures', async () => {
  const source = await readFile(new URL('./admin.html', import.meta.url), 'utf8');
  const inlineScript = source.match(/<script>\s*([\s\S]*)<\/script>\s*<\/body>/)?.[1];
  assert.ok(inlineScript, 'admin inline script should be extractable');

  const elements = new Map();
  const element = (id) => {
    if (!elements.has(id)) {
      elements.set(id, {
        id,
        textContent: '',
        innerHTML: '',
        value: '',
        addEventListener() {},
        getContext: () => ({
          clearRect() {},
          fillRect() {},
          set fillStyle(value) {},
          set globalAlpha(value) {}
        })
      });
    }
    return elements.get(id);
  };
  element('token').value = 'admin-token';

  const pending = [];
  const context = {
    console,
    innerWidth: 800,
    innerHeight: 600,
    Math,
    Date,
    setInterval: () => 1,
    clearInterval() {},
    requestAnimationFrame() {},
    window: { addEventListener() {} },
    document: {
      hidden: true,
      getElementById: element
    },
    fetch: async () => new Promise((resolve) => pending.push(resolve)),
    activityApi: {
      readApiJson: async (response) => {
        if (response.error) throw new Error(response.error);
        return response.data;
      },
      safeErrorMessage: (error, fallback) => error.message || fallback
    },
    adminRender: {
      buildStatsGridHtml: (data) => `stats:${data.version}`
    }
  };
  context.window = { ...context.window, document: context.document };
  vm.createContext(context);
  vm.runInContext(inlineScript, context);

  const first = context.load();
  const second = context.load();
  assert.equal(pending.length, 2);

  pending[1]({ data: { version: 'fresh' } });
  await second;
  assert.equal(element('content').innerHTML, 'stats:fresh');

  pending[0]({ error: 'stale failure' });
  await first;

  assert.equal(element('content').innerHTML, 'stats:fresh');
  assert.equal(element('err').textContent, '');
});
