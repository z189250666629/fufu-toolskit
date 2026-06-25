const DEFAULT_UNIT_NAMES = { 3: '天卡', 8: '周不刷新卡', 9: '月不刷新卡' };
const DEFAULT_QUOTA_UNIT = 500000;

function escapeHtml(value) {
  return String(value ?? '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}

function escapeJsStringForAttr(value) {
  return JSON.stringify(String(value ?? ''))
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}

function formatQuota(value, quotaUnit = DEFAULT_QUOTA_UNIT) {
  const unit = Number(quotaUnit) || DEFAULT_QUOTA_UNIT;
  return '$' + ((Number(value) || 0) / unit).toFixed(2);
}

function tokenIsValid(token) {
  return token?.status === 1 && (Number(token?.remain_quota) || 0) > 0;
}

function tokenIsInvalid(token) {
  return !tokenIsValid(token);
}

export function getValidTokens(tokens = []) {
  return (tokens || []).filter(tokenIsValid);
}

export function getInvalidTokens(tokens = []) {
  return (tokens || []).filter(tokenIsInvalid);
}

function renderValidToken(token, options) {
  const unitNames = options.unitNames || DEFAULT_UNIT_NAMES;
  return `
    <div class="token-item">
      <div>
        <div class="token-name">${escapeHtml(token.name || '(unnamed)')}</div>
        <div class="token-meta">${unitNames[token.interval_unit] || '未知类型'} · ${escapeHtml(token.group || '')}</div>
      </div>
      <div class="token-quota">${formatQuota(token.remain_quota, options.quotaUnit)}</div>
    </div>
  `;
}

function renderDeleteAction(token, options) {
  if (token.status === 1 || options.userRole === 'guest') return '';
  const tokenID = Number(token.id);
  if (!Number.isFinite(tokenID)) return '';
  return `<button class="btn btn-danger btn-small" onclick="deleteDisabledToken(${tokenID}, ${escapeJsStringForAttr(token.key)})">删除</button>`;
}

function renderInvalidToken(token, options) {
  const unitNames = options.unitNames || DEFAULT_UNIT_NAMES;
  const reason = token.status !== 1 ? '已禁用' : '无额度';
  return `
    <div class="token-item disabled">
      <div>
        <div class="token-name">${escapeHtml(token.name || '(unnamed)')}</div>
        <div class="token-meta">${unitNames[token.interval_unit] || '未知类型'} · ${escapeHtml(token.group || '')} · ${reason}</div>
      </div>
      <div class="token-actions">
        <span class="token-quota" style="color:var(--warn)">${formatQuota(token.remain_quota, options.quotaUnit)}</span>
        ${renderDeleteAction(token, options)}
      </div>
    </div>
  `;
}

export function renderTokenSections(tokens = [], options = {}) {
  const renderOptions = {
    unitNames: options.unitNames || DEFAULT_UNIT_NAMES,
    quotaUnit: options.quotaUnit || DEFAULT_QUOTA_UNIT,
    userRole: options.userRole || ''
  };
  const validTokens = getValidTokens(tokens);
  const invalidTokens = getInvalidTokens(tokens);
  let html = '';

  if (validTokens.length > 0) {
    html += `<div class="token-section">
      <div class="token-section-title valid">✓ 可合成 (${validTokens.length})</div>`;
    html += validTokens.map(token => renderValidToken(token, renderOptions)).join('');
    html += '</div>';
  }

  if (invalidTokens.length > 0) {
    html += `<div class="token-section">
      <div class="token-section-title invalid">⚠ 不可合成 (${invalidTokens.length})</div>`;
    html += invalidTokens.map(token => renderInvalidToken(token, renderOptions)).join('');
    html += '</div>';
  }

  return html;
}
