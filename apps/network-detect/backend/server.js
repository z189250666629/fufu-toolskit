import { createServer } from 'node:http';
import { readFile } from 'node:fs/promises';
import { existsSync } from 'node:fs';
import { homedir } from 'node:os';
import { basename, dirname, extname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const ROOT_DIR = resolve(__dirname, '..');
const FRONTEND_DIR = resolve(ROOT_DIR, 'frontend');
const PORT = Number(process.env.PORT || 8080);
const DEFAULT_QUOTA_UNIT = 500000;
const LOG_TYPE_CONSUME = 2;
const LOG_TYPE_ERROR = 5;
const CHANNEL_STATUS_ENABLED = 1;
const MODEL_STATUS_WINDOW_SECONDS = 10 * 60;
const MODEL_STATUS_CACHE_MS = MODEL_STATUS_WINDOW_SECONDS * 1000;
const MODEL_TEST_COOLDOWN_MS = 60 * 60 * 1000;
const MODEL_LOG_PAGE_SIZE = 100;
const MODEL_LOG_MAX_ROWS_PER_TYPE = 1000;
const QUOTA_TYPE_REQUEST = 1;
const BUILTIN_MANAGER_CONFIG_PATH = join(homedir(), 'Downloads', 'newapi-manager-config-2026-05-06.json');
const BUILTIN_MANAGED_SITE_NAMES = new Set(['次数fufu', 'token-fufu']);

const modelStatusCache = {
  value: null,
  expiresAt: 0,
  pending: null
};
const modelTestCooldowns = new Map();
const modelTestResults = new Map();

const TARGET_GROUPS = [
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
const DEPLOYMENT_SITE_ENV_PREFIXES = [
  {
    prefix: 'NEWAPI_API_SITE',
    defaults: {
      name: '次数fufu',
      url: 'https://api.fufuflower.top',
      rechargeRatio: 0.1,
      note: 'API 次数站管理配置'
    }
  },
  {
    prefix: 'NEWAPI_TOKEN_SITE',
    defaults: {
      name: 'token-fufu',
      url: 'https://token.fufuflower.top',
      rechargeRatio: 1,
      note: 'Token 站管理配置'
    }
  }
];
const MANAGED_SITE_ENV_LIMIT = 10;

const MIME_TYPES = {
  '.html': 'text/html; charset=utf-8',
  '.css': 'text/css; charset=utf-8',
  '.js': 'text/javascript; charset=utf-8',
  '.json': 'application/json; charset=utf-8',
  '.svg': 'image/svg+xml',
  '.png': 'image/png',
  '.jpg': 'image/jpeg',
  '.jpeg': 'image/jpeg',
  '.ico': 'image/x-icon'
};

function envValue(name) {
  const value = process.env[name];
  if (typeof value !== 'string') return '';
  return value.trim();
}

function splitEnvList(value) {
  return String(value || '')
    .split(/[\n,;]+/)
    .map((item) => item.trim())
    .filter(Boolean);
}

function safeJsonParse(raw, label) {
  try {
    return { data: JSON.parse(raw), error: null };
  } catch (error) {
    return { data: null, error: `${label} 不是有效 JSON: ${error.message}` };
  }
}

function normalizeBaseUrl(value) {
  const input = String(value || '').trim().replace(/\/+$/, '');
  if (!input) return '';
  try {
    const url = new URL(input);
    if (!['http:', 'https:'].includes(url.protocol)) return '';
    return url.toString().replace(/\/+$/, '');
  } catch {
    return '';
  }
}

function boolValue(value) {
  if (value === true || value === 1) return true;
  if (typeof value !== 'string') return false;
  return ['1', 'true', 'yes', 'on'].includes(value.trim().toLowerCase());
}

function positiveNumber(value, fallback) {
  const number = Number(value);
  return Number.isFinite(number) && number > 0 ? number : fallback;
}

function finiteNumber(value, fallback = null) {
  const number = Number(value);
  return Number.isFinite(number) ? number : fallback;
}

function coerceArrayConfig(value) {
  if (!value || typeof value !== 'object') return [];
  if (Array.isArray(value)) return value;
  if (Array.isArray(value.managedApiSites)) return value.managedApiSites;
  if (Array.isArray(value.managed_api_sites)) return value.managed_api_sites;
  if (Array.isArray(value.managedSites)) return value.managedSites;
  if (Array.isArray(value.managed_sites)) return value.managed_sites;
  if (Array.isArray(value.sources)) return value.sources;
  if (Array.isArray(value.admin_instances)) return value.admin_instances;
  if (Array.isArray(value.instances)) return value.instances;
  return [];
}

function itemToken(item) {
  const tokenEnv = String(item.tokenEnv || item.token_env || item.accessTokenEnv || item.access_token_env || '').trim();
  const envToken = tokenEnv ? envValue(tokenEnv) : '';
  return String(item.token || item.accessToken || item.access_token || envToken || '').trim();
}

function deploymentSiteFromEnv(prefix, defaults = {}) {
  const url = envValue(`${prefix}_URL`) || defaults.url || '';
  const token = envValue(`${prefix}_TOKEN`) || envValue(`${prefix}_ACCESS_TOKEN`);
  if (!url || !token) return null;

  return {
    name: envValue(`${prefix}_NAME`) || defaults.name || prefix,
    url,
    token,
    accessToken: envValue(`${prefix}_ACCESS_TOKEN`),
    userId: envValue(`${prefix}_USER_ID`) || defaults.userId || '1',
    kind: envValue(`${prefix}_KIND`) || defaults.kind || 'api',
    channelListEndpoint: envValue(`${prefix}_CHANNEL_LIST_ENDPOINT`) || defaults.channelListEndpoint || '/api/channel/search?keyword=&p=1&page_size=500',
    quotaUnit: envValue(`${prefix}_QUOTA_UNIT`) || defaults.quotaUnit || DEFAULT_QUOTA_UNIT,
    currency: envValue(`${prefix}_CURRENCY`) || defaults.currency || '$',
    rechargeRatio: envValue(`${prefix}_RECHARGE_RATIO`) || defaults.rechargeRatio || 1,
    skipUserHeader: envValue(`${prefix}_SKIP_USER_HEADER`) || defaults.skipUserHeader || false,
    note: envValue(`${prefix}_NOTE`) || defaults.note || ''
  };
}

function managedSitesFromDeploymentEnv() {
  const sites = DEPLOYMENT_SITE_ENV_PREFIXES
    .map(({ prefix, defaults }) => deploymentSiteFromEnv(prefix, defaults))
    .filter(Boolean);

  for (let index = 1; index <= MANAGED_SITE_ENV_LIMIT; index += 1) {
    const site = deploymentSiteFromEnv(`NEWAPI_MANAGED_SITE_${index}`, {
      name: `managed-site-${index}`,
      rechargeRatio: 1
    });
    if (site) sites.push(site);
  }

  return sites;
}

function connectivityGroupsFromEnv() {
  const inline = envValue('CONNECTIVITY_TARGETS') || envValue('FUFU_CONNECTIVITY_TARGETS');
  if (inline) {
    const parsed = safeJsonParse(inline, 'CONNECTIVITY_TARGETS');
    if (!parsed.error) {
      const groups = coerceArrayConfig(parsed.data)
        .map((group) => ({
          id: String(group.id || group.name || '').trim(),
          name: String(group.name || group.id || 'URL 组').trim(),
          urls: Array.isArray(group.urls) ? group.urls.map(normalizeBaseUrl).filter(Boolean) : []
        }))
        .filter((group) => group.id && group.urls.length);
      if (groups.length) return groups;
    }
  }

  const apiUrls = splitEnvList(
    envValue('CONNECTIVITY_API_URLS')
      || envValue('FUFU_API_URLS')
      || envValue('NEWAPI_API_SITE_URL')
  )
    .map(normalizeBaseUrl)
    .filter(Boolean);
  const tokenUrls = splitEnvList(
    envValue('CONNECTIVITY_TOKEN_URLS')
      || envValue('FUFU_TOKEN_URLS')
      || envValue('NEWAPI_TOKEN_SITE_URL')
  )
    .map(normalizeBaseUrl)
    .filter(Boolean);
  const groups = [];

  if (apiUrls.length) {
    groups.push({
      id: 'api',
      name: envValue('CONNECTIVITY_API_NAME') || 'API 次数站',
      urls: apiUrls
    });
  }

  if (tokenUrls.length) {
    groups.push({
      id: 'token',
      name: envValue('CONNECTIVITY_TOKEN_NAME') || 'Token 站',
      urls: tokenUrls
    });
  }

  return groups.length ? groups : TARGET_GROUPS;
}

async function readManagedApiSitesConfig() {
  const inline = envValue('NEWAPI_MANAGED_API_SITES') || envValue('NEWAPI_LOG_SOURCES');
  if (inline) {
    const parsed = safeJsonParse(inline, 'NEWAPI_MANAGED_API_SITES');
    if (parsed.error) return { sites: [], error: parsed.error };
    return normalizeManagedSites(parsed.data);
  }

  const deploymentSites = managedSitesFromDeploymentEnv();
  if (deploymentSites.length) {
    return normalizeManagedSites(deploymentSites);
  }

  const configuredPath = envValue('NEWAPI_MANAGED_API_CONFIG') || envValue('NEWAPI_LOG_CONFIG');
  const configCandidates = configuredPath
    ? [{ path: configuredPath, names: null }]
    : [
        { path: resolve(ROOT_DIR, 'newapi-managed-api-sites.json'), names: null },
        { path: BUILTIN_MANAGER_CONFIG_PATH, names: BUILTIN_MANAGED_SITE_NAMES }
      ];

  const existingConfigs = configCandidates.filter((candidate) => existsSync(candidate.path));
  if (!existingConfigs.length) return { sites: [], error: '' };

  let lastError = '';
  for (const config of existingConfigs) {
    try {
      const raw = await readFile(config.path, 'utf8');
      const parsed = safeJsonParse(raw, basename(config.path));
      if (parsed.error) {
        lastError = parsed.error;
        continue;
      }

      const normalized = normalizeManagedSites(parsed.data, { names: config.names });
      if (normalized.sites.length || configuredPath) return normalized;
      lastError = normalized.error;
    } catch (error) {
      lastError = `读取 NewAPI 配置失败: ${error.message}`;
    }
  }

  return { sites: [], error: lastError };
}

function normalizeManagedSites(value, options = {}) {
  const items = coerceArrayConfig(value);
  const siteMap = new Map();

  for (const item of items) {
    if (!item || typeof item !== 'object') continue;
    const name = String(item.name || '').trim();
    if (options.names && !options.names.has(name)) continue;
    const url = normalizeBaseUrl(item.url);
    const token = itemToken(item);
    if (!name || !url || !token) continue;

    const kind = String(item.kind || item.role || item.siteType || item.site_type || 'api').trim().toLowerCase();
    if (!['api', 'managed-api', 'managed_api', 'admin'].includes(kind)) continue;

    siteMap.set(name, {
      name,
      url,
      token,
      userId: String(item.userId || item.user_id || '1').trim() || '1',
      skipUserHeader: boolValue(item.skipUserHeader ?? item.skip_user_header),
      quotaUnit: positiveNumber(item.quotaUnit ?? item.quota_unit, DEFAULT_QUOTA_UNIT),
      currency: String(item.currency || '$').trim() || '$',
      rechargeRatio: positiveNumber(item.rechargeRatio ?? item.recharge_ratio ?? item.exchangeRate ?? item.exchange_rate, 1),
      channelListEndpoint: String(item.channelListEndpoint || item.channel_list_endpoint || '').trim(),
      note: String(item.note || '').trim()
    });
  }

  return {
    sites: [...siteMap.values()],
    error: siteMap.size === 0 && items.length > 0 ? '配置文件中没有可用的 API 站点' : ''
  };
}

function publicSite(site) {
  return {
    name: site.name,
    url: site.url,
    userId: site.userId,
    quotaUnit: site.quotaUnit,
    currency: site.currency,
    rechargeRatio: site.rechargeRatio,
    skipUserHeader: site.skipUserHeader,
    note: site.note
  };
}

function sendJson(res, status, payload) {
  const body = JSON.stringify(payload);
  res.writeHead(status, {
    'Content-Type': 'application/json; charset=utf-8',
    'Cache-Control': 'no-store',
    'Content-Length': Buffer.byteLength(body)
  });
  res.end(body);
}

function sendText(res, status, text, contentType = 'text/plain; charset=utf-8') {
  res.writeHead(status, {
    'Content-Type': contentType,
    'Cache-Control': 'no-store',
    'Content-Length': Buffer.byteLength(text)
  });
  res.end(text);
}

async function readJsonBody(req, maxBytes = 16 * 1024) {
  let size = 0;
  const chunks = [];
  for await (const chunk of req) {
    size += chunk.length;
    if (size > maxBytes) {
      const error = new Error('请求体过大');
      error.status = 413;
      throw error;
    }
    chunks.push(chunk);
  }
  const raw = Buffer.concat(chunks).toString('utf8').trim();
  if (!raw) return {};
  try {
    return JSON.parse(raw);
  } catch {
    const error = new Error('请求体不是有效 JSON');
    error.status = 400;
    throw error;
  }
}

function dashboardViewKey() {
  return envValue('NEWAPI_DASHBOARD_VIEW_KEY') || envValue('NEWAPI_LOG_VIEW_KEY');
}

function isAuthorized(req) {
  const expected = dashboardViewKey();
  if (!expected) return true;
  const provided = String(req.headers['x-dashboard-key'] || req.headers['x-log-key'] || '').trim();
  return provided !== '' && provided === expected;
}

function requireAuthorized(req, res) {
  if (isAuthorized(req)) return true;
  sendJson(res, 401, {
    error: '访问密钥不正确',
    requiresKey: true
  });
  return false;
}

function boundedInt(searchParams, name, fallback, min, max) {
  const value = Number(searchParams.get(name));
  if (!Number.isFinite(value)) return fallback;
  return Math.max(min, Math.min(max, Math.trunc(value)));
}

function trimFilter(searchParams, name, maxLength = 120) {
  const value = String(searchParams.get(name) || '').trim();
  return value ? value.slice(0, maxLength) : '';
}

function siteHeaders(site) {
  const headers = {
    Accept: 'application/json',
    Authorization: `Bearer ${site.token}`
  };
  if (!site.skipUserHeader) headers['New-Api-User'] = String(site.userId || '1');
  return headers;
}

async function newApiGet(site, path, timeoutMs = 12000) {
  if (!path.startsWith('/api/')) {
    return { ok: false, error: '不允许的 NewAPI 路径', status: 400 };
  }

  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), timeoutMs);
  try {
    const response = await fetch(site.url + path, {
      method: 'GET',
      headers: siteHeaders(site),
      redirect: 'manual',
      signal: controller.signal
    });
    const text = await response.text();
    const data = text ? JSON.parse(text) : null;
    if (!response.ok) {
      return { ok: false, status: response.status, error: upstreamErrorMessage(data, response.status), data };
    }
    if (data && data.success === false) {
      return { ok: false, status: response.status, error: upstreamErrorMessage(data, response.status), data };
    }
    return { ok: true, status: response.status, data };
  } catch (error) {
    return {
      ok: false,
      status: 0,
      error: error.name === 'AbortError' ? 'NewAPI 请求超时' : `NewAPI 请求失败: ${error.message}`
    };
  } finally {
    clearTimeout(timer);
  }
}

function upstreamErrorMessage(data, status) {
  if (data && typeof data === 'object') {
    if (typeof data.message === 'string' && data.message.trim()) return data.message.trim();
    if (typeof data.error === 'string' && data.error.trim()) return data.error.trim();
  }
  return status ? `NewAPI HTTP ${status}` : 'NewAPI 请求失败';
}

async function firstSuccess(site, paths) {
  const errors = [];
  for (const path of paths) {
    const result = await newApiGet(site, path);
    if (result.ok) return result;
    errors.push(result.error || '请求失败');
  }
  return { ok: false, error: [...new Set(errors)].join('；') || '请求失败' };
}

function extractApiData(result) {
  const payload = result?.data;
  if (payload && typeof payload === 'object' && 'data' in payload) return payload.data;
  return payload;
}

function extractItems(result) {
  const data = extractApiData(result);
  if (Array.isArray(data)) return data;
  if (data && typeof data === 'object' && Array.isArray(data.items)) return data.items;
  return [];
}

function extractTotal(result, fallback) {
  const data = extractApiData(result);
  if (data && typeof data === 'object' && Number.isFinite(Number(data.total))) return Number(data.total);
  return fallback;
}

function extractStat(result) {
  const data = extractApiData(result) || {};
  return {
    quota: Number(data.quota) || 0,
    rpm: Number(data.rpm) || 0,
    tpm: Number(data.tpm) || 0
  };
}

function sanitizeLog(log) {
  return {
    id: log.id,
    createdAt: Number(log.created_at) || 0,
    type: Number(log.type) || 0,
    content: String(log.content || '').slice(0, 240),
    username: String(log.username || ''),
    tokenName: String(log.token_name || ''),
    modelName: String(log.model_name || ''),
    quota: Number(log.quota) || 0,
    promptTokens: Number(log.prompt_tokens) || 0,
    completionTokens: Number(log.completion_tokens) || 0,
    useTime: Number(log.use_time) || 0,
    isStream: Boolean(log.is_stream),
    channel: Number(log.channel) || 0,
    channelName: String(log.channel_name || ''),
    group: String(log.group || ''),
    requestId: String(log.request_id || '')
  };
}

function buildLogQuery({ page, size, type, startTimestamp, endTimestamp, modelName, tokenName, group, requestId }) {
  const params = new URLSearchParams({
    p: String(page),
    size: String(size),
    page_size: String(size),
    type: String(type),
    start_timestamp: String(startTimestamp),
    end_timestamp: String(endTimestamp)
  });
  if (modelName) params.set('model_name', modelName);
  if (tokenName) params.set('token_name', tokenName);
  if (group) params.set('group', group);
  if (requestId) params.set('request_id', requestId);
  return params.toString();
}

function buildStatQuery({ type, startTimestamp, endTimestamp, modelName, tokenName, group }) {
  const params = new URLSearchParams({
    type: String(type),
    start_timestamp: String(startTimestamp),
    end_timestamp: String(endTimestamp)
  });
  if (modelName) params.set('model_name', modelName);
  if (tokenName) params.set('token_name', tokenName);
  if (group) params.set('group', group);
  return params.toString();
}

async function loadSiteLogPage(site, options) {
  const logQuery = buildLogQuery(options);
  const logPaths = [
    `/api/log/self?${logQuery}`,
    `/api/log/?${logQuery}`
  ];

  const logsResult = await firstSuccess(site, logPaths);

  if (!logsResult.ok) {
    return {
      ok: false,
      error: logsResult.error || '日志接口不可用',
      logs: [],
      total: 0
    };
  }

  const rawLogs = extractItems(logsResult);
  return {
    ok: true,
    error: '',
    logs: rawLogs.map(sanitizeLog),
    total: extractTotal(logsResult, rawLogs.length)
  };
}

async function loadSiteLogs(site, options) {
  const statQuery = buildStatQuery(options);
  const statPaths = [
    `/api/log/self/stat?${statQuery}`,
    `/api/log/stat?${statQuery}`
  ];

  const [logsData, statResult] = await Promise.all([
    loadSiteLogPage(site, options),
    firstSuccess(site, statPaths)
  ]);

  return {
    ...logsData,
    stat: statResult.ok ? extractStat(statResult) : null,
    statError: statResult.ok ? '' : statResult.error
  };
}

async function loadSiteLogWindow(site, query, maxRows = MODEL_LOG_MAX_ROWS_PER_TYPE) {
  const rows = [];
  let total = 0;
  let page = 1;
  let lastError = '';

  while (rows.length < maxRows) {
    const data = await loadSiteLogPage(site, {
      ...query,
      page,
      size: MODEL_LOG_PAGE_SIZE
    });

    if (!data.ok) {
      lastError = data.error;
      return {
        ok: false,
        error: lastError,
        logs: rows,
        total: rows.length
      };
    }

    rows.push(...data.logs);
    total = data.total || rows.length;
    if (data.logs.length < MODEL_LOG_PAGE_SIZE || rows.length >= total) break;
    page += 1;
  }

  return {
    ok: true,
    error: '',
    logs: rows.slice(0, maxRows),
    total
  };
}

function channelEndpointCandidates(site, size) {
  const candidates = [];
  if (site.channelListEndpoint) candidates.push(site.channelListEndpoint);
  candidates.push(`/api/channel/search?keyword=&p=1&page_size=${size}`);
  candidates.push(`/api/channel/?p=1&page_size=${size}`);
  candidates.push(`/api/channel/search?keyword=&p=0&size=${size}`);
  candidates.push(`/api/channel/?p=0&size=${size}`);
  return [...new Set(candidates)];
}

function parseModels(value) {
  if (Array.isArray(value)) return value.map(String).map((item) => item.trim()).filter(Boolean);
  const raw = String(value || '').trim();
  if (!raw) return [];
  if (raw.startsWith('[')) {
    try {
      const parsed = JSON.parse(raw);
      if (Array.isArray(parsed)) return parsed.map(String).map((item) => item.trim()).filter(Boolean);
    } catch {
      // Fall back to delimiter parsing below.
    }
  }
  return raw
    .split(/[\n,]+/)
    .map((item) => item.trim())
    .filter(Boolean);
}

function sanitizeChannel(channel) {
  const groups = parseGroups(channel.group);
  return {
    id: Number(channel.id) || 0,
    name: String(channel.name || ''),
    status: Number(channel.status) || 0,
    group: String(channel.group || ''),
    groups,
    tag: channel.tag ? String(channel.tag) : '',
    responseTime: Number(channel.response_time) || 0,
    usedQuota: Number(channel.used_quota) || 0,
    models: parseModels(channel.models),
    testModel: channel.test_model ? String(channel.test_model) : '',
    baseUrl: channel.base_url ? String(channel.base_url) : ''
  };
}

function parseGroups(value) {
  return String(value || '')
    .split(/[\n,]+/)
    .map((item) => item.trim())
    .filter(Boolean);
}

async function loadSiteChannels(site) {
  const result = await firstSuccess(site, channelEndpointCandidates(site, 500));
  if (!result.ok) {
    return {
      ok: false,
      error: result.error || '通道接口不可用',
      channels: []
    };
  }
  return {
    ok: true,
    error: '',
    channels: extractItems(result).map(sanitizeChannel)
  };
}

function sanitizePricingModel(model) {
  return {
    modelName: String(model.model_name || model.modelName || '').trim(),
    quotaType: Number(model.quota_type ?? model.quotaType) || 0,
    modelRatio: finiteNumber(model.model_ratio ?? model.modelRatio, 0),
    completionRatio: finiteNumber(model.completion_ratio ?? model.completionRatio, 1),
    modelPrice: finiteNumber(model.model_price ?? model.modelPrice, 0),
    enableGroups: Array.isArray(model.enable_groups)
      ? model.enable_groups.map(String).map((item) => item.trim()).filter(Boolean)
      : [],
    billingMode: String(model.billing_mode || model.billingMode || '').trim()
  };
}

async function loadSitePricing(site) {
  const result = await newApiGet(site, '/api/pricing', 12000);
  if (!result.ok) {
    return {
      ok: false,
      error: result.error || '价格接口不可用',
      models: new Map(),
      groupRatio: {}
    };
  }

  const pricingPayload = result.data && typeof result.data === 'object' ? result.data : {};
  const models = new Map();
  for (const item of extractItems(result).map(sanitizePricingModel)) {
    if (item.modelName) models.set(item.modelName, item);
  }

  return {
    ok: true,
    error: '',
    models,
    groupRatio: pricingPayload.group_ratio && typeof pricingPayload.group_ratio === 'object'
      ? pricingPayload.group_ratio
      : {}
  };
}

function modelPriceForGroup(site, pricingData, model, group) {
  const pricing = pricingData.models.get(model);
  if (!pricing) return null;
  if (pricing.enableGroups.length && !pricing.enableGroups.includes(group) && !pricing.enableGroups.includes('all')) return null;

  const groupRatio = finiteNumber(pricingData.groupRatio?.[group], 1) || 1;
  const rechargeRatio = finiteNumber(site.rechargeRatio, 1) || 1;
  const multiplier = groupRatio * rechargeRatio;

  if (pricing.billingMode) {
    return {
      available: true,
      type: 'dynamic',
      currency: site.currency,
      unit: '',
      input: null,
      output: null,
      request: null
    };
  }

  if (pricing.quotaType === QUOTA_TYPE_REQUEST) {
    return {
      available: true,
      type: 'request',
      currency: site.currency,
      unit: '次',
      input: null,
      output: null,
      request: pricing.modelPrice * multiplier
    };
  }

  const input = pricing.modelRatio * 2 * multiplier;
  return {
    available: true,
    type: 'token',
    currency: site.currency,
    unit: '1M tokens',
    input,
    output: input * pricing.completionRatio,
    request: null
  };
}

function aggregateLogModels(logs) {
  const models = new Map();
  for (const log of logs) {
    const model = log.modelName || 'unknown';
    if (!models.has(model)) {
      models.set(model, {
        model,
        requests: 0,
        quota: 0,
        promptTokens: 0,
        completionTokens: 0,
        errors: 0
      });
    }
    const row = models.get(model);
    row.requests += 1;
    row.quota += log.quota;
    row.promptTokens += log.promptTokens;
    row.completionTokens += log.completionTokens;
    if (log.type === 5) row.errors += 1;
  }
  return [...models.values()].sort((a, b) => b.quota - a.quota || a.model.localeCompare(b.model));
}

function buildSitePayload(site, logsData, channelsData) {
  const enabledChannels = channelsData.channels.filter((channel) => channel.status === CHANNEL_STATUS_ENABLED);
  return {
    site: publicSite(site),
    logs: logsData.logs,
    logTotal: logsData.total,
    logOk: logsData.ok,
    logError: logsData.error,
    stat: logsData.stat,
    statError: logsData.statError,
    channelsOk: channelsData.ok,
    channelsError: channelsData.error,
    channelCount: channelsData.channels.length,
    enabledChannelCount: enabledChannels.length,
    modelUsage: aggregateLogModels(logsData.logs),
    channels: channelsData.channels
  };
}

function buildModelAvailability(sitePayloads) {
  const modelMap = new Map();

  for (const payload of sitePayloads) {
    const siteName = payload.site.name;
    for (const channel of payload.channels) {
      for (const model of channel.models) {
        if (!modelMap.has(model)) {
          modelMap.set(model, {
            model,
            availableSites: 0,
            totalEnabledChannels: 0,
            totalChannels: 0,
            perSite: {}
          });
        }
        const row = modelMap.get(model);
        if (!row.perSite[siteName]) {
          row.perSite[siteName] = {
            status: 'unavailable',
            enabledChannels: 0,
            totalChannels: 0,
            groups: [],
            bestResponseTime: null,
            channelNames: []
          };
        }
        const cell = row.perSite[siteName];
        cell.totalChannels += 1;
        row.totalChannels += 1;
        if (channel.status === CHANNEL_STATUS_ENABLED) {
          cell.enabledChannels += 1;
          row.totalEnabledChannels += 1;
          cell.status = 'available';
          if (channel.responseTime > 0 && (cell.bestResponseTime === null || channel.responseTime < cell.bestResponseTime)) {
            cell.bestResponseTime = channel.responseTime;
          }
        }
        if (channel.group && !cell.groups.includes(channel.group)) cell.groups.push(channel.group);
        if (channel.name && cell.channelNames.length < 3 && !cell.channelNames.includes(channel.name)) {
          cell.channelNames.push(channel.name);
        }
      }
    }
  }

  const models = [...modelMap.values()].map((row) => {
    let availableSites = 0;
    for (const cell of Object.values(row.perSite)) {
      if (cell.enabledChannels > 0) availableSites += 1;
      cell.groups.sort((a, b) => a.localeCompare(b));
    }
    return {
      ...row,
      availableSites
    };
  });

  return models.sort((a, b) => b.availableSites - a.availableSites || b.totalEnabledChannels - a.totalEnabledChannels || a.model.localeCompare(b.model));
}

function modelStatusKey(siteName, model) {
  return `${siteName}\u0000${model}`;
}

function latestTimestamp(logs) {
  return logs.reduce((max, log) => Math.max(max, Number(log.createdAt) || 0), 0);
}

function statusFromCounts(successCount, failureCount) {
  if (successCount > 0 && failureCount === 0) return 'operational';
  if (successCount > 0 && failureCount > 0) return 'degraded';
  if (successCount === 0 && failureCount > 0) return 'down';
  return 'unknown';
}

function modelRowStatus(cells) {
  const configuredCells = cells.filter((cell) => cell.configured);
  if (configuredCells.some((cell) => cell.status === 'down')) return 'down';
  if (configuredCells.some((cell) => cell.status === 'degraded')) return 'degraded';
  if (configuredCells.some((cell) => cell.status === 'unknown')) return 'unknown';
  if (configuredCells.some((cell) => cell.status === 'operational')) return 'operational';
  return 'unknown';
}

function siteStatusFromCounts(successCount, failureCount) {
  return statusFromCounts(successCount, failureCount);
}

function successRate(successCount, failureCount) {
  const total = successCount + failureCount;
  return total > 0 ? successCount / total : null;
}

function buildChannelModelIndex(channels) {
  const index = new Map();
  for (const channel of channels) {
    for (const model of channel.models) {
      if (!index.has(model)) {
        index.set(model, {
          model,
          totalChannels: 0,
          enabledChannels: 0,
          groups: [],
          groupStats: {}
        });
      }
      const item = index.get(model);
      item.totalChannels += 1;
      if (channel.status === CHANNEL_STATUS_ENABLED) item.enabledChannels += 1;
      for (const group of channel.groups) {
        if (!item.groups.includes(group)) item.groups.push(group);
        if (!item.groupStats[group]) {
          item.groupStats[group] = {
            group,
            totalChannels: 0,
            enabledChannels: 0
          };
        }
        item.groupStats[group].totalChannels += 1;
        if (channel.status === CHANNEL_STATUS_ENABLED) item.groupStats[group].enabledChannels += 1;
      }
    }
  }
  for (const item of index.values()) {
    item.groups.sort((a, b) => a.localeCompare(b));
  }
  return index;
}

function groupLogsByModel(logs) {
  const map = new Map();
  for (const log of logs) {
    const model = String(log.modelName || '').trim();
    if (!model) continue;
    if (!map.has(model)) map.set(model, []);
    map.get(model).push(log);
  }
  return map;
}

function groupLogsByModelAndGroup(logs) {
  const map = new Map();
  for (const log of logs) {
    const model = String(log.modelName || '').trim();
    if (!model) continue;
    if (!map.has(model)) map.set(model, new Map());
    const modelMap = map.get(model);
    const groups = parseGroups(log.group);
    for (const group of groups) {
      if (!modelMap.has(group)) modelMap.set(group, []);
      modelMap.get(group).push(log);
    }
  }
  return map;
}

function buildSiteModelStatus(site, channelsData, successLogsData, errorLogsData, pricingData) {
  const channelIndex = buildChannelModelIndex(channelsData.channels);
  const successByModel = groupLogsByModel(successLogsData.logs);
  const errorByModel = groupLogsByModel(errorLogsData.logs);
  const successByModelGroup = groupLogsByModelAndGroup(successLogsData.logs);
  const errorByModelGroup = groupLogsByModelAndGroup(errorLogsData.logs);
  const cells = {};
  const groupSet = new Set();
  let operationalModelCount = 0;
  let degradedModelCount = 0;
  let downModelCount = 0;
  let unknownModelCount = 0;

  for (const [model, channelStats] of channelIndex) {
    const modelSuccessLogs = successByModel.get(model) || [];
    const modelErrorLogs = errorByModel.get(model) || [];
    const successCount = modelSuccessLogs.length;
    const failureCount = modelErrorLogs.length;
    const status = statusFromCounts(successCount, failureCount);
    const key = modelStatusKey(site.name, model);
    const groupStats = {};
    for (const group of channelStats.groups) {
      groupSet.add(group);
      const groupSuccessLogs = successByModelGroup.get(model)?.get(group) || [];
      const groupErrorLogs = errorByModelGroup.get(model)?.get(group) || [];
      const groupSuccessCount = groupSuccessLogs.length;
      const groupFailureCount = groupErrorLogs.length;
      groupStats[group] = {
        group,
        configured: true,
        pricing: modelPriceForGroup(site, pricingData, model, group),
        status: statusFromCounts(groupSuccessCount, groupFailureCount),
        successRate: successRate(groupSuccessCount, groupFailureCount),
        requestCount: groupSuccessCount + groupFailureCount,
        successCount: groupSuccessCount,
        failureCount: groupFailureCount,
        lastSuccessAt: latestTimestamp(groupSuccessLogs),
        lastFailureAt: latestTimestamp(groupErrorLogs),
        lastSeenAt: Math.max(latestTimestamp(groupSuccessLogs), latestTimestamp(groupErrorLogs)),
        channelCount: channelStats.groupStats[group]?.totalChannels || 0,
        enabledChannelCount: channelStats.groupStats[group]?.enabledChannels || 0
      };
    }
    const cell = {
      siteName: site.name,
      model,
      configured: true,
      status,
      successRate: successRate(successCount, failureCount),
      requestCount: successCount + failureCount,
      successCount,
      failureCount,
      lastSuccessAt: latestTimestamp(modelSuccessLogs),
      lastFailureAt: latestTimestamp(modelErrorLogs),
      lastSeenAt: Math.max(latestTimestamp(modelSuccessLogs), latestTimestamp(modelErrorLogs)),
      channelCount: channelStats.totalChannels,
      enabledChannelCount: channelStats.enabledChannels,
      groups: channelStats.groups,
      groupStats,
      pricing: modelPriceForGroup(site, pricingData, model, channelStats.groups[0] || ''),
      manualTest: modelTestResults.get(key) || null,
      nextTestAllowedAt: Math.floor((modelTestCooldowns.get(key) || 0) / 1000)
    };

    if (status === 'operational') operationalModelCount += 1;
    else if (status === 'degraded') degradedModelCount += 1;
    else if (status === 'down') downModelCount += 1;
    else unknownModelCount += 1;

    cells[model] = cell;
  }

  const successCount = successLogsData.logs.length;
  const failureCount = errorLogsData.logs.length;
  const modelCount = channelIndex.size;
  return {
    site: publicSite(site),
    groups: [...groupSet].sort((a, b) => a.localeCompare(b)),
    status: siteStatusFromCounts(successCount, failureCount),
    successRate: successRate(successCount, failureCount),
    requestCount: successCount + failureCount,
    successCount,
    failureCount,
    modelCount,
    operationalModelCount,
    degradedModelCount,
    downModelCount,
    unknownModelCount,
    modelAvailabilityRate: modelCount > 0 ? operationalModelCount / modelCount : null,
    logOk: successLogsData.ok && errorLogsData.ok,
    logError: [successLogsData.error, errorLogsData.error].filter(Boolean).join('；'),
    channelsOk: channelsData.ok,
    channelsError: channelsData.error,
    pricingOk: pricingData.ok,
    pricingError: pricingData.error,
    cells
  };
}

function sortModelStatusRows(rows) {
  const rank = {
    down: 0,
    degraded: 1,
    unknown: 2,
    operational: 3
  };
  return rows.sort((a, b) => {
    const statusRank = rank[a.status] - rank[b.status];
    if (statusRank !== 0) return statusRank;
    return a.model.localeCompare(b.model);
  });
}

async function buildModelStatus() {
  const config = await readManagedApiSitesConfig();
  const sites = config.sites;
  const now = Math.floor(Date.now() / 1000);
  const startTimestamp = now - MODEL_STATUS_WINDOW_SECONDS;

  if (!sites.length) {
    return {
      configured: false,
      configError: config.error,
      requiresKey: Boolean(dashboardViewKey()),
      generatedAt: now,
      expiresAt: now + MODEL_STATUS_WINDOW_SECONDS,
      windowSeconds: MODEL_STATUS_WINDOW_SECONDS,
      refreshEverySeconds: MODEL_STATUS_WINDOW_SECONDS,
      sites: [],
      models: [],
      totals: {
        siteCount: 0,
        modelCount: 0,
        requestCount: 0,
        successCount: 0,
        failureCount: 0,
        operational: 0,
        degraded: 0,
        down: 0,
        unknown: 0
      }
    };
  }

  const queryBase = {
    page: 1,
    size: MODEL_LOG_PAGE_SIZE,
    startTimestamp,
    endTimestamp: now,
    modelName: '',
    tokenName: '',
    group: '',
    requestId: ''
  };

  const siteStatuses = await Promise.all(sites.map(async (site) => {
    const [channelsData, successLogsData, errorLogsData, pricingData] = await Promise.all([
      loadSiteChannels(site),
      loadSiteLogWindow(site, { ...queryBase, type: LOG_TYPE_CONSUME }),
      loadSiteLogWindow(site, { ...queryBase, type: LOG_TYPE_ERROR }),
      loadSitePricing(site)
    ]);
    return buildSiteModelStatus(site, channelsData, successLogsData, errorLogsData, pricingData);
  }));

  const modelNames = [...new Set(siteStatuses.flatMap((siteStatus) => Object.keys(siteStatus.cells)))].sort((a, b) => a.localeCompare(b));
  const rows = sortModelStatusRows(modelNames.map((model) => {
    const perSite = {};
    for (const siteStatus of siteStatuses) {
      perSite[siteStatus.site.name] = siteStatus.cells[model] || {
        siteName: siteStatus.site.name,
        model,
        configured: false,
        status: 'unknown',
        successRate: null,
        requestCount: 0,
        successCount: 0,
        failureCount: 0,
        lastSuccessAt: 0,
        lastFailureAt: 0,
        lastSeenAt: 0,
        channelCount: 0,
        enabledChannelCount: 0,
        groups: [],
        groupStats: {},
        pricing: null,
        manualTest: null,
        nextTestAllowedAt: 0
      };
    }
    const cells = Object.values(perSite);
    return {
      model,
      status: modelRowStatus(cells),
      operationalSites: cells.filter((cell) => cell.configured && cell.status === 'operational').length,
      configuredSites: cells.filter((cell) => cell.configured).length,
      perSite
    };
  }));

  const totals = rows.reduce(
    (acc, row) => {
      acc[row.status] += 1;
      return acc;
    },
    {
      siteCount: siteStatuses.length,
      modelCount: rows.length,
      requestCount: siteStatuses.reduce((sum, item) => sum + item.requestCount, 0),
      successCount: siteStatuses.reduce((sum, item) => sum + item.successCount, 0),
      failureCount: siteStatuses.reduce((sum, item) => sum + item.failureCount, 0),
      operational: 0,
      degraded: 0,
      down: 0,
      unknown: 0
    }
  );

  return {
    configured: true,
    configError: config.error,
    requiresKey: Boolean(dashboardViewKey()),
    generatedAt: now,
    expiresAt: now + MODEL_STATUS_WINDOW_SECONDS,
    windowSeconds: MODEL_STATUS_WINDOW_SECONDS,
    refreshEverySeconds: MODEL_STATUS_WINDOW_SECONDS,
    sites: siteStatuses.map(({ cells, ...siteStatus }) => siteStatus),
    models: rows,
    totals
  };
}

async function getModelStatus(force = false) {
  const now = Date.now();
  if (!force && modelStatusCache.value && now < modelStatusCache.expiresAt) return modelStatusCache.value;
  if (!force && modelStatusCache.pending) return modelStatusCache.pending;

  modelStatusCache.pending = buildModelStatus()
    .then((status) => {
      modelStatusCache.value = status;
      modelStatusCache.expiresAt = Date.now() + MODEL_STATUS_CACHE_MS;
      return status;
    })
    .finally(() => {
      modelStatusCache.pending = null;
    });

  return modelStatusCache.pending;
}

function testResultMessage(result) {
  const data = result?.data;
  if (data && typeof data === 'object') {
    if (typeof data.message === 'string' && data.message.trim()) return data.message.trim();
    if (data.data && typeof data.data === 'object' && typeof data.data.message === 'string') return data.data.message.trim();
  }
  return result.ok ? '测试通过' : result.error || '测试失败';
}

function supportsStreamModelTest(model) {
  const name = String(model || '').trim().toLowerCase();
  if (!name) return true;
  return !(
    name.includes('rerank') ||
    name.includes('embedding') ||
    name.includes('embed') ||
    name.startsWith('m3e') ||
    name.includes('bge-') ||
    name.includes('seedream')
  );
}

function channelTestPath(channelId, model, stream) {
  const params = new URLSearchParams({ model });
  if (stream) params.set('stream', 'true');
  return `/api/channel/test/${channelId}?${params.toString()}`;
}

function refreshModelRowStatus(modelStatus, model) {
  const row = modelStatus.models.find((item) => item.model === model);
  if (!row) return;
  const cells = Object.values(row.perSite);
  row.status = modelRowStatus(cells);
  row.operationalSites = cells.filter((cell) => cell.configured && cell.status === 'operational').length;
  row.configuredSites = cells.filter((cell) => cell.configured).length;
}

function applyManualTestResult(modelStatus, siteName, model, test) {
  const row = modelStatus?.models?.find((item) => item.model === model);
  const cell = row?.perSite?.[siteName];
  if (!cell) return null;
  cell.manualTest = test;
  cell.nextTestAllowedAt = test.nextAllowedAt;
  refreshModelRowStatus(modelStatus, model);
  return cell;
}

async function testModelCell(siteName, model) {
  return testModelCellInGroup(siteName, model, '');
}

async function testModelCellInGroup(siteName, model, group) {
  const config = await readManagedApiSitesConfig();
  const site = config.sites.find((item) => item.name === siteName);
  if (!site) {
    const error = new Error('站点不存在');
    error.status = 404;
    throw error;
  }

  const status = await getModelStatus();
  const row = status.models.find((item) => item.model === model);
  const cell = row?.perSite?.[siteName];
  if (!cell?.configured) {
    const error = new Error('该站点没有配置这个模型');
    error.status = 400;
    throw error;
  }
  const normalizedGroup = String(group || '').trim();
  if (normalizedGroup && !cell.groupStats?.[normalizedGroup]?.configured) {
    const error = new Error('该站点分组没有配置这个模型');
    error.status = 400;
    throw error;
  }

  const key = modelStatusKey(siteName, model);
  const nowMs = Date.now();
  const blockedUntil = modelTestCooldowns.get(key) || 0;
  if (blockedUntil > nowMs) {
    const error = new Error('该模型测试仍在冷却中');
    error.status = 429;
    error.nextAllowedAt = Math.floor(blockedUntil / 1000);
    throw error;
  }

  const channelsData = await loadSiteChannels(site);
  if (!channelsData.ok) {
    const error = new Error(channelsData.error || '通道接口不可用');
    error.status = 502;
    throw error;
  }
  const candidates = channelsData.channels
    .filter((channel) => channel.status === CHANNEL_STATUS_ENABLED)
    .filter((channel) => channel.models.includes(model))
    .filter((channel) => !normalizedGroup || channel.groups.includes(normalizedGroup))
    .sort((a, b) => {
      const aMs = a.responseTime > 0 ? a.responseTime : Number.POSITIVE_INFINITY;
      const bMs = b.responseTime > 0 ? b.responseTime : Number.POSITIVE_INFINITY;
      return aMs - bMs || a.id - b.id;
    });
  if (!candidates.length) {
    const error = new Error('当前单元格没有启用通道可测试');
    error.status = 400;
    throw error;
  }

  const nextAllowedMs = nowMs + MODEL_TEST_COOLDOWN_MS;
  modelTestCooldowns.set(key, nextAllowedMs);
  let result = null;
  const stream = supportsStreamModelTest(model);
  for (const channel of candidates) {
    result = await newApiGet(site, channelTestPath(channel.id, model, stream), 45000);
    if (result.ok) break;
  }
  const test = {
    ok: Boolean(result?.ok),
    status: result?.ok ? 'operational' : 'down',
    stream,
    testedAt: Math.floor(Date.now() / 1000),
    message: testResultMessage(result).slice(0, 180),
    nextAllowedAt: Math.floor(nextAllowedMs / 1000)
  };
  modelTestResults.set(key, test);
  const updatedCell = applyManualTestResult(status, siteName, model, test);
  return {
    siteName,
    model,
    test,
    cell: updatedCell
  };
}

async function buildOverview(searchParams) {
  const config = await readManagedApiSitesConfig();
  const sites = config.sites;
  const now = Math.floor(Date.now() / 1000);
  const hours = boundedInt(searchParams, 'hours', 24, 1, 720);
  const page = boundedInt(searchParams, 'page', 1, 1, 10000);
  const size = boundedInt(searchParams, 'size', 30, 1, 100);
  const type = boundedInt(searchParams, 'type', LOG_TYPE_CONSUME, 0, 6);
  const endTimestamp = boundedInt(searchParams, 'end_timestamp', now, 1, Number.MAX_SAFE_INTEGER);
  const startTimestamp = boundedInt(searchParams, 'start_timestamp', endTimestamp - hours * 3600, 1, Number.MAX_SAFE_INTEGER);
  const query = {
    page,
    size,
    type,
    startTimestamp: Math.min(startTimestamp, endTimestamp),
    endTimestamp: Math.max(startTimestamp, endTimestamp),
    modelName: trimFilter(searchParams, 'model_name'),
    tokenName: trimFilter(searchParams, 'token_name'),
    group: trimFilter(searchParams, 'group'),
    requestId: trimFilter(searchParams, 'request_id')
  };

  const sitePayloads = await Promise.all(sites.map(async (site) => {
    const [logsData, channelsData] = await Promise.all([
      loadSiteLogs(site, query),
      loadSiteChannels(site)
    ]);
    return buildSitePayload(site, logsData, channelsData);
  }));

  const totals = sitePayloads.reduce(
    (acc, payload) => {
      acc.logRows += payload.logs.length;
      acc.totalLogs += payload.logTotal || payload.logs.length;
      acc.channelCount += payload.channelCount;
      acc.enabledChannelCount += payload.enabledChannelCount;
      if (payload.stat) {
        acc.quota += payload.stat.quota;
        acc.rpm += payload.stat.rpm;
        acc.tpm += payload.stat.tpm;
      }
      return acc;
    },
    { quota: 0, rpm: 0, tpm: 0, logRows: 0, totalLogs: 0, channelCount: 0, enabledChannelCount: 0 }
  );

  const modelAvailability = buildModelAvailability(sitePayloads);
  const allLogs = sitePayloads
    .flatMap((payload) => payload.logs.map((log) => ({ ...log, siteName: payload.site.name })))
    .sort((a, b) => b.createdAt - a.createdAt);

  return {
    configured: sites.length > 0,
    configError: config.error,
    requiresKey: Boolean(dashboardViewKey()),
    generatedAt: now,
    window: {
      hours,
      startTimestamp: query.startTimestamp,
      endTimestamp: query.endTimestamp
    },
    query: {
      type,
      page,
      size,
      modelName: query.modelName,
      tokenName: query.tokenName,
      group: query.group,
      requestId: query.requestId
    },
    sites: sitePayloads.map(({ channels, ...payload }) => payload),
    allLogs,
    totals,
    modelAvailability
  };
}

function getClientIp(req) {
  const cfIp = String(req.headers['cf-connecting-ip'] || '').trim();
  if (cfIp) return cfIp;
  const realIp = String(req.headers['x-real-ip'] || '').trim();
  if (realIp) return realIp;
  const forwarded = String(req.headers['x-forwarded-for'] || '').split(',').map((item) => item.trim()).find(Boolean);
  if (forwarded) return forwarded;
  return req.socket.remoteAddress || '';
}

async function handleApi(req, res, pathname, searchParams) {
  const method = req.method || 'GET';

  if (pathname === '/api/health') {
    if (method !== 'GET') {
      sendJson(res, 405, { error: 'Only GET is supported' });
      return;
    }
    sendJson(res, 200, { ok: true });
    return;
  }

  if (pathname === '/api/client') {
    if (method !== 'GET') {
      sendJson(res, 405, { error: 'Only GET is supported' });
      return;
    }
    sendJson(res, 200, {
      ip: getClientIp(req),
      serverTime: Date.now(),
      origin: req.headers.origin || '',
      userAgent: req.headers['user-agent'] || ''
    });
    return;
  }

  if (pathname === '/api/connectivity/targets') {
    if (method !== 'GET') {
      sendJson(res, 405, { error: 'Only GET is supported' });
      return;
    }
    sendJson(res, 200, { groups: connectivityGroupsFromEnv() });
    return;
  }

  if (pathname === '/api/newapi/sites') {
    if (method !== 'GET') {
      sendJson(res, 405, { error: 'Only GET is supported' });
      return;
    }
    if (!requireAuthorized(req, res)) return;
    const config = await readManagedApiSitesConfig();
    sendJson(res, config.error && config.sites.length === 0 ? 500 : 200, {
      configured: config.sites.length > 0,
      requiresKey: Boolean(dashboardViewKey()),
      error: config.error,
      sites: config.sites.map(publicSite)
    });
    return;
  }

  if (pathname === '/api/newapi/overview') {
    if (method !== 'GET') {
      sendJson(res, 405, { error: 'Only GET is supported' });
      return;
    }
    if (!requireAuthorized(req, res)) return;
    const overview = await buildOverview(searchParams);
    sendJson(res, overview.configError && !overview.configured ? 500 : 200, overview);
    return;
  }

  if (pathname === '/api/newapi/model-status') {
    if (method !== 'GET') {
      sendJson(res, 405, { error: 'Only GET is supported' });
      return;
    }
    if (!requireAuthorized(req, res)) return;
    const status = await getModelStatus(searchParams.get('refresh') === '1');
    sendJson(res, status.configError && !status.configured ? 500 : 200, status);
    return;
  }

  if (pathname === '/api/newapi/model-status/test') {
    if (method !== 'POST') {
      sendJson(res, 405, { error: 'Only POST is supported' });
      return;
    }
    if (!requireAuthorized(req, res)) return;

    let body;
    try {
      body = await readJsonBody(req);
    } catch (error) {
      sendJson(res, error.status || 400, { error: error.message || '请求体无效' });
      return;
    }

    const siteName = String(body.siteName || '').trim().slice(0, 120);
    const model = String(body.model || '').trim().slice(0, 240);
    const group = String(body.group || '').trim().slice(0, 120);
    if (!siteName || !model) {
      sendJson(res, 400, { error: 'siteName 和 model 必填' });
      return;
    }

    try {
      const result = await testModelCellInGroup(siteName, model, group);
      sendJson(res, 200, result);
    } catch (error) {
      sendJson(res, error.status || 500, {
        error: error.message || '模型测试失败',
        nextAllowedAt: error.nextAllowedAt || 0
      });
    }
    return;
  }

  sendJson(res, 404, { error: 'API not found' });
}

async function serveStatic(req, res, pathname) {
  const requestedPath = decodeURIComponent(pathname === '/' ? '/index.html' : pathname);
  const absolutePath = resolve(FRONTEND_DIR, '.' + requestedPath);
  if (!absolutePath.startsWith(FRONTEND_DIR)) {
    sendText(res, 403, 'Forbidden');
    return;
  }

  try {
    const content = await readFile(absolutePath);
    const contentType = MIME_TYPES[extname(absolutePath).toLowerCase()] || 'application/octet-stream';
    res.writeHead(200, {
      'Content-Type': contentType,
      'Cache-Control': /html|css|javascript/.test(contentType) ? 'no-store' : 'public, max-age=300',
      'Content-Length': content.length
    });
    res.end(content);
  } catch {
    const fallback = join(FRONTEND_DIR, 'index.html');
    try {
      const content = await readFile(fallback);
      res.writeHead(200, {
        'Content-Type': MIME_TYPES['.html'],
        'Cache-Control': 'no-store',
        'Content-Length': content.length
      });
      res.end(content);
    } catch {
      sendText(res, 503, 'Frontend not found');
    }
  }
}

const server = createServer(async (req, res) => {
  try {
    const url = new URL(req.url || '/', `http://${req.headers.host || 'localhost'}`);

    if (url.pathname.startsWith('/api/')) {
      await handleApi(req, res, url.pathname, url.searchParams);
      return;
    }

    if ((req.method || 'GET') !== 'GET') {
      sendJson(res, 405, { error: 'Only GET is supported' });
      return;
    }

    await serveStatic(req, res, url.pathname);
  } catch (error) {
    console.error(error);
    sendJson(res, 500, { error: 'Internal server error' });
  }
});

server.listen(PORT, '0.0.0.0', () => {
  console.log(`network-detact dashboard listening on :${PORT}`);
});
