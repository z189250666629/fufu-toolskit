import test from 'node:test';
import assert from 'node:assert/strict';

import {
  buildConnectivityCompletionState,
  flattenConnectivityTargets,
  probeNoCors,
  runConnectivityTests,
  testConnectivityTarget
} from './connectivity_runner.js';

test('flattenConnectivityTargets expands groups in display order', () => {
  const targets = flattenConnectivityTargets([
    { id: 'api', name: 'API', urls: ['https://api-a.test', 'https://api-b.test'] },
    { id: 'token', name: 'Token', urls: ['https://token.test'] }
  ]);

  assert.deepEqual(targets.map(({ group, url }) => [group.id, url]), [
    ['api', 'https://api-a.test'],
    ['api', 'https://api-b.test'],
    ['token', 'https://token.test']
  ]);
});

test('probeNoCors samples URL reachability with no-cors fetches', async () => {
  const samples = [];
  const calls = [];
  const nowValues = [10, 30, 50, 90];
  let bustCount = 0;

  const result = await probeNoCors('https://api.example.test', (current, total) => {
    samples.push([current, total]);
  }, {
    sampleCount: 2,
    timeoutMs: 123,
    now: () => nowValues.shift(),
    addCacheBustImpl: (url) => `${url}?bust=${++bustCount}`,
    fetchWithTimeoutImpl: async (url, options, timeoutMs) => {
      calls.push([url, options.method, options.mode, options.cache, options.credentials, options.redirect, timeoutMs]);
    }
  });

  assert.deepEqual(samples, [[1, 2], [2, 2]]);
  assert.deepEqual(calls, [
    ['https://api.example.test?bust=1', 'GET', 'no-cors', 'no-store', 'omit', 'follow', 123],
    ['https://api.example.test?bust=2', 'GET', 'no-cors', 'no-store', 'omit', 'follow', 123]
  ]);
  assert.deepEqual(result, {
    ok: true,
    successRate: 1,
    averageMs: 30,
    lastError: ''
  });
});

test('testConnectivityTarget emits progress and normalizes probe result', async () => {
  const states = [];
  const result = await testConnectivityTarget({
    group: { id: 'api', name: 'API' },
    url: 'https://api.example.test',
    index: 1,
    total: 3,
    setConnectivityState: (partial) => states.push(partial),
    probeImpl: async (_url, onSample) => {
      onSample(2, 4);
      return { ok: true, successRate: 0.5, averageMs: 88, lastError: '' };
    }
  });

  assert.equal(states[0].progress, 30);
  assert.equal(states[0].progressText, '测试 API: https://api.example.test');
  assert.equal(states[1].progress, 45);
  assert.equal(states[1].progressText, 'API 采样 2/4');
  assert.deepEqual(result, {
    groupId: 'api',
    groupName: 'API',
    url: 'https://api.example.test',
    reachable: true,
    successRate: 0.5,
    averageMs: 88,
    lastError: ''
  });
});

test('buildConnectivityCompletionState describes full, partial and failed runs', () => {
  assert.deepEqual(
    buildConnectivityCompletionState([
      { reachable: true },
      { reachable: true }
    ], '2026-06-10 10:00:00'),
    {
      running: false,
      mode: 'complete',
      tone: 'ok',
      icon: 'OK',
      title: '全部可达',
      text: '当前用户浏览器可以访问全部 API 次数站和 Token 站 Base URL。',
      progress: 100,
      progressText: '测试完成',
      currentUrl: '全部目标测试完成',
      success: '100%',
      testedAt: '2026-06-10 10:00:00',
      results: [{ reachable: true }, { reachable: true }]
    }
  );

  assert.equal(buildConnectivityCompletionState([{ reachable: true }, { reachable: false }], 'now').tone, 'warn');
  assert.equal(buildConnectivityCompletionState([{ reachable: false }], 'now').title, '全部不可达');
});

test('runConnectivityTests handles empty targets and successful sequence', async () => {
  const emptyStates = [];
  const emptyResults = await runConnectivityTests({
    connectivity: { running: false },
    targetGroups: [],
    setConnectivityState: (partial) => emptyStates.push(partial),
    nowText: () => 'now'
  });

  assert.deepEqual(emptyResults, []);
  assert.equal(emptyStates.at(-1).title, '没有测试目标');

  const states = [];
  const results = await runConnectivityTests({
    connectivity: { running: false },
    targetGroups: [{ id: 'api', name: 'API', urls: ['https://api.example.test'] }],
    setConnectivityState: (partial) => states.push(partial),
    nowText: () => 'done',
    testTargetImpl: async ({ group, url }) => ({
      groupId: group.id,
      groupName: group.name,
      url,
      reachable: true,
      successRate: 1,
      averageMs: 20,
      lastError: ''
    })
  });

  assert.equal(results.length, 1);
  assert.equal(states[0].title, '测试中');
  assert.deepEqual(states[1].results, results);
  assert.equal(states.at(-1).title, '全部可达');
  assert.equal(states.at(-1).testedAt, 'done');
});
