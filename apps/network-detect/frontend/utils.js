export function escapeHtml(value) {
  return String(value ?? '')
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#039;');
}

export function idFragment(value) {
  return String(value ?? '')
    .trim()
    .replace(/[^A-Za-z0-9_-]+/g, '-')
    .replace(/^-+|-+$/g, '') || 'item';
}

export function modelScopeTabId(siteName) {
  return `modelScopeTab-${idFragment(siteName)}`;
}

export function modelSiteDisplayName(siteName) {
  if (siteName === '次数fufu') return '次数站';
  if (siteName === 'token-fufu') return 'token站';
  return siteName || '模型站点';
}

export function compactNumber(value) {
  const number = Number(value) || 0;
  if (Math.abs(number) >= 1e9) return `${(number / 1e9).toFixed(2)}B`;
  if (Math.abs(number) >= 1e6) return `${(number / 1e6).toFixed(2)}M`;
  if (Math.abs(number) >= 1e3) return `${(number / 1e3).toFixed(1)}K`;
  return number.toLocaleString();
}

export function formatTime(timestamp) {
  const value = Number(timestamp);
  if (!Number.isFinite(value) || value <= 0) return '-';
  return new Date(value * 1000).toLocaleString('zh-CN', { hour12: false });
}

export function formatServerTime(value) {
  const number = Number(value);
  if (!Number.isFinite(number)) return '-';
  return new Date(number).toLocaleString('zh-CN', { hour12: false });
}

export function average(values) {
  if (!values.length) return null;
  return values.reduce((sum, value) => sum + value, 0) / values.length;
}

export function formatMs(value) {
  if (value == null || !Number.isFinite(value)) return '-';
  return value >= 1000 ? `${(value / 1000).toFixed(2)} s` : `${value.toFixed(0)} ms`;
}

export function formatRate(value) {
  return `${Math.round(Number(value || 0) * 100)}%`;
}

export function modelStatusMeta(status, configured = true) {
  if (!configured) return { label: '未配置', tone: 'muted' };
  if (status === 'operational') return { label: '正常', tone: 'ok' };
  if (status === 'degraded') return { label: '部分异常', tone: 'warn' };
  if (status === 'down') return { label: '不可用', tone: 'bad' };
  return { label: '未知/无调用', tone: 'muted' };
}

export function formatNullableRate(value) {
  if (value == null || !Number.isFinite(Number(value))) return '-';
  return `${Math.round(Number(value) * 100)}%`;
}

export function formatPriceValue(value, currency = '$') {
  const number = Number(value);
  if (!Number.isFinite(number)) return '-';
  const abs = Math.abs(number);
  const digits = abs >= 1 ? 4 : 6;
  const text = number.toLocaleString('en-US', {
    minimumFractionDigits: 0,
    maximumFractionDigits: digits
  });
  return `${currency || '$'}${text}`;
}

export function formatShortTime(timestamp) {
  const value = Number(timestamp);
  if (!Number.isFinite(value) || value <= 0) return '-';
  return new Date(value * 1000).toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false
  });
}

export function formatWindow(seconds) {
  const value = Number(seconds) || 3600;
  if (value >= 3600) return `${Math.round(value / 3600)} 小时`;
  return `${Math.round(value / 60)} 分钟`;
}

export function formatCooldown(timestamp) {
  const value = Number(timestamp);
  if (!Number.isFinite(value) || value <= 0) return '';
  const targetMs = value > 10_000_000_000 ? value : value * 1000;
  const seconds = Math.max(0, Math.ceil((targetMs - Date.now()) / 1000));
  if (seconds <= 0) return '';
  if (seconds >= 3600) return `${Math.ceil(seconds / 3600)} 小时后`;
  if (seconds >= 60) return `${Math.ceil(seconds / 60)} 分钟后`;
  return `${seconds} 秒后`;
}

export function statusFromCounts(successCount, failureCount) {
  if (successCount > 0 && failureCount === 0) return 'operational';
  if (successCount > 0 && failureCount > 0) return 'degraded';
  if (successCount === 0 && failureCount > 0) return 'down';
  return 'unknown';
}

export function statusFromSummary(summary) {
  if (summary.down > 0) return 'down';
  if (summary.degraded > 0) return 'degraded';
  if (summary.unknown > 0 && summary.operational === 0) return 'unknown';
  if (summary.operational > 0) return 'operational';
  return 'unknown';
}

export function successRate(successCount, failureCount) {
  const total = Number(successCount || 0) + Number(failureCount || 0);
  return total > 0 ? Number(successCount || 0) / total : null;
}
