import test from 'node:test';
import assert from 'node:assert/strict';

import {
  activeModelScope,
  applyManualTestDisplay,
  scopedModelRows,
  scopedSummary
} from './model_selectors.js';

const modelStatus = {
  sites: [
    { site: { name: '次数fufu' }, groups: ['mix'] },
    { site: { name: 'token-fufu' }, groups: ['vip', 'default'] }
  ],
  models: [
    {
      model: 'zeta',
      perSite: {
        'token-fufu': {
          configured: true,
          enabledChannelCount: 1,
          groupStats: { vip: { configured: true, status: 'operational', requestCount: 2, successCount: 2, failureCount: 0, enabledChannelCount: 1 } }
        }
      }
    },
    {
      model: 'alpha',
      perSite: {
        'token-fufu': {
          configured: true,
          enabledChannelCount: 1,
          groupStats: { vip: { configured: true, status: 'down', requestCount: 1, successCount: 0, failureCount: 1, enabledChannelCount: 1 } }
        }
      }
    }
  ]
};

test('activeModelScope chooses selected site and token group', () => {
  assert.deepEqual(
    activeModelScope(modelStatus, { selectedModelSite: 'token-fufu', selectedTokenGroup: 'vip' }),
    { site: modelStatus.sites[1], siteName: 'token-fufu', group: 'vip', groups: ['vip', 'default'] }
  );
  assert.equal(activeModelScope(modelStatus, { selectedModelSite: 'missing' }).siteName, '次数fufu');
});

test('manual test display can override unknown cells', () => {
  const got = applyManualTestDisplay({
    status: 'unknown',
    requestCount: 0,
    successCount: 0,
    failureCount: 0,
    manualTest: { ok: true, testedAt: 100 }
  });

  assert.equal(got.status, 'operational');
  assert.equal(got.manualTestTone, 'ok');
  assert.equal(got.successCount, 1);
});

test('scopedModelRows filters, sorts and summarizes rows', () => {
  const rows = scopedModelRows(
    modelStatus,
    { siteName: 'token-fufu', group: 'vip' },
    { modelFilter: '' }
  );

  assert.deepEqual(rows.map((item) => item.row.model), ['alpha', 'zeta']);
  const summary = scopedSummary(rows);
  assert.equal(summary.modelCount, 2);
  assert.equal(summary.failureCount, 1);
  assert.equal(summary.status, 'down');
});
