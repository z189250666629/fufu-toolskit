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
