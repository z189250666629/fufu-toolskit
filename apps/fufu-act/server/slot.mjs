import crypto from 'crypto';

// 卡密额度 → 抽奖次数映射
export const SPIN_MAP = {
  0.1: 100,
  100: 1,
  150: 1,
  300: 3,
  500: 4,
  1000: 10,
};

// 奖池：type 决定结果种类
//   'win'   → 中奖，记录、扣次数、发奖
//   'miss'  → 不中奖，扣次数但不发奖
//   'retry' → 再来一次，不扣次数
// 权重总和 = 10000（万分比）

// 通用池（测试卡、$150 等）
export const PRIZE_POOL = [
  { type: 'miss',  dollars: 0,    weight: 500 },    // 5%
  { type: 'retry', dollars: 0,    weight: 500 },    // 5%
  { type: 'win',   dollars: 1,    weight: 1500 },   // 15%
  { type: 'win',   dollars: 5,    weight: 3000 },   // 30%
  { type: 'win',   dollars: 10,   weight: 2000 },   // 20%
  { type: 'win',   dollars: 20,   weight: 1200 },   // 12%
  { type: 'win',   dollars: 50,   weight: 580 },    // 5.8%
  { type: 'win',   dollars: 100,  weight: 380 },    // 3.8%
  { type: 'win',   dollars: 200,  weight: 200 },    // 2%
  { type: 'win',   dollars: 500,  weight: 100 },    // 1%
  { type: 'win',   dollars: 1000, weight: 40 },     // 0.4%
];

// 按额度分级的奖池：≥自身额度的奖品降权
const TIER_POOLS = {
  // $100卡(1次)：$100+ 大幅降低
  100: [
    { type: 'miss',  dollars: 0,    weight: 500 },
    { type: 'retry', dollars: 0,    weight: 500 },
    { type: 'win',   dollars: 1,    weight: 1800 },
    { type: 'win',   dollars: 5,    weight: 3500 },
    { type: 'win',   dollars: 10,   weight: 2300 },
    { type: 'win',   dollars: 20,   weight: 1000 },
    { type: 'win',   dollars: 50,   weight: 250 },
    { type: 'win',   dollars: 100,  weight: 100 },
    { type: 'win',   dollars: 200,  weight: 30 },
    { type: 'win',   dollars: 500,  weight: 15 },
    { type: 'win',   dollars: 1000, weight: 5 },
  ],
  // $150卡(1次)：比$100池稍好，使回报率对齐
  150: [
    { type: 'miss',  dollars: 0,    weight: 500 },
    { type: 'retry', dollars: 0,    weight: 500 },
    { type: 'win',   dollars: 1,    weight: 1200 },
    { type: 'win',   dollars: 5,    weight: 3000 },
    { type: 'win',   dollars: 10,   weight: 2500 },
    { type: 'win',   dollars: 20,   weight: 1500 },
    { type: 'win',   dollars: 50,   weight: 500 },
    { type: 'win',   dollars: 100,  weight: 180 },
    { type: 'win',   dollars: 200,  weight: 70 },
    { type: 'win',   dollars: 500,  weight: 35 },
    { type: 'win',   dollars: 1000, weight: 15 },
  ],
  // $300卡(3次)：$50+ 降低
  300: [
    { type: 'miss',  dollars: 0,    weight: 500 },
    { type: 'retry', dollars: 0,    weight: 500 },
    { type: 'win',   dollars: 1,    weight: 2200 },
    { type: 'win',   dollars: 5,    weight: 3300 },
    { type: 'win',   dollars: 10,   weight: 2000 },
    { type: 'win',   dollars: 20,   weight: 1000 },
    { type: 'win',   dollars: 50,   weight: 300 },
    { type: 'win',   dollars: 100,  weight: 120 },
    { type: 'win',   dollars: 200,  weight: 50 },
    { type: 'win',   dollars: 500,  weight: 20 },
    { type: 'win',   dollars: 1000, weight: 10 },
  ],
  // $500卡(4次)：$500+ 降低
  500: [
    { type: 'miss',  dollars: 0,    weight: 500 },
    { type: 'retry', dollars: 0,    weight: 500 },
    { type: 'win',   dollars: 1,    weight: 1500 },
    { type: 'win',   dollars: 5,    weight: 3100 },
    { type: 'win',   dollars: 10,   weight: 2100 },
    { type: 'win',   dollars: 20,   weight: 1200 },
    { type: 'win',   dollars: 50,   weight: 580 },
    { type: 'win',   dollars: 100,  weight: 300 },
    { type: 'win',   dollars: 200,  weight: 150 },
    { type: 'win',   dollars: 500,  weight: 40 },    // 1% → 0.4%
    { type: 'win',   dollars: 1000, weight: 30 },    // 0.4% → 0.3%
  ],
  // $1000卡(10次)：$1000 降低
  1000: [
    { type: 'miss',  dollars: 0,    weight: 500 },
    { type: 'retry', dollars: 0,    weight: 500 },
    { type: 'win',   dollars: 1,    weight: 1500 },
    { type: 'win',   dollars: 5,    weight: 3000 },
    { type: 'win',   dollars: 10,   weight: 2000 },
    { type: 'win',   dollars: 20,   weight: 1200 },
    { type: 'win',   dollars: 50,   weight: 580 },
    { type: 'win',   dollars: 100,  weight: 380 },
    { type: 'win',   dollars: 200,  weight: 200 },
    { type: 'win',   dollars: 500,  weight: 120 },
    { type: 'win',   dollars: 1000, weight: 20 },    // 0.4% → 0.2%
  ],
};

export const SCRATCH_REWARDS = [2, 4, 6, 8, 12, 15];
export const SCRATCH_MINES = 2;
export const SCRATCH_MAX_REVEALS = 6;

const TOTAL_WEIGHT = PRIZE_POOL.reduce((s, p) => s + p.weight, 0);
if (TOTAL_WEIGHT !== 10000) {
  PRIZE_POOL[0].weight += (10000 - TOTAL_WEIGHT);
}
const FINAL_WEIGHT = PRIZE_POOL.reduce((s, p) => s + p.weight, 0);

// 预计算各分级池的总权重
const TIER_WEIGHTS = {};
for (const [tier, pool] of Object.entries(TIER_POOLS)) {
  const tw = pool.reduce((s, p) => s + p.weight, 0);
  if (tw !== 10000) pool[0].weight += (10000 - tw);
  TIER_WEIGHTS[tier] = pool.reduce((s, p) => s + p.weight, 0);
}

export function secureRandomInt(max) {
  const buf = crypto.randomBytes(4);
  return buf.readUInt32BE(0) % max;
}

// 中了 $1000 头奖后，剩余次数用这个概率池（小奖为主）
const POST_JACKPOT_POOL = [
  { type: 'miss',  dollars: 0,    weight: 500 },    // 5%
  { type: 'retry', dollars: 0,    weight: 500 },    // 5%
  { type: 'win',   dollars: 1,    weight: 3000 },   // 30%
  { type: 'win',   dollars: 5,    weight: 3500 },   // 35%
  { type: 'win',   dollars: 10,   weight: 1700 },   // 17%
  { type: 'win',   dollars: 20,   weight: 800 },    // 8%
];
const POST_JACKPOT_WEIGHT = POST_JACKPOT_POOL.reduce((s, p) => s + p.weight, 0);

/**
 * 从指定奖池抽奖
 */
function rollFromPool(pool, totalWeight) {
  const roll = secureRandomInt(totalWeight);
  let cumulative = 0;
  for (const p of pool) {
    cumulative += p.weight;
    if (roll < cumulative) return p;
  }
  return pool[0];
}

/**
 * 服务端抽奖
 * @param {number} cardDollars - 卡密额度
 * @param {boolean} hasWonJackpot - 是否已中过 $1000
 * @param {object} opts - 额外选项
 * @param {number} opts.usedSpins - 已用次数
 * @param {number} opts.totalSpins - 总次数
 * @param {number} opts.maxWon - 历史单次最高中奖额
 * @returns {{ dollars: number, isRetry: boolean, isMiss: boolean }}
 */
export function spin(cardDollars, hasWonJackpot, opts = {}) {
  const { usedSpins = 0, totalSpins = 0, maxWon = 0, forcePrize } = opts;
  const remaining = totalSpins - usedSpins;
  const isLastSpin = remaining === 1;

  // === 预设中奖 ===
  if (forcePrize != null && forcePrize > 0) {
    return { dollars: forcePrize, isRetry: false, isMiss: false };
  }

  // === 保底机制 ===
  // $1000卡：前7次没中过 ≥$50，第8次（最后一次）保底 $100
  if (cardDollars === 1000 && isLastSpin && maxWon < 50) {
    return { dollars: 100, isRetry: false, isMiss: false };
  }
  // $500卡：前3次没中过 ≥$50，第4次（最后一次）保底 $20
  if (cardDollars === 500 && isLastSpin && maxWon < 50) {
    return { dollars: 20, isRetry: false, isMiss: false };
  }

  // === 中了头奖后，剩余次数用小奖池 ===
  if (hasWonJackpot) {
    const prize = rollFromPool(POST_JACKPOT_POOL, POST_JACKPOT_WEIGHT);
    if (prize.type === 'retry') return { dollars: 0, isRetry: true, isMiss: false };
    if (prize.type === 'miss') return { dollars: 0, isRetry: false, isMiss: true };
    return { dollars: prize.dollars, isRetry: false, isMiss: false };
  }

  // === 单次中奖超过卡面额度一半，后续走小奖池 ===
  if (cardDollars >= 100 && maxWon >= cardDollars * 0.5) {
    const prize = rollFromPool(POST_JACKPOT_POOL, POST_JACKPOT_WEIGHT);
    if (prize.type === 'retry') return { dollars: 0, isRetry: true, isMiss: false };
    if (prize.type === 'miss') return { dollars: 0, isRetry: false, isMiss: true };
    return { dollars: prize.dollars, isRetry: false, isMiss: false };
  }

  // === 正常抽奖（按额度选池） ===
  const pool = TIER_POOLS[cardDollars] || PRIZE_POOL;
  const poolWeight = TIER_WEIGHTS[cardDollars] || FINAL_WEIGHT;
  const prize = rollFromPool(pool, poolWeight);

  if (prize.type === 'retry') {
    return { dollars: 0, isRetry: true, isMiss: false };
  }
  if (prize.type === 'miss') {
    return { dollars: 0, isRetry: false, isMiss: true };
  }

  // $1000 头奖：仅 $1000 卡和测试卡可中，且每张卡限一次；否则降级为"再来一次"
  if (prize.dollars === 1000) {
    if ((cardDollars !== 1000 && cardDollars !== 0.1) || hasWonJackpot) {
      return { dollars: 0, isRetry: true, isMiss: false };
    }
  }

  return { dollars: prize.dollars, isRetry: false, isMiss: false };
}
