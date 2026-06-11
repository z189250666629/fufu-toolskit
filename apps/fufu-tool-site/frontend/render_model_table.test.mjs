import test from 'node:test';
import assert from 'node:assert/strict';

import { renderModelTableRows } from './render_model_table.js';
import { modelCellKey } from './model_test_runner.js';

test('renderModelTableRows renders escaped rows with test actions', () => {
  const html = renderModelTableRows([
    {
      row: { model: '<gpt-4>' },
      cell: {
        siteName: 'site',
        model: '<gpt-4>',
        groups: ['vip'],
        status: 'operational',
        successRate: 0.75,
        pricing: null,
        manualTestTone: 'ok',
        lastSuccessAt: 1760000000
      }
    }
  ], { testingCells: new Set([modelCellKey('site', '<gpt-4>', 'vip')]) });

  assert.match(html, /class="is-manual-ok"/);
  assert.match(html, /&lt;gpt-4&gt;/);
  assert.doesNotMatch(html, /<gpt-4>/);
  assert.match(html, /75%/);
  assert.match(html, /data-group="vip"/);
  assert.match(html, /测试中/);
});
