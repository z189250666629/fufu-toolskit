import test from 'node:test';
import assert from 'node:assert/strict';

import {
  loadModelStatusState,
  loadStaticContextState
} from './app_data.js';

test('loadStaticContextState stores client and target groups with fallbacks', async () => {
  const state = { client: null, targets: [] };
  const calls = [];

  await loadStaticContextState(state, async (path) => {
    calls.push(path);
    if (path === '/api/client') return { ip: '127.0.0.1' };
    if (path === '/api/connectivity/targets') return { groups: [{ id: 'api', urls: ['https://api.test'] }] };
    throw new Error(`unexpected ${path}`);
  });

  assert.deepEqual(calls.sort(), ['/api/client', '/api/connectivity/targets'].sort());
  assert.deepEqual(state.client, { ip: '127.0.0.1' });
  assert.deepEqual(state.targets, [{ id: 'api', urls: ['https://api.test'] }]);
});

test('loadStaticContextState tolerates failed optional requests', async () => {
  const state = { client: { old: true }, targets: [{ old: true }] };

  await loadStaticContextState(state, async () => {
    throw new Error('offline');
  });

  assert.equal(state.client, null);
  assert.deepEqual(state.targets, []);
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
