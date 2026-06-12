/**
 * 萌次元商城 - 虚拟卡密批量上传脚本
 * 
 * 用法:
 *   node upload.mjs --item <item_id> --sku <sku_id> --file <cards.txt> [--remark <备注>] [--unique]
 *   node upload.mjs --list   # 列出所有商品和SKU
 * 
 * 卡密文件格式: 一行一个卡密
 */

import crypto from 'crypto';
import fs from 'fs';

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
  let encrypted = cipher.update(data, 'utf8', 'base64');
  encrypted += cipher.final('base64');
  return encrypted;
}

function decrypt(data, key16) {
  const keyBuf = Buffer.from(key16, 'utf8');
  const decipher = crypto.createDecipheriv('aes-128-cbc', keyBuf, keyBuf);
  let decrypted = decipher.update(data, 'base64', 'utf8');
  decrypted += decipher.final('utf8');
  return decrypted;
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
  str = str.slice(0, -1);
  str += '&key=' + secret;
  return crypto.createHash('md5').update(str).digest('hex');
}

async function post(path, data = {}) {
  const secret = crypto.createHash('md5').update(Date.now().toString()).digest('hex');
  const key16 = secret.substring(0, 16);
  const signature = generateSignature(data, secret);
  const body = encrypt(JSON.stringify(data), key16);

  const res = await fetch(BASE_URL + path, {
    method: 'POST',
    headers: {
      'Content-Type': 'text/plain',
      'Cookie': COOKIE,
      'Secret': secret,
      'Signature': signature,
    },
    body,
  });

  const resSecret = res.headers.get('Secret');
  const rawText = await res.text();

  try {
    if (resSecret) {
      const resKey16 = resSecret.substring(0, 16);
      return JSON.parse(decrypt(rawText, resKey16));
    }
    return JSON.parse(rawText);
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

  const res = await fetch(BASE_URL + '/admin', {
    method: 'POST',
    headers: { 'Content-Type': 'text/plain', 'Secret': secret, 'Signature': signature },
    body,
    redirect: 'manual',
  });

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
}

// ============ 商品/SKU 映射 ============
const ITEMS = {
  23: { name: 'Gemini', skus: { 46: 'Gemini青春版日卡', 45: 'Gemini标准版日卡', 54: 'Gemini周三百次卡', 55: 'Gemini周五百次卡', 56: 'Gemini周一千次卡' } },
  24: { name: 'Claude', skus: { 48: 'Claude日卡', 57: 'Claude周三百次卡', 58: 'Claude周五百次卡', 59: 'Claude周一千次卡' } },
  25: { name: 'Claude 每日特惠', skus: { 50: 'Claude 活动日卡' } },
  26: { name: 'Gemini 每日特惠', skus: { 51: 'Gemini 青春版日卡-活动', 52: 'Gemini 标准版日卡-活动' } },
  28: { name: 'FuFu 混合卡', skus: { 65: '混合卡 月一百次卡', 64: '混合卡 月一百五十次卡', 60: '混合卡 月三百次卡', 61: '混合卡 月五百次卡', 62: '混合卡 月一千次卡' } },
  29: { name: 'FuFu 混合特惠卡', skus: { 66: 'FuFu 55次混合特惠卡' } },
  30: { name: 'Token 站额度', skus: { 67: '五块钱余额', 68: '十块钱余额', 70: '五十块钱额度' } },
  31: { name: 'Gemini-Free计划', skus: { 69: 'Gemini-Free计划' } },
};

// ============ 命令行 ============
const args = process.argv.slice(2);

function getArg(name) {
  const idx = args.indexOf('--' + name);
  return idx >= 0 && idx + 1 < args.length ? args[idx + 1] : null;
}

function hasFlag(name) {
  return args.includes('--' + name);
}

async function listItems() {
  console.log('\n📦 商品和SKU列表:\n');
  for (const [itemId, item] of Object.entries(ITEMS)) {
    console.log(`  [商品 ${itemId}] ${item.name}`);
    for (const [skuId, skuName] of Object.entries(item.skus)) {
      console.log(`    └─ [SKU ${skuId}] ${skuName}`);
    }
    console.log();
  }
}

async function uploadCards() {
  const itemId = getArg('item');
  const skuId = getArg('sku');
  const filePath = getArg('file');
  const remark = getArg('remark') || '';
  const unique = hasFlag('unique') ? 1 : 0;

  if (!itemId || !skuId || !filePath) {
    console.error('用法: node upload.mjs --item <item_id> --sku <sku_id> --file <cards.txt> [--remark <备注>] [--unique]');
    console.error('     node upload.mjs --list');
    process.exit(1);
  }

  if (!fs.existsSync(filePath)) {
    console.error(`❌ 文件不存在: ${filePath}`);
    process.exit(1);
  }

  const cards = fs.readFileSync(filePath, 'utf8').split('\n').map(l => l.trim()).filter(Boolean);
  if (cards.length === 0) {
    console.error('❌ 卡密文件为空');
    process.exit(1);
  }

  const itemInfo = ITEMS[itemId];
  const skuName = itemInfo?.skus?.[skuId];
  console.log(`\n📤 准备上传卡密`);
  console.log(`   商品: ${itemInfo?.name || itemId}`);
  console.log(`   SKU:  ${skuName || skuId}`);
  console.log(`   数量: ${cards.length} 张`);
  console.log(`   去重: ${unique ? '是' : '否'}`);
  if (remark) console.log(`   备注: ${remark}`);
  console.log();

  // 每批最多 500 张，避免请求过大
  const BATCH = 500;
  let total = 0, success = 0, failed = 0;

  for (let i = 0; i < cards.length; i += BATCH) {
    const batch = cards.slice(i, i + BATCH);
    const cardText = batch.join('\n');

    console.log(`⏳ 上传第 ${i + 1}-${i + batch.length} 张...`);

    const res = await post('/plugin/virtual-card-ship/card/add', {
      item_id: parseInt(itemId),
      sku_id: parseInt(skuId),
      remark,
      unique,
      upload_type: 0,
      card: cardText,
    });

    if (res.code === 200) {
      console.log(`   ✅ 成功! ${res.msg || ''}`);
      success += batch.length;
    } else {
      console.log(`   ❌ 失败: ${res.msg || JSON.stringify(res)}`);
      failed += batch.length;
    }
    total += batch.length;
  }

  console.log(`\n📊 上传完成: 共 ${total} 张, 成功 ${success}, 失败 ${failed}`);
}

// ============ 入口 ============
console.log('🔑 登录中...');
if (!(await login())) process.exit(1);
console.log('✅ 登录成功\n');

if (hasFlag('list')) {
  await listItems();
} else {
  await uploadCards();
}
