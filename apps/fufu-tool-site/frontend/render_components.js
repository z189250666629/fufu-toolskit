import {
  escapeHtml,
  formatPriceValue,
  modelStatusMeta
} from './utils.js';

export function renderMetric(label, value, sub = '', tone = '') {
  return `
    <div class="metric ${tone ? `metric-${tone}` : ''}" data-slot="card">
      <div class="metric-head" data-slot="card-header">
        <span data-slot="card-title">${escapeHtml(label)}</span>
        ${sub ? `<small data-slot="card-description">${escapeHtml(sub)}</small>` : '<small class="sr-only" data-slot="card-description">当前指标</small>'}
      </div>
      <div class="metric-content" data-slot="card-content">
        <b>${escapeHtml(value)}</b>
      </div>
    </div>
  `;
}

export function renderStateCard(title, description, content = '') {
  return `
    <div class="model-state-card" data-slot="card">
      <div class="model-state-copy" data-slot="card-header">
        <h3 data-slot="card-title">${escapeHtml(title)}</h3>
        <p data-slot="card-description">${escapeHtml(description)}</p>
      </div>
      <span data-slot="card-content">${escapeHtml(content || title)}</span>
    </div>
  `;
}

export function renderChip(label, tone = 'muted', className = '') {
  return `
    <span class="chip ${className} ${tone}" data-slot="chip">
      <span class="chip-label" data-slot="chip-label">${escapeHtml(label)}</span>
    </span>
  `;
}

export function renderAlert(message, tone = 'danger', className = '') {
  const titles = {
    danger: '错误',
    info: '提示',
    success: '完成',
    warning: '注意'
  };
  const title = titles[tone] || '提示';
  const role = tone === 'danger' ? 'alert' : 'status';
  return `
    <div class="notice alert ${className}" role="${role}" data-slot="alert" data-status="${escapeHtml(tone)}">
      <span class="alert-indicator" data-slot="alert-indicator" aria-hidden="true"></span>
      <span class="alert-content" data-slot="alert-content">
        <span class="alert-title sr-only" data-slot="alert-title">${title}</span>
        <span class="alert-description" data-slot="alert-description">${escapeHtml(message)}</span>
      </span>
    </div>
  `;
}

export function renderStatusPill(status, configured = true) {
  const meta = modelStatusMeta(status, configured);
  return renderChip(meta.label, meta.tone, 'status');
}

export function renderPriceCell(pricing) {
  if (!pricing || pricing.available === false) return '<span class="price-empty">-</span>';
  const hasStandardPricing = Number.isFinite(Number(pricing.input)) || Number.isFinite(Number(pricing.output));
  const hasRequestPricing = Number.isFinite(Number(pricing.request));
  if (pricing.available !== true && !pricing.type && !hasStandardPricing && !hasRequestPricing) {
    return '<span class="price-empty">-</span>';
  }
  if (pricing.type === 'dynamic') {
    return `
      <div class="price-cell">
        <b>阶梯计费</b>
        <small>按规则计算</small>
      </div>
    `;
  }
  if (pricing.type === 'request' || (hasRequestPricing && !hasStandardPricing)) {
    return `
      <div class="price-cell">
        <b>${escapeHtml(formatPriceValue(pricing.request, pricing.currency))}</b>
        <small>每次请求</small>
      </div>
    `;
  }
  return `
    <div class="price-cell">
      <b>入 ${escapeHtml(formatPriceValue(pricing.input, pricing.currency))}</b>
      <small>出 ${escapeHtml(formatPriceValue(pricing.output, pricing.currency))} / 1M</small>
    </div>
  `;
}
