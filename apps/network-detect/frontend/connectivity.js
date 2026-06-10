export const SAMPLE_COUNT = 4;
export const TIMEOUT_MS = 8000;

export const DEFAULT_TARGET_GROUPS = [
  {
    id: 'api',
    name: 'API 次数站',
    urls: [
      'https://api.fufuapi.top',
      'https://api.fufuapi.online',
      'https://api.fufuflower.top'
    ]
  },
  {
    id: 'token',
    name: 'Token 站',
    urls: [
      'https://token.fufuapi.top',
      'https://token.fufuapi.online',
      'https://token.fufuflower.top'
    ]
  }
];

export function normalizeTargetGroups(targets, fallback = DEFAULT_TARGET_GROUPS) {
  const normalized = cleanTargetGroups(targets);
  if (normalized.length) return normalized;
  return cleanTargetGroups(fallback);
}

function cleanTargetGroups(groups = []) {
  return groups
    .map((group) => ({
      id: String(group.id || group.name || ''),
      name: String(group.name || group.id || 'URL 组'),
      urls: Array.isArray(group.urls)
        ? group.urls.map(safeTargetOrigin).filter(Boolean)
        : []
    }))
    .filter((group) => group.urls.length);
}

function safeTargetOrigin(value) {
  const raw = String(value ?? '').trim();
  if (!raw) return '';
  let parsed;
  try {
    parsed = new URL(raw);
  } catch {
    return '';
  }
  if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') return '';
  const hostname = parsed.hostname.replace(/^\[|\]$/g, '').replace(/\.$/, '').toLowerCase();
  if (!hostname || hostname === 'localhost' || hostname.endsWith('.localhost')) return '';
  if (isUnsafeIPAddress(hostname)) return '';
  return parsed.origin;
}

function isUnsafeIPAddress(hostname) {
  const ipv4 = hostname.match(/^(\d{1,3})(?:\.(\d{1,3})){3}$/);
  if (ipv4) {
    const parts = hostname.split('.').map(Number);
    if (parts.some((part) => part < 0 || part > 255)) return true;
    const [a, b] = parts;
    return (
      a === 0 ||
      a === 10 ||
      a === 127 ||
      (a === 169 && b === 254) ||
      (a === 172 && b >= 16 && b <= 31) ||
      (a === 192 && b === 168)
    );
  }
  if (hostname.includes(':')) {
    const compact = hostname.toLowerCase();
    return compact === '::1' || compact === '::' || compact.startsWith('fc') || compact.startsWith('fd') || compact.startsWith('fe80:');
  }
  return false;
}

export function addCacheBust(url, baseHref = globalThis.location?.href || 'http://localhost/') {
  const next = new URL(url, baseHref);
  next.searchParams.set('_fufu_connect_test', `${Date.now()}_${Math.random().toString(16).slice(2)}`);
  return next.toString();
}

export function fetchWithTimeout(url, options, timeoutMs, deps = {}) {
  const {
    fetchImpl = globalThis.fetch,
    AbortControllerImpl = globalThis.AbortController,
    setTimeoutImpl = globalThis.setTimeout,
    clearTimeoutImpl = globalThis.clearTimeout
  } = deps;
  const controller = new AbortControllerImpl();
  const timer = setTimeoutImpl(() => controller.abort(), timeoutMs);
  return fetchImpl(url, { ...options, signal: controller.signal })
    .finally(() => clearTimeoutImpl(timer));
}

export function fetchErrorText(error) {
  if (error?.name === 'AbortError') return '请求超时';
  return '请求失败或被浏览器拦截';
}
