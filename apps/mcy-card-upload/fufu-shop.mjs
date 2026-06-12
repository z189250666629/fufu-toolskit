/**
 * fufu-shop 信息查询脚本
 * 
 * 用法:
 *   node fufu-shop.mjs                  # 总览：商品列表 + 库存统计
 *   node fufu-shop.mjs --cards [选项]    # 查询卡密列表
 *     --item <id>                        # 按商品筛选
 *     --sku <id>                         # 按SKU筛选
 *     --status <0|1>                     # 0=未使用 1=已使用
 *     --page <n>                         # 页码 (默认1)
 *     --limit <n>                        # 每页数量 (默认20)
 *   node fufu-shop.mjs --stock           # 各SKU可用库存汇总
 */

import crypto from 'crypto';

// ============ 配置 ============
const BASE_URL = process.env.MCY_BASE_URL || 'https://shop.fufuflower.top';
let COOKIE = '';

// 凭据从环境变量读取（与仓库其余部分保持一致：MCY_USERNAME / MCY_PASSWORD）
const MCY_EMAIL = process.env.MCY_USERNAME || process.env.MCY_EMAIL;
const MCY_PASSWORD = process.env.MCY_PASSWORD;
if (!MCY_EMAIL || !MCY_PASSWORD) {
  console.error('❌ 缺少凭据：请设置环境变量 MCY_USERNAME 和 MCY_PASSWORD');
  console.error("   PowerShell 示例: $env:MCY_USERNAME='you@example.com'; $env:MCY_PASSWORD='***'");
  process.exit(1);
}

// ============ 加密工具 ============
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
    if (v === '' || v === undefined || typeof v === 'object' || Number.isNaN(v)) continue;
    str += k + '=' + v + '&';
  }
  str = str.slice(0, -1) + '&key=' + secret;
  return crypto.createHash('md5').update(str).digest('hex');
}

async function post(path, data = {}) {
  const secret = crypto.createHash('md5').update(Date.now().toString()).digest('hex');
  const key16 = secret.substring(0, 16);
  const signature = generateSignature(data, secret);
  const body = encrypt(JSON.stringify(data), key16);

  const res = await fetch(BASE_URL + path, {
    method: 'POST',
    headers: { 'Content-Type': 'text/plain', 'Cookie': COOKIE, 'Secret': secret, 'Signature': signature },
    body,
  });

  const resSecret = res.headers.get('Secret');
  const rawText = await res.text();
  try {
    return resSecret ? JSON.parse(decrypt(rawText, resSecret.substring(0, 16))) : JSON.parse(rawText);
  } catch {
    return { code: -1, msg: 'Decrypt failed', raw: rawText.substring(0, 200) };
  }
}

// ============ 登录 ============
async function login() {
  const secret = crypto.createHash('md5').update(Date.now().toString()).digest('hex');
  const key16 = secret.substring(0, 16);
  const data = { email: MCY_EMAIL, password: MCY_PASSWORD };
  const signature = generateSignature(data, secret);
  const body = encrypt(JSON.stringify(data), key16);

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

  const resSecret = res.headers.get('Secret');
  const rawText = await res.text();
  let result;
  try {
    result = resSecret ? JSON.parse(decrypt(rawText, resSecret.substring(0, 16))) : JSON.parse(rawText);
  } catch {
    result = { code: -1 };
  }

  // token 在响应体里，拼到 cookie 中
  if (result.code === 200 && result.data?.token) {
    cookieParts.push('manage_token=' + encodeURIComponent(result.data.token));
    COOKIE = cookieParts.join('; ');
    return true;
  }

  // 有些版本 token 直接在 set-cookie 里
  if (cookieParts.some(c => c.startsWith('manage_token='))) {
    COOKIE = cookieParts.join('; ');
    return true;
  }

  console.error('❌ 登录失败:', result.msg || JSON.stringify(result));
  return false;
}

// ============ 查询功能 ============
async function getOverview() {
  console.log('\n🏪 fufu小店 信息总览\n');

  // 获取商品列表
  const itemRes = await post('/admin/repertory/item/get', { page: 1, limit: 50, display_scope: 'all' });
  if (itemRes.code !== 200) { console.error('❌ 获取商品失败:', itemRes.msg); return; }

  const items = itemRes.data.list || [];
  console.log(`📦 商品总数: ${items.length}\n`);

  for (const item of items) {
    const statusText = item.status === 1 ? '✅上架' : item.status === 2 ? '⬇️下架' : `状态${item.status}`;
    console.log(`  [${item.id}] ${item.name}  (${statusText} | ${item.plugin})`);

    // 获取 SKU (id 和 type 是 query string 参数)
    const skuRes = await post(`/admin/repertory/item/sku/get?id=${item.id}&type=edit`, { page: 1, limit: 50 });
    const skus = skuRes.data?.list || skuRes.data || [];
    for (const sku of skus) {
      const stock = sku.stock ?? sku.card_count ?? '?';
      console.log(`    └─ [SKU ${sku.id}] ${sku.name}  ¥${parseFloat(sku.stock_price).toFixed(2)}  库存:${stock}`);
    }
    console.log();
  }

  // 卡密总览
  const cardRes = await post('/plugin/virtual-card-ship/card/get', { page: 1, limit: 1 });
  if (cardRes.code === 200) {
    console.log('🎴 卡密统计:');
    console.log(`   总数: ${cardRes.card_count}`);
    console.log(`   已用: ${cardRes.card_used_count}`);
    console.log(`   可用: ${cardRes.card_usable_count}`);
    console.log(`   冻结: ${cardRes.card_frozen_count}`);
  }
  console.log();
}

async function getCards(opts) {
  const data = { page: opts.page || 1, limit: opts.limit || 20 };
  if (opts.item) data.item_id = parseInt(opts.item);
  if (opts.sku) data.sku_id = parseInt(opts.sku);
  if (opts.status !== undefined) data.status = parseInt(opts.status);

  const res = await post('/plugin/virtual-card-ship/card/get', data);
  if (res.code !== 200) { console.error('❌', res.msg); return; }

  console.log(`\n🎴 卡密列表 (${res.data.total} 条, 第${data.page}页)\n`);
  console.log(`   总数:${res.card_count} | 已用:${res.card_used_count} | 可用:${res.card_usable_count} | 冻结:${res.card_frozen_count}\n`);

  for (const c of res.data.list) {
    const st = c.status === 0 ? '🟢可用' : c.status === 1 ? '🔴已用' : `状态${c.status}`;
    console.log(`  [${c.id}] ${st} | ${c.item?.name} / ${c.sku?.name}`);
    console.log(`         ${c.card}`);
    if (c.remark) console.log(`         备注: ${c.remark}`);
    console.log(`         创建: ${c.create_time}${c.purchase_time ? ' | 售出: ' + c.purchase_time : ''}`);
    console.log();
  }
}

async function getStock() {
  console.log('\n📊 各SKU可用库存汇总\n');

  const itemRes = await post('/admin/repertory/item/get', { page: 1, limit: 50, display_scope: 'all' });
  if (itemRes.code !== 200) { console.error('❌', itemRes.msg); return; }

  const items = itemRes.data.list || [];
  let totalUsable = 0;

  for (const item of items) {
    if (item.plugin !== 'VirtualCardShip') continue;

    const skuRes = await post(`/admin/repertory/item/sku/get?id=${item.id}&type=edit`, { page: 1, limit: 50 });
    const skus = skuRes.data?.list || skuRes.data || [];

    console.log(`  📦 ${item.name}`);
    for (const sku of skus) {
      // 查询该 SKU 可用卡密数
      const cardRes = await post('/plugin/virtual-card-ship/card/get', { item_id: item.id, sku_id: sku.id, status: 0, page: 1, limit: 1 });
      const usable = cardRes.data?.total ?? '?';
      totalUsable += typeof usable === 'number' ? usable : 0;
      console.log(`    └─ [${sku.id}] ${sku.name}: ${usable} 张可用`);
    }
    console.log();
  }

  console.log(`  🎯 总可用库存: ${totalUsable} 张\n`);
}

// ============ 入口 ============
const args = process.argv.slice(2);
function getArg(name) {
  const idx = args.indexOf('--' + name);
  return idx >= 0 && idx + 1 < args.length ? args[idx + 1] : null;
}
function hasFlag(name) { return args.includes('--' + name); }

console.log('🔑 登录中...');
if (!(await login())) process.exit(1);
console.log('✅ 登录成功');

if (hasFlag('cards')) {
  await getCards({ item: getArg('item'), sku: getArg('sku'), status: getArg('status'), page: getArg('page'), limit: getArg('limit') });
} else if (hasFlag('stock')) {
  await getStock();
} else {
  await getOverview();
}
