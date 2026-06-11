import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { Script, createContext } from 'node:vm';

async function loadAdminRender() {
  const source = await readFile(new URL('./admin-render.js', import.meta.url), 'utf8');
  const context = createContext({});
  new Script(source).runInContext(context);
  return context.adminRender;
}

test('admin render escapes badge statuses and tolerates missing rows', async () => {
  const render = await loadAdminRender();
  const html = render.buildStatsGridHtml({
    totalSpins: 2,
    totalWon: 5,
    ev: '<script>alert(1)</script>',
    queueRows: [{ status: 'paid" onclick="alert(1)', count: 1, total: 2 }]
  });

  assert.match(html, /class="stats-grid"/);
  assert.match(html, /PRIZE DIST/);
  assert.match(html, /CARD TIERS/);
  assert.match(html, /SCRATCH CARD/);
  assert.match(html, /&lt;script&gt;alert\(1\)&lt;\/script&gt;/);
  assert.match(html, /badge-unknown/);
  assert.match(html, /PAID&quot; ONCLICK=&quot;ALERT\(1\)/);
  assert.doesNotMatch(html, /onclick=/);
  assert.doesNotMatch(html, /<script>/);
});

test('admin render preserves backend status badge classes', async () => {
  const render = await loadAdminRender();
  const html = render.buildStatsGridHtml({
    queueRows: [
      { status: 'pending', count: 1, total: 2 },
      { status: 'done', count: 1, total: 2 }
    ],
    scratchRows: [
      { status: 'playing', count: 1, total: 0 },
      { status: 'won', count: 1, total: 8 },
      { status: 'lost', count: 1, total: 0 },
      { status: 'cashout', count: 1, total: 4 }
    ]
  });

  for (const status of ['pending', 'done', 'playing', 'won', 'lost', 'cashout']) {
    assert.match(html, new RegExp(`badge-${status}`));
  }
});

test('admin render builds sale card manager without leaking generated keys', async () => {
  const render = await loadAdminRender();
  const html = render.buildSaleCardManagerHtml({
    plans: [{
      id: 'fufu<special>',
      name: '<img src=x onerror=alert(1)>',
      quota: 55,
      group: 'mix',
      intervalUnit: 9,
      itemId: 29,
      skuId: 66
    }],
    schedule: {
      enabled: true,
      time: '08:30',
      timezone: 'Asia/Shanghai',
      jobs: [{ plan: 'fufu<special>', count: 2, enabled: true }]
    }
  }, {
    uploaded: 1,
    keys: ['sk-secret-generated-card']
  });

  assert.match(html, /SALE CARD/);
  assert.match(html, /每日自动任务/);
  assert.match(html, /id="sale-card-plan"/);
  assert.match(html, /id="sale-card-save"/);
  assert.match(html, /id="sale-card-run"/);
  assert.match(html, /data-sale-card-plan="fufu&lt;special&gt;"/);
  assert.match(html, /&lt;img src=x onerror=alert\(1\)&gt;/);
  assert.match(html, /上架成功：1 张/);
  assert.doesNotMatch(html, /<img/);
  assert.doesNotMatch(html, /sk-secret-generated-card/);
});
