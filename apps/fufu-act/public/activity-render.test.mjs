import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { Script, createContext } from 'node:vm';

async function loadActivityRender() {
  const source = await readFile(new URL('./activity-render.js', import.meta.url), 'utf8');
  const context = createContext({});
  new Script(source).runInContext(context);
  return context.activityRender;
}

test('activity render escapes history timestamp and coerces prize rows', async () => {
  const render = await loadActivityRender();
  const symbols = { 5: '🍀' };

  const prizeHtml = render.buildPrizeTableHtml([
    { dollars: '500', rank: 'jackpot', label: '<b>大奖</b>', pct: '0.40<script>' },
    { dollars: '300', rank: 'second', label: '二等奖', pct: '1.20' },
    { dollars: '13', rank: 'minimum', label: '最小保底奖' },
    { dollars: 'not-a-number' }
  ], symbols);

  assert.match(prizeHtml, /class="prize-row jackpot"/);
  assert.match(prizeHtml, /&lt;b&gt;大奖&lt;\/b&gt; \$500 JP/);
  assert.match(prizeHtml, /class="prize-row second"/);
  assert.match(prizeHtml, /二等奖 \$300/);
  assert.match(prizeHtml, /🛡️/);
  assert.match(prizeHtml, /最小保底奖 \$13/);
  assert.doesNotMatch(prizeHtml, /class="odds"/);
  assert.doesNotMatch(prizeHtml, /0\.40&lt;script&gt;%/);
  assert.doesNotMatch(prizeHtml, /0\.40<script>%/);
  assert.doesNotMatch(prizeHtml, /<b>大奖<\/b>/);

  const historyHtml = render.buildHistoryHtml([
    { prize_dollars: '5', created_at: '<img src=x onerror=alert(1)>' }
  ], symbols);

  assert.match(historyHtml, /🍀 \$5/);
  assert.match(historyHtml, /&lt;img src=x onerror=alert\(1\)&gt;/);
  assert.doesNotMatch(historyHtml, /<img/);
});
