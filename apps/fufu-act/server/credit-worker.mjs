import { fufuApi } from '../scripts/api-act.mjs';

const MAX_RETRIES = 5;
const POLL_INTERVAL = 3000;

export function startCreditWorker(db) {
  async function processOne(job) {
    const keyword = job.card_key.replace(/^sk-/, '');

    const search = await fufuApi('GET', `/api/token/search?keyword=&token=${keyword}&p=0&size=10`);
    let token = search.data?.find(t => t.key === keyword);

    if (!token) throw new Error('token not found on fufu');

    const addQuota = job.prize_dollars * 500000;
    const newQuota = (token.remain_quota || 0) + addQuota;
    const name = token.name && token.name.length > 30 ? token.name.slice(0, 30) : token.name;
    const res = await fufuApi('PUT', '/api/token/', { ...token, name, remain_quota: newQuota });

    if (!res.success) throw new Error(`fufu PUT failed: ${JSON.stringify(res)}`);
  }

  async function tick() {
    // 每张卡只取最早的一条 pending，保证同卡串行
    const jobs = db.prepare(`
      SELECT cq.* FROM credit_queue cq
      INNER JOIN (
        SELECT card_key, MIN(id) as min_id
        FROM credit_queue
        WHERE status = 'pending' AND retries < ?
        GROUP BY card_key
      ) earliest ON cq.id = earliest.min_id
      ORDER BY cq.id ASC
      LIMIT 10
    `).all(MAX_RETRIES);

    for (const job of jobs) {
      try {
        await processOne(job);
        db.prepare(
          "UPDATE credit_queue SET status = 'done', processed_at = datetime('now') WHERE id = ?"
        ).run(job.id);
      } catch (e) {
        const retries = job.retries + 1;
        const status = retries >= MAX_RETRIES ? 'failed' : 'pending';
        db.prepare(
          "UPDATE credit_queue SET retries = ?, status = ?, error = ? WHERE id = ?"
        ).run(retries, status, String(e), job.id);
        if (status === 'failed') {
          console.error(`credit_queue #${job.id} failed permanently:`, e.message);
        }
      }
    }
  }

  const timer = setInterval(tick, POLL_INTERVAL);
  tick();

  return { stop: () => clearInterval(timer) };
}
