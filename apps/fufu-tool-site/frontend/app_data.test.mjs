import test from 'node:test';
import assert from 'node:assert/strict';

import {
  loadModelStatusState,
  loadStaticContextState
} from './app_data.js';

test('loadStaticContextState stores client and target groups with fallbacks', async () => {
  const state = { client: null, clientLoadError: 'old client error', targets: [], connectivityTargetError: 'old error' };
  const calls = [];

  await loadStaticContextState(state, async (path) => {
    calls.push(path);
    if (path === '/api/client') return { ip: '127.0.0.1' };
    if (path === '/api/connectivity/targets') return { groups: [{ id: 'api', urls: ['https://api.test'] }] };
    throw new Error(`unexpected ${path}`);
  });

  assert.deepEqual(calls.sort(), ['/api/client', '/api/connectivity/targets'].sort());
  assert.deepEqual(state.client, { ip: '127.0.0.1' });
  assert.equal(state.clientLoadError, '');
  assert.deepEqual(state.targets, [{ id: 'api', urls: ['https://api.test'] }]);
  assert.equal(state.connectivityTargetError, '');
});

test('loadStaticContextState tolerates failed optional requests', async () => {
  const state = { client: { old: true }, clientLoadError: '', targets: [{ old: true }], connectivityTargetError: '' };

  await loadStaticContextState(state, async () => {
    throw new Error('offline');
  });

  assert.equal(state.client, null);
  assert.equal(state.clientLoadError, 'offline');
  assert.deepEqual(state.targets, []);
  assert.equal(state.connectivityTargetError, 'offline');
});

test('loadStaticContextState records connectivity targets error without losing client context', async () => {
  const state = { client: null, clientLoadError: '', targets: [{ old: true }], connectivityTargetError: '' };

  await loadStaticContextState(state, async (path) => {
    if (path === '/api/client') return { ip: '127.0.0.1' };
    if (path === '/api/connectivity/targets') throw new Error('CONNECTIVITY_TARGETS 不是有效 JSON');
    throw new Error(`unexpected ${path}`);
  });

  assert.deepEqual(state.client, { ip: '127.0.0.1' });
  assert.equal(state.clientLoadError, '');
  assert.deepEqual(state.targets, []);
  assert.equal(state.connectivityTargetError, 'CONNECTIVITY_TARGETS 不是有效 JSON');
});

test('loadStaticContextState records client load error without blocking targets', async () => {
  const state = { client: { old: true }, clientLoadError: '', targets: [], connectivityTargetError: '' };

  await loadStaticContextState(state, async (path) => {
    if (path === '/api/client') throw new Error('client endpoint returned HTML');
    if (path === '/api/connectivity/targets') return { groups: [{ id: 'api', urls: ['https://api.test'] }] };
    throw new Error(`unexpected ${path}`);
  });

  assert.equal(state.client, null);
  assert.equal(state.clientLoadError, 'client endpoint returned HTML');
  assert.deepEqual(state.targets, [{ id: 'api', urls: ['https://api.test'] }]);
  assert.equal(state.connectivityTargetError, '');
});

test('loadModelStatusState sets loading, clears errors and stores status', async () => {
  const renders = [];
  const state = {
    loading: false,
    error: 'old',
    modelTestMessage: 'old',
    modelStatus: null,
    initialized: false
  };

  await loadModelStatusState(state, {
    refresh: true,
    fetchJsonImpl: async (path) => {
      assert.equal(path, '/api/newapi/model-status?refresh=1');
      return { generatedAt: 123, configured: true };
    },
    render: () => renders.push({ loading: state.loading, status: state.modelStatus })
  });

  assert.deepEqual(renders, [
    { loading: true, status: null },
    { loading: false, status: { generatedAt: 123, configured: true } }
  ]);
  assert.equal(state.error, '');
  assert.equal(state.modelTestMessage, '');
  assert.equal(state.initialized, true);
});

test('loadModelStatusState preserves error data with generated timestamp', async () => {
  const state = {
    loading: false,
    error: '',
    modelTestMessage: '',
    modelStatus: null,
    initialized: false
  };
  const error = new Error('server failed');
  error.data = { generatedAt: 456, configured: false };

  await loadModelStatusState(state, {
    renderStart: false,
    fetchJsonImpl: async () => { throw error; },
    render: () => {}
  });

  assert.equal(state.loading, false);
  assert.equal(state.error, 'server failed');
  assert.deepEqual(state.modelStatus, { generatedAt: 456, configured: false });
  assert.equal(state.initialized, false);
});

test('loadModelStatusState keeps the latest overlapping response', async () => {
  const state = {
    loading: false,
    error: '',
    modelTestMessage: '',
    modelStatus: null,
    initialized: false
  };
  const pending = [];
  const fetchJsonImpl = async () => new Promise((resolve) => pending.push(resolve));

  const first = loadModelStatusState(state, { fetchJsonImpl, render: () => {} });
  const second = loadModelStatusState(state, { refresh: true, fetchJsonImpl, render: () => {} });
  assert.equal(pending.length, 2);

  pending[1]({ generatedAt: 2, configured: true });
  await second;
  assert.equal(state.modelStatus.generatedAt, 2);

  pending[0]({ generatedAt: 1, configured: true });
  await first;

  assert.equal(state.loading, false);
  assert.equal(state.error, '');
  assert.equal(state.initialized, true);
  assert.equal(state.modelStatus.generatedAt, 2);
});
