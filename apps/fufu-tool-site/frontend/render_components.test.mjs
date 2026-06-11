import test from 'node:test';
import assert from 'node:assert/strict';

import {
  renderAlert,
  renderChip,
  renderMetric,
  renderPriceCell,
  renderStateCard,
  renderStatusPill
} from './render_components.js';

test('renderMetric escapes values and includes optional sub text', () => {
  const html = renderMetric('<label>', '<value>', '<sub>', 'danger');
  assert.match(html, /metric-danger/);
  assert.match(html, /&lt;label&gt;/);
  assert.match(html, /&lt;value&gt;/);
  assert.match(html, /&lt;sub&gt;/);
});

test('renderChip and renderAlert escape user-facing content', () => {
  assert.match(renderChip('<ok>', 'ok', 'status'), /&lt;ok&gt;/);
  const alert = renderAlert('<bad>', 'danger');
  assert.match(alert, /role="alert"/);
  assert.match(alert, /&lt;bad&gt;/);
});

test('renderStateCard and status pill produce expected labels', () => {
  assert.match(renderStateCard('标题', '描述'), /标题/);
  assert.match(renderStatusPill('operational'), /正常/);
  assert.match(renderStatusPill('down'), /不可用/);
});

test('renderPriceCell handles empty and request pricing', () => {
  assert.match(renderPriceCell(null), /price-empty/);
  assert.match(renderPriceCell({ available: true, type: 'request', request: 0.2, currency: '¥' }), /每次请求/);
});

test('renderPriceCell displays backend pricing shape without available flag', () => {
  const html = renderPriceCell({ input: 0.1, output: 0.2, currency: 'CNY' });

  assert.doesNotMatch(html, /price-empty/);
  assert.match(html, /入/);
  assert.match(html, /出/);
  assert.match(html, /CNY/);
});
