import test from 'node:test';
import assert from 'node:assert/strict';

import {
  buildConnectivityErrorState,
  buildNoTargetsState,
  buildRunningState,
  flattenConnectivityTargets
} from './connectivity_states.js';

test('flattenConnectivityTargets expands groups without mutating input', () => {
  const groups = [{ id: 'api', name: 'API', urls: ['https://a.test', 'https://b.test'] }];
  const targets = flattenConnectivityTargets(groups);

  assert.deepEqual(targets.map((target) => [target.group.id, target.url]), [
    ['api', 'https://a.test'],
    ['api', 'https://b.test']
  ]);
  assert.deepEqual(groups[0].urls, ['https://a.test', 'https://b.test']);
});

test('flattenConnectivityTargets trims URLs and drops empty or malformed groups', () => {
  const groups = [
    { id: 'api', name: 'API', urls: [' https://a.test ', '', '   '] },
    { id: 'bad', name: 'Bad', urls: 'https://bad.test' },
    { id: 'token', name: 'Token', urls: ['\thttps://token.test\n'] }
  ];

  const targets = flattenConnectivityTargets(groups);

  assert.deepEqual(targets.map((target) => [target.group.id, target.url]), [
    ['api', 'https://a.test'],
    ['token', 'https://token.test']
  ]);
  assert.deepEqual(groups[0].urls, [' https://a.test ', '', '   ']);
});

test('connectivity base states expose stable UI copy', () => {
 assert.equal(buildRunningState().title, '测试中');
 assert.equal(buildRunningState().running, true);
 assert.equal(buildNoTargetsState('now').title, '没有测试目标');

  const errState = buildConnectivityErrorState(new Error('浏览器执行检测时发生异常。'), [{ reachable: false }], 'done');
  assert.equal(errState.title, '测试异常');
  assert.equal(errState.text, '浏览器执行检测时发生异常。');
  assert.equal(errState.testedAt, 'done');
  assert.deepEqual(errState.results, [{ reachable: false }]);
});

test('buildConnectivityErrorState masks raw exception details', () => {
  const errState = buildConnectivityErrorState(new Error('failed https://10.0.0.5?token=sk-secret'), [], 'done');

  assert.equal(errState.title, '测试异常');
  assert.equal(errState.text, '浏览器执行检测时发生异常。');
  for (const leaked of ['10.0.0.5', 'sk-secret', 'token=', 'failed']) {
    assert.ok(!errState.text.includes(leaked), `error state leaked ${leaked}: ${errState.text}`);
  }
});
