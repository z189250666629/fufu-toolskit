import test from 'node:test';
import assert from 'node:assert/strict';

import {
  manualTestRowClass,
  renderModelAvailability,
  renderModelTestAction,
  renderTokenGroupSelect
} from './render_models.js';
import { modelCellKey } from './model_test_runner.js';

test('renderModelTestAction reflects testing state and cooldown', () => {
  const key = modelCellKey('site', 'model-a', 'vip');
  const html = renderModelTestAction(
    { siteName: 'site', model: 'model-a', groups: ['vip'] },
    { testingCells: new Set([key]) }
  );

  assert.match(html, /disabled/);
  assert.match(html, /测试中/);
  assert.match(html, /data-group="vip"/);
});

test('manualTestRowClass maps manual result tone', () => {
  assert.equal(manualTestRowClass({ manualTestTone: 'ok' }), 'is-manual-ok');
  assert.equal(manualTestRowClass({ manualTestTone: 'bad' }), 'is-manual-bad');
  assert.equal(manualTestRowClass({}), '');
});

test('renderTokenGroupSelect opens selected group list', () => {
  const html = renderTokenGroupSelect(['vip', 'default'], 'vip', { groupSelectOpen: true });
  assert.match(html, /aria-expanded="true"/);
  assert.match(html, /data-token-group-option="vip"/);
  assert.match(html, /aria-selected="true"/);
});

test('renderModelAvailability handles empty configured state', () => {
  const html = renderModelAvailability({
    state: {
      loading: false,
      error: '',
      modelStatus: { configured: false, configError: 'missing config', sites: [] },
      testingCells: new Set(),
      groupSelectOpen: false,
      modelFilter: ''
    }
  });

  assert.match(html, /暂无模型状态数据/);
  assert.match(html, /missing config/);
});
