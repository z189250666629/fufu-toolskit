import test from 'node:test';
import assert from 'node:assert/strict';

import {
  average,
  compactNumber,
  escapeHtml,
  formatMs,
  formatNullableRate,
  formatPriceValue,
  formatRate,
  formatWindow,
  idFragment,
  modelSiteDisplayName,
  statusFromCounts,
  successRate
} from './utils.js';

test('HTML and id formatting helpers', () => {
  assert.equal(escapeHtml(`<b data-x="1">Tom & Jerry's</b>`), '&lt;b data-x=&quot;1&quot;&gt;Tom &amp; Jerry&#039;s&lt;/b&gt;');
  assert.equal(idFragment('  token fufu / vip  '), 'token-fufu-vip');
  assert.equal(idFragment('   '), 'item');
  assert.equal(modelSiteDisplayName('次数fufu'), '次数站');
  assert.equal(modelSiteDisplayName('token-fufu'), 'token站');
});

test('numeric display helpers', () => {
  assert.equal(compactNumber(1500), '1.5K');
  assert.equal(formatMs(1200), '1.20 s');
  assert.equal(formatMs(42), '42 ms');
  assert.equal(formatRate(0.876), '88%');
  assert.equal(formatNullableRate(null), '-');
  assert.equal(formatNullableRate(0.333), '33%');
  assert.equal(formatPriceValue(0.1234567, '¥'), '¥0.123457');
  assert.equal(formatWindow(7200), '2 小时');
  assert.equal(formatWindow(600), '10 分钟');
});

test('summary math helpers', () => {
  assert.equal(average([1, 2, 3]), 2);
  assert.equal(average([]), null);
  assert.equal(successRate(3, 1), 0.75);
  assert.equal(successRate(0, 0), null);
  assert.equal(statusFromCounts(1, 0), 'operational');
  assert.equal(statusFromCounts(1, 1), 'degraded');
  assert.equal(statusFromCounts(0, 1), 'down');
  assert.equal(statusFromCounts(0, 0), 'unknown');
});
