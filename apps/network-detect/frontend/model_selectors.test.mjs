import test from 'node:test';
import assert from 'node:assert/strict';

import {
  activeModelScope,
  applyManualTestDisplay,
  groupCellFor,
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

test('activeModelScope falls back when mix group is unavailable', () => {
  const status = {
    sites: [
      { site: { name: '次数fufu' }, groups: ['vip', 'default'] }
    ]
  };

  assert.equal(activeModelScope(status, {}).group, 'vip');
  assert.equal(activeModelScope(status, { selectedTokenGroup: 'default' }).group, 'default');
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

test('manual test display does not double count materialized successful manual tests', () => {
  const got = applyManualTestDisplay({
    status: 'operational',
    requestCount: 1,
    successCount: 1,
    failureCount: 0,
    lastSuccessAt: 100,
    lastSeenAt: 100,
    manualTest: { ok: true, status: 'operational', testedAt: 100 }
  });

  assert.equal(got.manualTestTone, 'ok');
  assert.equal(got.successCount, 1);
  assert.equal(got.failureCount, 0);
  assert.equal(got.requestCount, 1);
});

test('groupCellFor does not double count materialized group manual tests', () => {
  const got = groupCellFor({
    model: 'model-a',
    perSite: {
      site: {
        configured: true,
        groupStats: {
          vip: {
            configured: true,
            status: 'operational',
            requestCount: 1,
            successCount: 1,
            failureCount: 0,
            lastSuccessAt: 100,
            enabledChannelCount: 1,
            manualTest: { ok: true, status: 'operational', testedAt: 100 }
          }
        }
      }
    }
  }, 'site', 'vip');

  assert.equal(got.manualTestTone, 'ok');
  assert.equal(got.successCount, 1);
  assert.equal(got.requestCount, 1);
});

test('groupCellFor ignores manual test from another group', () => {
  const got = groupCellFor({
    model: 'model-a',
    perSite: {
      site: {
        configured: true,
        manualTest: { ok: true, status: 'operational', testedAt: 100, group: 'default' },
        groupStats: {
          vip: {
            configured: true,
            status: 'unknown',
            requestCount: 0,
            successCount: 0,
            failureCount: 0,
            enabledChannelCount: 1
          }
        }
      }
    }
  }, 'site', 'vip');

  assert.equal(got.status, 'unknown');
  assert.equal(got.manualTestTone, undefined);
  assert.equal(got.requestCount, 0);
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

test('scopedSummary normalizes count fields before accumulating', () => {
  const summary = scopedSummary([
    { cell: { status: 'operational', requestCount: '3', successCount: '3', failureCount: '0' } },
    { cell: { status: 'degraded', requestCount: '2', successCount: '1', failureCount: '1' } }
  ]);

  assert.equal(summary.requestCount, 5);
  assert.equal(summary.successCount, 4);
  assert.equal(summary.failureCount, 1);
  assert.equal(summary.successRate, 0.8);
  assert.equal(summary.status, 'degraded');
});
