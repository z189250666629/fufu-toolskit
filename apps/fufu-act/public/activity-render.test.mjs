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
  const symbols = { 1000: '<b>JP</b>', 5: '🍀' };

  const prizeHtml = render.buildPrizeTableHtml([
    { dollars: '1000', pct: '0.40<script>' },
    { dollars: 'not-a-number' }
  ], symbols);

  assert.match(prizeHtml, /class="prize-row jackpot"/);
  assert.match(prizeHtml, /\$1000 JP/);
  assert.match(prizeHtml, /0\.40&lt;script&gt;%/);
  assert.doesNotMatch(prizeHtml, /0\.40<script>%/);
  assert.match(prizeHtml, /&lt;b&gt;JP&lt;\/b&gt;/);
  assert.doesNotMatch(prizeHtml, /<b>JP<\/b>/);

  const historyHtml = render.buildHistoryHtml([
    { prize_dollars: '5', created_at: '<img src=x onerror=alert(1)>' }
  ], symbols);

  assert.match(historyHtml, /🍀 \$5/);
  assert.match(historyHtml, /&lt;img src=x onerror=alert\(1\)&gt;/);
  assert.doesNotMatch(historyHtml, /<img/);
});
