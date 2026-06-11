import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';

const source = await readFile(new URL('./index.html', import.meta.url), 'utf8');

test('admin entry is a dedicated top-right navigation action', () => {
  assert.match(source, /class="top-actions"/);
  assert.match(source, /<a\s+href="\/admin"\s+class="admin-entry"/);
  assert.match(source, /管理后台/);
  assert.match(source, /\.top-actions\s*\{[\s\S]*position:\s*fixed;[\s\S]*top:\s*24px;[\s\S]*right:\s*24px;/);
  assert.doesNotMatch(source, /class="nav-card[^"]*"[^>]*data-href="\/admin"/);
  assert.doesNotMatch(source, /<div class="card-title">活动后台<\/div>/);
});
