import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';

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
