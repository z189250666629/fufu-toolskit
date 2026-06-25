/**
 * 萌次元商城（MCY / fufu 小店）加密 API 客户端 —— upload.mjs / fufu-shop.mjs 共用。
 *
 * 商城 API 约定（实测，详见 apps/fufu-act/shop_inventory.go）：
 *  - 请求体 AES-128-CBC 加密 + MD5 Secret/Signature 头；登录 POST /admin → manage_token cookie。
 *  - card/get 的精准过滤只认 `equal-<字段>` 前缀；裸 item_id / sku_id / status 会被静默忽略，
 *    返回的是全局总数（约 21k）。
 *  - 同一会话禁止并发：并发请求会收到 body 级「登录已过期」(HTTP 200 + code!=200)。调用方必须
 *    串行；post() 会在检测到会话过期时自动重登一次并重试。
 *  - base_url 必须是 https://host（http 会 301 跳转到 https，加密 POST 无法存活）。
 *
 * 零第三方依赖，仅用内置 crypto。
 */

import crypto from 'crypto';

// ============ 配置 ============
export const BASE_URL = (process.env.MCY_BASE_URL || 'https://shop.fufuflower.top').replace(/\/+$/, '');

// 凭据从环境变量读取（与仓库其余部分保持一致：MCY_USERNAME / MCY_PASSWORD）
const MCY_EMAIL = process.env.MCY_USERNAME || process.env.MCY_EMAIL;
const MCY_PASSWORD = process.env.MCY_PASSWORD;
if (!MCY_EMAIL || !MCY_PASSWORD) {
  console.error('❌ 缺少凭据：请设置环境变量 MCY_USERNAME 和 MCY_PASSWORD');
  console.error("   PowerShell 示例: $env:MCY_USERNAME='you@example.com'; $env:MCY_PASSWORD='***'");
  process.exit(1);
}

let COOKIE = '';

// ============ 加密 / 签名 ============
function encrypt(data, key16) {
  const keyBuf = Buffer.from(key16, 'utf8');
  const cipher = crypto.createCipheriv('aes-128-cbc', keyBuf, keyBuf);
  return cipher.update(data, 'utf8', 'base64') + cipher.final('base64');
}

function decrypt(data, key16) {
  const keyBuf = Buffer.from(key16, 'utf8');
  const decipher = crypto.createDecipheriv('aes-128-cbc', keyBuf, keyBuf);
  return decipher.update(data, 'base64', 'utf8') + decipher.final('utf8');
}

function generateSignature(data, secret) {
  const obj = { ...data };
  delete obj.sign;
  const keys = Object.keys(obj).sort();
  let str = '';
  for (const k of keys) {
    const v = obj[k];
    // 只跳过 '' / undefined / object / NaN —— 数字 0（如 equal-status:0）必须进签名串，
    // 否则该过滤会从签名里掉出去，商城静默回退到全局总数。
    if (v === '' || v === undefined || typeof v === 'object' || Number.isNaN(v)) continue;
    str += k + '=' + v + '&';
  }
  str = str.slice(0, -1) + '&key=' + secret;
  return crypto.createHash('md5').update(str).digest('hex');
}

function parseResponse(rawText, resSecret) {
  try {
    return resSecret ? JSON.parse(decrypt(rawText, resSecret.substring(0, 16))) : JSON.parse(rawText);
  } catch {
    return { code: -1, msg: 'Decrypt failed', raw: rawText.substring(0, 200) };
  }
}

// 会话过期标记（与 apps/fufu-act/shop_inventory.go 的 mcyIsSessionExpired 保持一致）
const SESSION_EXPIRED_MARKERS = ['登录已过期', '登录失效', '未登录', '请登录', '请先登录'];
function isSessionExpired(result) {
  if (!result || result.code === 200) return false;
  const msg = String(result.msg ?? result.message ?? '');
  return SESSION_EXPIRED_MARKERS.some(m => msg.includes(m));
}

// ============ 登录 ============
export async function login() {
  const secret = crypto.createHash('md5').update(Date.now().toString()).digest('hex');
  const key16 = secret.substring(0, 16);
  const data = { email: MCY_EMAIL, password: MCY_PASSWORD };
  const signature = generateSignature(data, secret);
  const body = encrypt(JSON.stringify(data), key16);

  // 网络 / 解析异常都收敛为 return false，避免无 try/catch 的顶层 await login() 抛出未捕获的 rejection。
  try {
    // 登录接口是 POST /admin
    const res = await fetch(BASE_URL + '/admin', {
      method: 'POST',
      headers: { 'Content-Type': 'text/plain', 'Secret': secret, 'Signature': signature },
      body,
      redirect: 'manual',
    });

    // 收集 set-cookie
    const setCookies = res.headers.getSetCookie?.() || [];
    const cookieParts = setCookies.map(c => c.split(';')[0]);
    const result = parseResponse(await res.text(), res.headers.get('Secret'));

    // token 优先取响应体，其次看 set-cookie（有些版本 token 直接在 set-cookie 里）
    if (result.code === 200 && result.data?.token) {
      cookieParts.push('manage_token=' + encodeURIComponent(result.data.token));
      COOKIE = cookieParts.join('; ');
      return true;
    }
    if (cookieParts.some(c => c.startsWith('manage_token='))) {
      COOKIE = cookieParts.join('; ');
      return true;
    }

    console.error('❌ 登录失败:', result.msg || JSON.stringify(result));
    return false;
  } catch (err) {
    console.error('❌ 登录请求失败:', err.message);
    return false;
  }
}

// ============ 请求 ============
// 注意：商城禁止并发，调用方必须串行。会话过期时自动重登一次并重试。
export async function post(path, data = {}, { retryOnExpiry = true } = {}) {
  const secret = crypto.createHash('md5').update(Date.now().toString()).digest('hex');
  const key16 = secret.substring(0, 16);
  const signature = generateSignature(data, secret);
  const body = encrypt(JSON.stringify(data), key16);

  const res = await fetch(BASE_URL + path, {
    method: 'POST',
    headers: { 'Content-Type': 'text/plain', 'Cookie': COOKIE, 'Secret': secret, 'Signature': signature },
    body,
  });

  const result = parseResponse(await res.text(), res.headers.get('Secret'));

  if (retryOnExpiry && isSessionExpired(result)) {
    if (await login()) return post(path, data, { retryOnExpiry: false });
  }
  return result;
}

// ============ 商品 / SKU（实时拉取，避免硬编码漂移） ============
export async function fetchItems({ page = 1, limit = 50, displayScope = 'all' } = {}) {
  const res = await post('/admin/repertory/item/get', { page, limit, display_scope: displayScope });
  if (res.code !== 200) throw new Error(`获取商品失败: ${res.msg || JSON.stringify(res)}`);
  return res.data?.list || [];
}

export async function fetchSkus(itemId) {
  // id 与 type 是 query string 参数
  const res = await post(`/admin/repertory/item/sku/get?id=${itemId}&type=edit`, { page: 1, limit: 50 });
  if (res.code !== 200) throw new Error(`获取 SKU 失败 (item ${itemId}): ${res.msg || JSON.stringify(res)}`);
  return res.data?.list || res.data || [];
}
