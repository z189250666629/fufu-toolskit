#!/usr/bin/env node
/**
 * fufu-act API helpers.
 *
 * This module is intentionally self-contained in the merged monorepo.  The
 * original project imported helpers from an external local `skills/fufu-shop`
 * directory, which would make this app fail after being moved.  Configure the
 * NewAPI and MCY shop endpoints with environment variables documented in the
 * root README.
 */

const DEFAULT_QUOTA_UNIT = Number(process.env.FUFU_QUOTA_UNIT || process.env.NEWAPI_QUOTA_UNIT || 500000);

function envValue(name) {
  const value = process.env[name];
  return typeof value === 'string' ? value.trim() : '';
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

function asArray(value) {
  return Array.isArray(value) ? value : [];
}

function makeCardType(dollars, unit = 9, group = 'mix') {
  return {
    quota: Math.round(Number(dollars) * DEFAULT_QUOTA_UNIT),
    unit,
    group,
  };
}

// Card type names used by the restock scripts.  Aliases are kept so existing
// one-off node commands can pass either the display name or a compact key.
export const CARD_TYPES = {
  'FuFu 55次混合特惠卡': makeCardType(55, 3),
  daily55: makeCardType(55, 3),
  '混合卡 月一百次卡': makeCardType(100, 9),
  monthly100: makeCardType(100, 9),
  '混合卡 月一百五十次卡': makeCardType(150, 9),
  monthly150: makeCardType(150, 9),
  '混合卡 月三百次卡': makeCardType(300, 9),
  monthly300: makeCardType(300, 9),
  '混合卡 月五百次卡': makeCardType(500, 9),
  monthly500: makeCardType(500, 9),
  '混合卡 月一千次卡': makeCardType(1000, 9),
  monthly1000: makeCardType(1000, 9),
};

function newApiConfig() {
  const baseUrl = normalizeBaseUrl(
    envValue('FUFU_API_BASE_URL')
      || envValue('FUFU_API_URL')
      || envValue('NEWAPI_API_SITE_URL')
      || 'https://api.fufuflower.top'
  );
  const token = envValue('FUFU_API_TOKEN')
    || envValue('NEWAPI_API_SITE_TOKEN')
    || envValue('NEWAPI_TOKEN_SITE_TOKEN');
  const userId = envValue('FUFU_API_USER_ID') || envValue('NEWAPI_API_SITE_USER_ID') || '1';

  if (!baseUrl) throw new Error('缺少有效的 FUFU_API_BASE_URL / NEWAPI_API_SITE_URL');
  if (!token) throw new Error('缺少 FUFU_API_TOKEN / NEWAPI_API_SITE_TOKEN');
  return { baseUrl, token, userId };
}

function endpointUrl(baseUrl, endpoint) {
  const path = String(endpoint || '');
  if (/^https?:\/\//i.test(path)) return path;
  return `${baseUrl}${path.startsWith('/') ? '' : '/'}${path}`;
}

async function parseResponse(response) {
  const text = await response.text();
  if (!text.trim()) return {};
  try {
    return JSON.parse(text);
  } catch {
    return { message: text };
  }
}

export async function fufuApi(method, endpoint, body = undefined) {
  const { baseUrl, token, userId } = newApiConfig();
  const headers = {
    Authorization: `Bearer ${token}`,
    'New-Api-User': userId,
  };
  const init = { method: method.toUpperCase(), headers };
  if (body !== undefined && init.method !== 'GET' && init.method !== 'HEAD') {
    headers['Content-Type'] = 'application/json';
    init.body = JSON.stringify(body);
  }

  const response = await fetch(endpointUrl(baseUrl, endpoint), init);
  const data = await parseResponse(response);
  if (!response.ok) {
    return {
      success: false,
      status: response.status,
      message: data.message || data.error || response.statusText,
      ...data,
    };
  }
  return data;
}

let mcyCookie = envValue('MCY_COOKIE');

function mcyConfig() {
  const baseUrl = normalizeBaseUrl(envValue('MCY_BASE_URL') || envValue('SHOP_BASE_URL'));
  if (!baseUrl) throw new Error('缺少 MCY_BASE_URL / SHOP_BASE_URL');
  return {
    baseUrl,
    username: envValue('MCY_USERNAME') || envValue('SHOP_USERNAME'),
    password: envValue('MCY_PASSWORD') || envValue('SHOP_PASSWORD'),
    loginEndpoint: envValue('MCY_LOGIN_ENDPOINT') || '/admin/login',
  };
}

export async function mcyLogin() {
  if (mcyCookie) return { success: true, cookie: mcyCookie, fromEnv: !!envValue('MCY_COOKIE') };

  const { baseUrl, username, password, loginEndpoint } = mcyConfig();
  if (!username || !password) {
    throw new Error('缺少 MCY_COOKIE，或 MCY_USERNAME / MCY_PASSWORD');
  }

  const response = await fetch(endpointUrl(baseUrl, loginEndpoint), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password }),
  });
  const setCookie = response.headers.get('set-cookie');
  const data = await parseResponse(response);
  if (!response.ok) {
    throw new Error(data.message || data.error || `MCY login failed: ${response.status}`);
  }
  if (setCookie) {
    mcyCookie = setCookie.split(',').map((item) => item.split(';')[0]).join('; ');
  }
  return data;
}

export async function mcyPost(endpoint, payload = {}) {
  const { baseUrl } = mcyConfig();
  if (!mcyCookie) await mcyLogin();
  const response = await fetch(endpointUrl(baseUrl, endpoint), {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Cookie: mcyCookie,
    },
    body: JSON.stringify(payload),
  });
  const data = await parseResponse(response);
  if (!response.ok) {
    return {
      success: false,
      status: response.status,
      message: data.message || data.error || response.statusText,
      ...data,
    };
  }
  return data;
}

/**
 * Upload generated card keys to the MCY virtual-card plugin.
 *
 * MCY deployments often customize the exact endpoint/payload.  Override the
 * endpoint with MCY_UPLOAD_ENDPOINT when needed.  The default payload includes
 * several common field names so it works with typical virtual-card handlers.
 */
export async function uploadCards(cardName, keys, remark = '') {
  const cardList = asArray(keys).map((key) => String(key).trim()).filter(Boolean);
  if (!cardList.length) return { success: false, uploaded: 0, error: '没有可上传的卡密' };

  const endpoint = envValue('MCY_UPLOAD_ENDPOINT') || '/plugin/virtual-card-ship/card/add';
  const joined = cardList.join('\n');
  const payload = {
    name: cardName,
    cardName,
    title: cardName,
    cards: joined,
    card: joined,
    content: joined,
    list: cardList,
    remark,
  };
  const result = await mcyPost(endpoint, payload);
  return {
    success: result.success !== false,
    uploaded: cardList.length,
    response: result,
  };
}

/**
 * Activity 版：在次数 fufu 生成卡密（命名规则: {额度}-act-{xxxx}）
 * @param {string} cardType - CARD_TYPES 中的 key
 * @param {number} count - 生成数量
 * @param {string} remark - 备注
 * @returns {object} { success, generated, keys, errors }
 */
export async function generateCardsActivity(cardType, count = 1, remark = '') {
  const cfg = CARD_TYPES[cardType];
  if (!cfg) return { success: false, error: `未知卡类型: ${cardType}` };

  const keys = [];
  const errors = [];

  const dollars = cfg.quota / DEFAULT_QUOTA_UNIT;
  const suffix = Date.now().toString(36).slice(-4);
  const name = `${dollars}-act-${suffix}`;
  const res = await fufuApi('POST', `/api/token/tokens?tokenCount=${count}`, {
    name,
    remain_quota: cfg.quota,
    unlimited_quota: false,
    expired_time: -1,
    group: cfg.group,
    interval_quota: cfg.quota,
    interval_time: -1,
    trigger_last_time: 0,
    interval_unit: cfg.unit,
    remark,
  });

  if (!res.success) {
    errors.push(`批量生成失败: ${res.message || JSON.stringify(res)}`);
  } else if (res.keys?.length) {
    keys.push(...res.keys);
  } else if (res.data?.keys?.length) {
    keys.push(...res.data.keys);
  } else {
    errors.push('批量生成成功但未返回 keys');
  }

  return { success: keys.length > 0, generated: keys.length, keys, errors };
}

/**
 * Activity 版：生成随机额度的特惠卡（每张额度在 minDollars~maxDollars 之间随机整数）
 * @param {string} cardType - CARD_TYPES 中的 key（用于获取 unit/group 等基础配置）
 * @param {number} count - 生成数量
 * @param {number} minDollars - 最小额度（美元整数）
 * @param {number} maxDollars - 最大额度（美元整数）
 * @param {string} remark - 备注
 * @returns {object} { success, generated, keys, details, errors }
 */
export async function generateCardsActivityRandom(cardType, count = 1, minDollars = 55, maxDollars = 75, remark = '') {
  const cfg = CARD_TYPES[cardType];
  if (!cfg) return { success: false, error: `未知卡类型: ${cardType}` };

  const keys = [];
  const details = [];
  const errors = [];

  for (let i = 0; i < count; i += 1) {
    const dollars = Math.floor(Math.random() * (maxDollars - minDollars + 1)) + minDollars;
    const quota = dollars * DEFAULT_QUOTA_UNIT;
    const suffix = (Date.now() + i).toString(36).slice(-4);
    const name = `${dollars}-act-${suffix}`;

    const res = await fufuApi('POST', '/api/token/tokens?tokenCount=1', {
      name,
      remain_quota: quota,
      unlimited_quota: false,
      expired_time: -1,
      group: cfg.group,
      interval_quota: quota,
      interval_time: -1,
      trigger_last_time: 0,
      interval_unit: cfg.unit,
      remark,
    });

    if (!res.success) {
      errors.push(`第${i + 1}张($${dollars})生成失败: ${res.message || JSON.stringify(res)}`);
    } else if (res.keys?.length || res.data?.keys?.length) {
      const key = (res.keys || res.data.keys)[0];
      keys.push(key);
      details.push({ key, dollars, quota });
    } else {
      errors.push(`第${i + 1}张($${dollars})生成成功但未返回 key`);
    }
  }

  return { success: keys.length > 0, generated: keys.length, keys, details, errors };
}
