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

test('connectivity base states expose stable UI copy', () => {
  assert.equal(buildRunningState().title, '测试中');
  assert.equal(buildRunningState().running, true);
  assert.equal(buildNoTargetsState('now').title, '没有测试目标');

  const errState = buildConnectivityErrorState(new Error('boom'), [{ reachable: false }], 'done');
  assert.equal(errState.title, '测试异常');
  assert.equal(errState.text, 'boom');
  assert.equal(errState.testedAt, 'done');
  assert.deepEqual(errState.results, [{ reachable: false }]);
});