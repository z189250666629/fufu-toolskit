import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import vm from 'node:vm';

function loadTraceResults() {
  const context = { window: {} };
  context.globalThis = context.window;
  vm.runInNewContext(
    readFileSync(new URL('./combine_trace_results.js', import.meta.url), 'utf8'),
    context
  );
  return context.window.combineTraceResults;
}

test('renderTraceResultsHtml escapes unknown trace statuses', () => {
  const { renderTraceResultsHtml } = loadTraceResults();

  const html = renderTraceResultsHtml([
    {
      mergeId: 1,
      createdAt: 1710000000000,
      status: '<img src=x onerror=alert(1)>',
      direction: 'source',
      sourceKeys: [{ key: 'sk-source-token-1234567890', name: 'source' }],
      resultKey: { key: 'sk-result-token-1234567890', name: 'result' }
    }
  ]);

  assert.match(html, /&lt;img src=x onerror=alert\(1\)&gt;/);
  assert.doesNotMatch(html, /<img src=x onerror=alert\(1\)>/);
});
