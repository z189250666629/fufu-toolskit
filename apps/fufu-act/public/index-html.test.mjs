import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';

test('scratch test-card flag uses shared standalone-marker helper', async () => {
  const source = await readFile(new URL('./index.html', import.meta.url), 'utf8');

  assert.match(source, /<script src="scratch-card\.js"><\/script>/);
  assert.match(source, /scratchIsTest = scratchCard\.isTestCardName\(data\.cardName\)/);
  assert.doesNotMatch(source, /scratchIsTest = .*includes\('test'\)/);
});

test('spin flow uses safe API parsing instead of exposing raw response errors', async () => {
  const source = await readFile(new URL('./index.html', import.meta.url), 'utf8');

  assert.match(source, /<script src="activity-api\.js"><\/script>/);
  assert.match(source, /activityApi\.readApiJson\(res,\s*\{/);
  assert.match(source, /activityApi\.safeErrorMessage\(err,\s*'抽奖失败，请稍后重试'\)/);
  assert.doesNotMatch(source, /data = await res\.json\(\);\s*if \(!res\.ok\) throw new Error\(data\.error/);
  assert.doesNotMatch(source, /\$\('result'\)\.textContent = err\.message \|\| '网络错误'/);
});

test('prize drawer reports load failures instead of silently swallowing them', async () => {
  const source = await readFile(new URL('./index.html', import.meta.url), 'utf8');
  const fetchPrizesStart = source.indexOf('async function fetchPrizes()');
  const fetchPrizesEnd = source.indexOf('function renderPrizeTable(rows)');
  const fetchPrizesSource = source.slice(fetchPrizesStart, fetchPrizesEnd);

  assert.match(source, /function renderPrizeLoadError\(\)/);
  assert.match(source, /serverErrorMessage: '奖池暂时无法加载'/);
  assert.match(fetchPrizesSource, /catch \(e\) \{\s*renderPrizeLoadError\(\);\s*\}/);
  assert.doesNotMatch(fetchPrizesSource, /if \(!res\.ok\) return;/);
  assert.doesNotMatch(fetchPrizesSource, /catch \(e\) \{ \/\* silent \*\/ \}/);
});
