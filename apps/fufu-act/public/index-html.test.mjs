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

test('login flow masks server errors instead of rendering raw data.error', async () => {
  const source = await readFile(new URL('./index.html', import.meta.url), 'utf8');
  const loginStart = source.indexOf("$('btn-login').onclick = async () =>");
  const loginEnd = source.indexOf("$('card-input').onkeydown");
  const loginSource = source.slice(loginStart, loginEnd);

  assert.match(loginSource, /activityApi\.readApiJson\(res,\s*\{/);
  assert.match(loginSource, /serverErrorMessage: '登录失败，请稍后重试'/);
  assert.match(loginSource, /clientErrorMessage: 'INVALID KEY'/);
  assert.match(loginSource, /activityApi\.safeErrorMessage\(e,\s*'登录失败，请稍后重试'\)/);
  assert.doesNotMatch(loginSource, /const data = await res\.json\(\)/);
  assert.doesNotMatch(loginSource, /\$\('login-error'\)\.textContent = data\.error/);
  assert.doesNotMatch(loginSource, /\$\('login-error'\)\.textContent = 'NETWORK ERROR'/);
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

test('prize drawer uses backend weight metadata instead of hardcoded weights', async () => {
  const source = await readFile(new URL('./index.html', import.meta.url), 'utf8');
  const fetchPrizesStart = source.indexOf('async function fetchPrizes()');
  const fetchPrizesEnd = source.indexOf('function renderPrizeTable(rows)');
  const fetchPrizesSource = source.slice(fetchPrizesStart, fetchPrizesEnd);

  assert.match(fetchPrizesSource, /p\.weight/);
  assert.match(fetchPrizesSource, /p\.totalWeight/);
  assert.doesNotMatch(fetchPrizesSource, /const weights = \{/);
  assert.doesNotMatch(fetchPrizesSource, /1:\s*1100/);
});

test('history refresh marks stale state without clearing previous rows', async () => {
  const source = await readFile(new URL('./index.html', import.meta.url), 'utf8');
  const refreshStart = source.indexOf('async function refreshHistory()');
  const refreshEnd = source.indexOf('/* ============================================================\n       Logout');
  const refreshSource = source.slice(refreshStart, refreshEnd);

  assert.match(source, /id="history-status"/);
  assert.match(source, /function renderHistoryRefreshError\(\)/);
  assert.match(refreshSource, /activityApi\.readApiJson\(res,\s*\{/);
  assert.match(refreshSource, /serverErrorMessage: '历史暂未刷新'/);
  assert.match(refreshSource, /catch \(e\) \{\s*renderHistoryRefreshError\(\);\s*\}/);
  assert.match(source, /\$\('history-status'\)\.textContent = '';/);
  assert.doesNotMatch(refreshSource, /if \(res\.ok\) \{\s*const data = await res\.json\(\);/);
  assert.doesNotMatch(refreshSource, /catch \(e\) \{ \/\* silent \*\/ \}/);
});
