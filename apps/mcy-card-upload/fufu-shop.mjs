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

import { login, post, fetchItems, fetchSkus } from './mcy-client.mjs';

const CARD_GET = '/plugin/virtual-card-ship/card/get';

function fmtPrice(v) {
  const n = parseFloat(v);
  return Number.isFinite(n) ? n.toFixed(2) : '?';
}

// ============ 查询功能 ============
async function getOverview() {
  console.log('\n🏪 fufu小店 信息总览\n');

  const items = await fetchItems();
  console.log(`📦 商品总数: ${items.length}\n`);

  for (const item of items) {
    const statusText = item.status === 1 ? '✅上架' : item.status === 2 ? '⬇️下架' : `状态${item.status}`;
    console.log(`  [${item.id}] ${item.name}  (${statusText} | ${item.plugin})`);

    const skus = await fetchSkus(item.id);
    for (const sku of skus) {
      const stock = sku.stock ?? sku.card_count ?? '?';
      console.log(`    └─ [SKU ${sku.id}] ${sku.name}  ¥${fmtPrice(sku.stock_price)}  库存:${stock}`);
    }
    console.log();
  }

  // 卡密总览
  const cardRes = await post(CARD_GET, { page: 1, limit: 1 });
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
  const data = { page: parseInt(opts.page) || 1, limit: parseInt(opts.limit) || 20 };
  // 精准过滤必须带 equal- 前缀；裸 item_id / sku_id / status 会被商城静默忽略并返回全局列表。
  if (opts.item != null) data['equal-item_id'] = parseInt(opts.item);
  if (opts.sku != null) data['equal-sku_id'] = parseInt(opts.sku);
  if (opts.status != null) data['equal-status'] = parseInt(opts.status);

  const res = await post(CARD_GET, data);
  if (res.code !== 200) { console.error('❌', res.msg); return; }

  console.log(`\n🎴 卡密列表 (${res.data?.total ?? 0} 条, 第${data.page}页)\n`);
  console.log(`   总数:${res.card_count} | 已用:${res.card_used_count} | 可用:${res.card_usable_count} | 冻结:${res.card_frozen_count}\n`);

  for (const c of res.data?.list || []) {
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

  const items = await fetchItems();
  let totalUsable = 0;

  for (const item of items) {
    if (item.plugin !== 'VirtualCardShip') continue;

    const skus = await fetchSkus(item.id);
    console.log(`  📦 ${item.name}`);
    for (const sku of skus) {
      // 精准查询该 SKU 可用卡密数：equal-* 过滤下 data.total 即该 SKU 的精确可用数。
      const cardRes = await post(CARD_GET, {
        'equal-item_id': item.id,
        'equal-sku_id': sku.id,
        'equal-status': 0, // 0 = 未使用
        page: 1,
        limit: 1,
      });
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

try {
  if (hasFlag('cards')) {
    await getCards({ item: getArg('item'), sku: getArg('sku'), status: getArg('status'), page: getArg('page'), limit: getArg('limit') });
  } else if (hasFlag('stock')) {
    await getStock();
  } else {
    await getOverview();
  }
} catch (err) {
  console.error('❌', err.message);
  process.exit(1);
}
