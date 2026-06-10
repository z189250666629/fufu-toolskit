import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';

test('scratch test-card flag uses shared standalone-marker helper', async () => {
  const source = await readFile(new URL('./index.html', import.meta.url), 'utf8');

  assert.match(source, /<script src="scratch-card\.js"><\/script>/);
  assert.match(source, /scratchIsTest = scratchCard\.isTestCardName\(data\.cardName\)/);
  assert.doesNotMatch(source, /scratchIsTest = .*includes\('test'\)/);
});
