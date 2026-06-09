import { Router } from 'express';
import { spin, SPIN_MAP, PRIZE_POOL, SCRATCH_REWARDS, SCRATCH_MINES, SCRATCH_MAX_REVEALS, secureRandomInt } from './slot.mjs';
import { fufuApi, CARD_TYPES, mcyLogin, mcyPost } from '../scripts/api-act.mjs';

// 活动开始时间（北京时间），在此之后售出的 shop 卡可参与
const ACT_START = '2026-05-01 00:00:00';
const ACT_END = '2026-05-08 23:59:59';
const ACT_START_TS = 1777564800;
const ACT_END_TS = 1778255999;

// 从 interval_quota 反推额度（美元）
function dollarsTier(intervalQuota) {
  return Math.round(intervalQuota / 500000) || null;
}

// 在 MCY 商城查找卡密的售出记录（分页遍历最近售出的卡）
async function findShopPurchase(cardKey) {
  try {
    await mcyLogin();
  } catch { return null; }
  const res = await mcyPost('/plugin/virtual-card-ship/card/get', {
    'equal-card': cardKey,
    page: 1,
    limit: 1,
  });
  const list = res.data?.list || [];
  return list[0] || null;
}

// per-card 互斥锁，防止同一张卡并发 spin 超发
const cardLocks = new Map();
function withCardLock(cardKey, fn) {
  const prev = cardLocks.get(cardKey) || Promise.resolve();
  const next = prev.then(fn, fn);
  cardLocks.set(cardKey, next.then(() => {
    if (cardLocks.get(cardKey) === next) cardLocks.delete(cardKey);
  }, () => {
    if (cardLocks.get(cardKey) === next) cardLocks.delete(cardKey);
  }));
  return next;
}

export function createRouter(db) {
  const router = Router();

  // ============================================================
  // POST /api/login — 用 act 卡密登录
  // ============================================================
  router.post('/login', async (req, res) => {
    const { cardKey } = req.body;
    if (!cardKey) return res.status(400).json({ error: '请输入卡密' });

    const key = cardKey.trim();

    let card = db.prepare('SELECT * FROM cards WHERE card_key = ?').get(key);

    if (!card) {
      const keyword = key.replace(/^sk-/, '');
      let token = null;

      const search = await fufuApi('GET', `/api/token/search?keyword=&token=${keyword}&p=0&size=10`);
      if (search.data?.length) {
        token = search.data.find(t => t.key === keyword);
      }

      if (!token) return res.status(404).json({ error: '卡密不存在' });

      const isActTest = token.name && token.name.includes('-act-') && token.name.includes('test');
      let dollars = null;
      let source = 'shop';
      let purchaseTime = null;
      let createdInRange = false;

      if (isActTest) {
        dollars = parseFloat(token.name.split('-act-')[0]);
        source = 'act';
        createdInRange = true;
      } else {
        createdInRange = !!(token.created_time
          && token.created_time >= ACT_START_TS && token.created_time <= ACT_END_TS);

        const shopCard = await findShopPurchase(key);
        const purchasedInRange = shopCard?.purchase_time
          && shopCard.purchase_time >= ACT_START && shopCard.purchase_time <= ACT_END;

        if (!createdInRange && !purchasedInRange) {
          return res.status(403).json({ error: '此卡密不在活动期间内，不参与活动' });
        }

        dollars = dollarsTier(token.interval_quota);
        purchaseTime = shopCard?.purchase_time || null;
      }

      const isScratchCard = dollars === 55 && (purchaseTime || createdInRange);
      if (!dollars || (!SPIN_MAP[dollars] && !isScratchCard)) {
        return res.status(403).json({ error: '此卡密额度不参与活动' });
      }

      const totalSpins = SPIN_MAP[dollars] || 0;
      db.prepare(`
        INSERT INTO cards (card_key, card_name, dollars, total_spins, source, purchase_time)
        VALUES (?, ?, ?, ?, ?, ?)
      `).run(key, token.name, dollars, totalSpins, source, purchaseTime);

      card = db.prepare('SELECT * FROM cards WHERE card_key = ?').get(key);
    }

    const history = db.prepare(
      'SELECT prize_dollars, created_at FROM spin_log WHERE card_key = ? AND is_retry = 0 AND prize_dollars > 0 ORDER BY id DESC'
    ).all(key);

    const isScratch = card.dollars === 55;

    let scratchGame = null;
    if (isScratch) {
      const sg = db.prepare('SELECT * FROM scratch_games WHERE card_key = ?').get(key);
      if (sg) {
        const gameOver = sg.status === 'won' || sg.status === 'lost' || sg.status === 'cashout';
        scratchGame = {
          revealed: JSON.parse(sg.revealed),
          prize: sg.prize_dollars,
          status: sg.status,
          mines: gameOver ? JSON.parse(sg.mine_pos) : undefined,
        };
      }
    }

    res.json({
      cardKey: key,
      cardName: card.card_name,
      dollars: card.dollars,
      totalSpins: card.total_spins,
      usedSpins: card.used_spins,
      remainingSpins: card.total_spins - card.used_spins,
      totalWon: card.total_won,
      wonJackpot: !!card.won_jackpot,
      history,
      isScratch,
      scratchGame,
    });
  });

  // ============================================================
  // POST /api/spin — 抽奖（per-card 锁保护）
  // ============================================================
  router.post('/spin', async (req, res) => {
    const { cardKey } = req.body;
    if (!cardKey) return res.status(400).json({ error: '请输入卡密' });

    const key = cardKey.trim();

    try {
      const result = await withCardLock(key, async () => {
        const card = db.prepare('SELECT * FROM cards WHERE card_key = ?').get(key);
        if (!card) throw { status: 404, error: '请先登录' };

        const remaining = card.total_spins - card.used_spins;
        if (remaining <= 0) throw { status: 403, error: '抽奖次数已用完' };

        const maxWonRow = db.prepare(
          'SELECT MAX(prize_dollars) as max_prize FROM spin_log WHERE card_key = ? AND is_retry = 0'
        ).get(key);
        const maxWon = maxWonRow?.max_prize || 0;

        // rigged: JSON object mapping spin number (1-based) to forced prize
        let forcePrize;
        if (card.rigged) {
          const riggedMap = JSON.parse(card.rigged);
          const nextSpin = card.used_spins + 1;
          forcePrize = riggedMap[nextSpin];
        }

        const spinResult = spin(card.dollars, !!card.won_jackpot, {
          usedSpins: card.used_spins,
          totalSpins: card.total_spins,
          maxWon,
          forcePrize,
        });

        if (spinResult.isRetry) {
          db.prepare(
            'INSERT INTO spin_log (card_key, prize_dollars, is_retry) VALUES (?, 0, 1)'
          ).run(key);
          return { isRetry: true, isMiss: false, message: '再来一次！', remainingSpins: remaining };
        }

        if (spinResult.isMiss) {
          db.transaction(() => {
            db.prepare("UPDATE cards SET used_spins = used_spins + 1, last_spin_at = datetime('now') WHERE card_key = ?").run(key);
            db.prepare('INSERT INTO spin_log (card_key, prize_dollars, is_retry) VALUES (?, 0, 0)').run(key);
          })();
          const updatedCard = db.prepare('SELECT * FROM cards WHERE card_key = ?').get(key);
          const newRemaining = updatedCard.total_spins - updatedCard.used_spins;

          // 次数用完，把 total_won 一次性入队充值
          if (newRemaining <= 0 && updatedCard.total_won > 0) {
            const existing = db.prepare(
              "SELECT id FROM credit_queue WHERE card_key = ? AND status IN ('pending', 'done')"
            ).get(key);
            if (!existing) {
              db.prepare('INSERT INTO credit_queue (card_key, prize_dollars) VALUES (?, ?)')
                .run(key, updatedCard.total_won);
            }
          }

          return {
            isRetry: false, isMiss: true, prize: 0,
            remainingSpins: newRemaining,
            totalWon: updatedCard.total_won,
          };
        }

        // 中奖
        const isJackpot = spinResult.dollars === 1000 ? 1 : 0;
        db.transaction(() => {
          db.prepare("UPDATE cards SET used_spins = used_spins + 1, won_jackpot = won_jackpot + ?, total_won = total_won + ?, last_spin_at = datetime('now') WHERE card_key = ?")
            .run(isJackpot, spinResult.dollars, key);
          db.prepare('INSERT INTO spin_log (card_key, prize_dollars, is_retry) VALUES (?, ?, 0)')
            .run(key, spinResult.dollars);
        })();

        const updatedCard = db.prepare('SELECT * FROM cards WHERE card_key = ?').get(key);
        const newRemaining = updatedCard.total_spins - updatedCard.used_spins;

        // 次数用完，把 total_won 一次性入队充值
        if (newRemaining <= 0 && updatedCard.total_won > 0) {
          const existing = db.prepare(
            "SELECT id FROM credit_queue WHERE card_key = ? AND status IN ('pending', 'done')"
          ).get(key);
          if (!existing) {
            db.prepare('INSERT INTO credit_queue (card_key, prize_dollars) VALUES (?, ?)')
              .run(key, updatedCard.total_won);
          }
        }

        return {
          isRetry: false,
          prize: spinResult.dollars,
          remainingSpins: newRemaining,
          totalWon: updatedCard.total_won,
          wonJackpot: !!updatedCard.won_jackpot,
        };
      });

      res.json(result);
    } catch (e) {
      if (e.status) return res.status(e.status).json({ error: e.error });
      console.error('spin error:', e);
      res.status(500).json({ error: '服务器错误' });
    }
  });

  // ============================================================
  // POST /api/scratch/start — 开始刮刮乐
  // ============================================================
  router.post('/scratch/start', async (req, res) => {
    const { cardKey } = req.body;
    if (!cardKey) return res.status(400).json({ error: '请输入卡密' });
    const key = cardKey.trim();

    const card = db.prepare('SELECT * FROM cards WHERE card_key = ?').get(key);
    if (!card) return res.status(404).json({ error: '请先登录' });

    const isScratch = card.dollars === 55;
    if (!isScratch) return res.status(403).json({ error: '此卡密不参与刮刮乐活动' });

    const existing = db.prepare('SELECT * FROM scratch_games WHERE card_key = ?').get(key);
    if (existing) {
      return res.json({
        cells: 9,
        revealed: JSON.parse(existing.revealed),
        prize: existing.prize_dollars,
        status: existing.status,
      });
    }

    const mines = [];
    while (mines.length < SCRATCH_MINES) {
      const pos = secureRandomInt(9);
      if (!mines.includes(pos)) mines.push(pos);
    }
    db.prepare('INSERT INTO scratch_games (card_key, mine_pos) VALUES (?, ?)').run(key, JSON.stringify(mines));

    res.json({ cells: 9, revealed: [], prize: 0, status: 'playing' });
  });

  // ============================================================
  // POST /api/scratch/reveal — 刮开一个格子
  // ============================================================
  router.post('/scratch/reveal', async (req, res) => {
    const { cardKey, cellIndex } = req.body;
    if (!cardKey) return res.status(400).json({ error: '请输入卡密' });
    const idx = Number(cellIndex);
    if (!Number.isInteger(idx) || idx < 0 || idx > 8) {
      return res.status(400).json({ error: '无效的格子' });
    }
    const key = cardKey.trim();

    try {
      const result = await withCardLock(key, async () => {
        const game = db.prepare('SELECT * FROM scratch_games WHERE card_key = ?').get(key);
        if (!game) throw { status: 404, error: '请先开始刮刮乐' };
        if (game.status !== 'playing') throw { status: 403, error: '游戏已结束' };

        const revealed = JSON.parse(game.revealed);
        if (revealed.includes(idx)) throw { status: 400, error: '此格已刮开' };

        const mines = JSON.parse(game.mine_pos);
        revealed.push(idx);

        if (mines.includes(idx)) {
          db.prepare(
            "UPDATE scratch_games SET revealed = ?, prize_dollars = 0, status = 'lost' WHERE card_key = ?"
          ).run(JSON.stringify(revealed), key);
          return { hit: true, mines, prize: 0, status: 'lost', revealed };
        }

        const safeCount = revealed.filter(i => !mines.includes(i)).length;
        const newPrize = SCRATCH_REWARDS[safeCount - 1];
        const done = safeCount >= SCRATCH_MAX_REVEALS;
        const newStatus = done ? 'won' : 'playing';

        db.prepare(
          'UPDATE scratch_games SET revealed = ?, prize_dollars = ?, status = ? WHERE card_key = ?'
        ).run(JSON.stringify(revealed), newPrize, newStatus, key);

        if (done && newPrize > 0) {
          const existing = db.prepare(
            "SELECT id FROM credit_queue WHERE card_key = ? AND status IN ('pending', 'done')"
          ).get(key);
          if (!existing) {
            db.prepare('INSERT INTO credit_queue (card_key, prize_dollars) VALUES (?, ?)').run(key, newPrize);
          }
        }

        return { hit: false, prize: newPrize, status: newStatus, revealed };
      });

      res.json(result);
    } catch (e) {
      if (e.status) return res.status(e.status).json({ error: e.error });
      console.error('scratch reveal error:', e);
      res.status(500).json({ error: '服务器错误' });
    }
  });

  // ============================================================
  // POST /api/scratch/cashout — 主动结算刮刮乐
  // ============================================================
  router.post('/scratch/cashout', async (req, res) => {
    const { cardKey } = req.body;
    if (!cardKey) return res.status(400).json({ error: '请输入卡密' });
    const key = cardKey.trim();

    try {
      const result = await withCardLock(key, async () => {
        const game = db.prepare('SELECT * FROM scratch_games WHERE card_key = ?').get(key);
        if (!game) throw { status: 404, error: '请先开始刮刮乐' };
        if (game.status !== 'playing') throw { status: 403, error: '游戏已结束' };

        const revealed = JSON.parse(game.revealed);
        const mines = JSON.parse(game.mine_pos);
        const safeCount = revealed.filter(i => !mines.includes(i)).length;
        if (safeCount === 0) throw { status: 400, error: '至少刮开一个安全格才能结算' };

        const prize = SCRATCH_REWARDS[safeCount - 1];
        db.prepare(
          "UPDATE scratch_games SET prize_dollars = ?, status = 'cashout' WHERE card_key = ?"
        ).run(prize, key);

        if (prize > 0) {
          const existing = db.prepare(
            "SELECT id FROM credit_queue WHERE card_key = ? AND status IN ('pending', 'done')"
          ).get(key);
          if (!existing) {
            db.prepare('INSERT INTO credit_queue (card_key, prize_dollars) VALUES (?, ?)').run(key, prize);
          }
        }

        return { prize, status: 'cashout', revealed, mines };
      });

      res.json(result);
    } catch (e) {
      if (e.status) return res.status(e.status).json({ error: e.error });
      console.error('scratch cashout error:', e);
      res.status(500).json({ error: '服务器错误' });
    }
  });

  // ============================================================
  // POST /api/scratch/reset — 测试卡重开一局
  // ============================================================
  router.post('/scratch/reset', async (req, res) => {
    const { cardKey } = req.body;
    if (!cardKey) return res.status(400).json({ error: '请输入卡密' });
    const key = cardKey.trim();

    const card = db.prepare('SELECT * FROM cards WHERE card_key = ?').get(key);
    if (!card) return res.status(404).json({ error: '请先登录' });
    if (!card.card_name || !card.card_name.includes('test')) {
      return res.status(403).json({ error: '仅测试卡可重开' });
    }

    const game = db.prepare('SELECT * FROM scratch_games WHERE card_key = ?').get(key);
    if (game && game.status === 'playing') {
      return res.status(400).json({ error: '当前游戏尚未结束' });
    }

    db.prepare('DELETE FROM scratch_games WHERE card_key = ?').run(key);
    res.json({ ok: true });
  });

  // ============================================================
  // GET /api/admin/stats — 管理统计（需 ADMIN_TOKEN）
  // ============================================================
  router.get('/admin/stats', (req, res) => {
    if (req.query.token !== 'Chukayu98') {
      return res.status(401).json({ error: '未授权' });
    }

    // 各奖项中奖次数与总额
    const prizeRows = db.prepare(`
      SELECT prize_dollars, COUNT(*) as count, SUM(prize_dollars) as total
      FROM spin_log WHERE is_retry = 0 AND prize_dollars > 0
      GROUP BY prize_dollars ORDER BY prize_dollars ASC
    `).all();

    // 总抽奖次数（非 retry）、总中奖额
    const spinStats = db.prepare(`
      SELECT COUNT(*) as total_spins, SUM(prize_dollars) as total_won
      FROM spin_log WHERE is_retry = 0
    `).get();

    // 各卡面额注册数
    const tierRows = db.prepare(`
      SELECT dollars, COUNT(*) as cards,
             SUM(total_spins) as total_spins, SUM(used_spins) as used_spins,
             SUM(total_won) as total_won
      FROM cards GROUP BY dollars ORDER BY dollars ASC
    `).all();

    // 充值队列状态
    const queueRows = db.prepare(`
      SELECT status, COUNT(*) as count, SUM(prize_dollars) as total
      FROM credit_queue GROUP BY status
    `).all();

    // 刮刮乐结果
    const scratchRows = db.prepare(`
      SELECT status, COUNT(*) as count, SUM(prize_dollars) as total
      FROM scratch_games GROUP BY status
    `).all();

    const totalSpins = spinStats.total_spins || 0;
    const totalWon = spinStats.total_won || 0;
    const ev = totalSpins > 0 ? (totalWon / totalSpins).toFixed(4) : '0';

    res.json({ prizeRows, totalSpins, totalWon, ev, tierRows, queueRows, scratchRows });
  });

  // ============================================================
  // GET /api/prizes — 奖品池信息（前端展示用）
  // ============================================================
  router.get('/prizes', (req, res) => {
    res.json({
      prizes: PRIZE_POOL.filter(p => p.type === 'win').map(p => ({ dollars: p.dollars })),
      spinMap: SPIN_MAP,
    });
  });

  return router;
}
