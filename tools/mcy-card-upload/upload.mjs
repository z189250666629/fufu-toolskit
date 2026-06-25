/**
 * 萌次元商城 - 虚拟卡密批量上传脚本
 *
 * 用法:
 *   node upload.mjs --item <item_id> --sku <sku_id> --file <cards.txt> [--remark <备注>] [--unique] [--batch-size <N>]
 *   node upload.mjs --list   # 实时列出所有商品和SKU
 *
 * 卡密文件格式: 一行一个卡密
 */

import fs from 'fs';
import { login, post, fetchItems, fetchSkus } from './mcy-client.mjs';
import { batchRanges, parsePositiveIntOption } from './upload-core.mjs';

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
  console.log('\n📦 商品和SKU列表（实时）:\n');
  const items = await fetchItems();
  for (const item of items) {
    console.log(`  [商品 ${item.id}] ${item.name}`);
    const skus = await fetchSkus(item.id);
    for (const sku of skus) {
      console.log(`    └─ [SKU ${sku.id}] ${sku.name}`);
    }
    console.log();
  }
}

// 从实时数据解析商品 / SKU 名称（找不到则回退到 ID）
async function resolveNames(itemId, skuId) {
  try {
    const items = await fetchItems();
    const item = items.find(i => String(i.id) === String(itemId));
    if (!item) return { itemName: itemId, skuName: skuId };
    const skus = await fetchSkus(item.id);
    const sku = skus.find(s => String(s.id) === String(skuId));
    return { itemName: item.name || itemId, skuName: sku?.name || skuId };
  } catch {
    return { itemName: itemId, skuName: skuId };
  }
}

async function uploadCards() {
  const itemId = getArg('item');
  const skuId = getArg('sku');
  const filePath = getArg('file');
  const remark = getArg('remark') || '';
  const unique = hasFlag('unique') ? 1 : 0;
  let batchSize;
  try {
    batchSize = parsePositiveIntOption(args, 'batch-size');
  } catch (err) {
    console.error(`❌ ${err.message}`);
    process.exit(1);
  }

  if (!itemId || !skuId || !filePath) {
    console.error('用法: node upload.mjs --item <item_id> --sku <sku_id> --file <cards.txt> [--remark <备注>] [--unique] [--batch-size <N>]');
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

  const { itemName, skuName } = await resolveNames(itemId, skuId);
  console.log(`\n📤 准备上传卡密`);
  console.log(`   商品: ${itemName}`);
  console.log(`   SKU:  ${skuName}`);
  console.log(`   数量: ${cards.length} 张`);
  console.log(`   去重: ${unique ? '是' : '否'}`);
  console.log(`   上传: ${batchSize ? `每批 ${batchSize} 张` : '一次性提交'}`);
  if (remark) console.log(`   备注: ${remark}`);
  console.log();

  // MCY card/add 支持一次性提交多行卡密；默认一次提交，避免大量分批请求占用带宽。
  // 如遇到网关请求体限制，可手动传 --batch-size <N> 回退到分批。
  let total = 0, success = 0, failed = 0;
  const batchCount = batchSize ? Math.ceil(cards.length / batchSize) : 1;

  for (const [from, to] of batchRanges(cards.length, batchSize)) {
    const batch = batchSize ? cards.slice(from, to) : cards;
    const cardText = batch.join('\n');
    const start = from + 1;
    const end = to;

    if (batchCount === 1) {
      console.log(`⏳ 一次性上传 ${batch.length} 张...`);
    } else {
      console.log(`⏳ 上传第 ${start}-${end} 张...`);
    }

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

try {
  if (hasFlag('list')) {
    await listItems();
  } else {
    await uploadCards();
  }
} catch (err) {
  console.error('❌', err.message);
  process.exit(1);
}
