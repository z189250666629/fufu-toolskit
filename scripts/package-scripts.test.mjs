import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';

const packageJson = JSON.parse(
  await readFile(new URL('../package.json', import.meta.url), 'utf8')
);

test('root npm test includes workspace Go checks', () => {
  const scripts = packageJson.scripts || {};

  assert.equal(scripts['test:workspace'], 'go test .');
  assert.match(scripts.test || '', /npm run test:workspace/);
});
