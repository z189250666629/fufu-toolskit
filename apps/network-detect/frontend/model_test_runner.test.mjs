import test from 'node:test';
import assert from 'node:assert/strict';

import {
  applyModelTestResultToState,
  modelCellKey,
  runModelCellTest,
  updateModelCell
} from './model_test_runner.js';

test('modelCellKey separates site and model without ambiguity', () => {
  assert.equal(modelCellKey('site', 'model'), 'site\u0000model');
});

test('updateModelCell replaces the matching per-site cell', () => {
  const modelStatus = {
    models: [
      { model: 'model-a', perSite: { site: { status: 'unknown' } } },
      { model: 'model-b', perSite: { site: { status: 'ok' } } }
    ]
  };
  const nextCell = { status: 'operational' };

  updateModelCell(modelStatus, 'site', 'model-a', nextCell);
  updateModelCell(modelStatus, 'missing', 'missing-model', { status: 'bad' });

  assert.equal(modelStatus.models[0].perSite.site, nextCell);
  assert.deepEqual(modelStatus.models[1].perSite.site, { status: 'ok' });
});

test('applyModelTestResultToState records returned test when API omits cell', () => {
  const modelStatus = {
    models: [{ model: 'model-a', perSite: { site: { status: 'unknown', requestCount: 0 } } }]
  };
  const testRecord = { ok: true, status: 'operational', testedAt: 123, message: 'ok', nextAllowedAt: 456 };

  assert.equal(applyModelTestResultToState(modelStatus, 'site', 'model-a', {
    siteName: 'site',
    model: 'model-a',
    test: testRecord
  }), true);

  assert.equal(modelStatus.models[0].perSite.site.manualTest, testRecord);
  assert.equal(modelStatus.models[0].perSite.site.nextTestAllowedAt, 456);
  assert.equal(modelStatus.models[0].perSite.site.status, 'unknown');
});

test('runModelCellTest toggles testing state and writes success message', async () => {
  const state = {
    testingCells: new Set(),
    modelTestMessage: 'old',
    modelStatus: {
      models: [{ model: 'model-a', perSite: { site: { status: 'unknown' } } }]
    }
  };
  let renders = 0;

  const started = await runModelCellTest({
    state,
    siteName: 'site',
    model: 'model-a',
    group: 'vip',
    postJsonImpl: async (path, body) => {
      assert.equal(path, '/api/newapi/model-status/test');
      assert.deepEqual(body, { siteName: 'site', model: 'model-a', group: 'vip' });
      assert.equal(state.testingCells.has(modelCellKey('site', 'model-a')), true);
      return { cell: { status: 'operational' }, test: { message: 'ok' } };
    },
    render: () => { renders += 1; }
  });

  assert.equal(started, true);
  assert.equal(renders, 2);
  assert.equal(state.testingCells.size, 0);
  assert.deepEqual(state.modelStatus.models[0].perSite.site, { status: 'operational' });
  assert.equal(state.modelTestMessage, 'site / model-a 测试完成：ok');
});

test('runModelCellTest records cooldown and failure message on API error', async () => {
  const state = {
    testingCells: new Set(),
    modelTestMessage: '',
    modelStatus: {
      models: [{ model: 'model-a', perSite: { site: { status: 'unknown' } } }]
    }
  };
  const error = new Error('too many requests');
  error.data = { nextAllowedAt: 123 };

  const started = await runModelCellTest({
    state,
    siteName: 'site',
    model: 'model-a',
    postJsonImpl: async () => { throw error; },
    render: () => {}
  });

  assert.equal(started, true);
  assert.equal(state.testingCells.size, 0);
  assert.equal(state.modelStatus.models[0].perSite.site.nextTestAllowedAt, 123);
  assert.equal(state.modelTestMessage, 'site / model-a 测试失败：too many requests');
});

test('runModelCellTest skips missing or already running cells', async () => {
  const key = modelCellKey('site', 'model-a');
  const state = {
    testingCells: new Set([key]),
    modelTestMessage: '',
    modelStatus: { models: [] }
  };
  let posted = false;

  assert.equal(await runModelCellTest({
    state,
    siteName: '',
    model: 'model-a',
    postJsonImpl: async () => { posted = true; },
    render: () => {}
  }), false);

  assert.equal(await runModelCellTest({
    state,
    siteName: 'site',
    model: 'model-a',
    postJsonImpl: async () => { posted = true; },
    render: () => {}
  }), false);

  assert.equal(posted, false);
});
