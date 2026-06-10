import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';

test('scratch test-card flag is case-insensitive', async () => {
  const source = await readFile(new URL('./index.html', import.meta.url), 'utf8');

  assert.match(source, /\(data\.cardName \|\| ''\)\.toLowerCase\(\)\.includes\('test'\)/);
});
