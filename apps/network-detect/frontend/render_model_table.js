import {
  escapeHtml,
  formatCooldown,
  formatNullableRate,
  formatShortTime
} from './utils.js';
import {
  renderPriceCell,
  renderStatusPill
} from './render_components.js';
import { modelCellKey } from './model_test_runner.js';

export function renderModelTestAction(cell, stateLike = {}) {
  const group = cell.groups?.[0] || '';
  const key = modelCellKey(cell.siteName, cell.model, group);
  const testing = stateLike.testingCells?.has(key);
  const cooldown = formatCooldown(cell.nextTestAllowedAt);

  return `
    <div class="model-action">
      <button
        class="button small model-test-button"
        type="button"
        data-model-test="1"
        data-site="${escapeHtml(cell.siteName)}"
        data-model="${escapeHtml(cell.model)}"
        data-group="${escapeHtml(group)}"
        ${testing || Boolean(cooldown) ? 'disabled' : ''}
      >${testing ? '测试中' : (cooldown || '测试')}</button>
    </div>
  `;
}

export function manualTestRowClass(cell) {
  if (cell.manualTestTone === 'ok') return 'is-manual-ok';
  if (cell.manualTestTone === 'bad') return 'is-manual-bad';
  return '';
}

export function renderModelTableRows(models, stateLike = {}) {
  return models.map(({ row, cell }) => `
    <tr class="${manualTestRowClass(cell)}">
      <td class="sticky-col model-name" data-label="模型">${escapeHtml(row.model)}</td>
      <td data-label="价格">${renderPriceCell(cell.pricing)}</td>
      <td data-label="状态">${renderStatusPill(cell.status, true)}</td>
      <td data-label="成功率">${escapeHtml(formatNullableRate(cell.successRate))}</td>
      <td data-label="最近">${escapeHtml(formatShortTime(cell.lastSuccessAt || cell.lastFailureAt || cell.lastSeenAt))}</td>
      <td data-label="操作">${renderModelTestAction(cell, stateLike)}</td>
    </tr>
  `).join('');
}
