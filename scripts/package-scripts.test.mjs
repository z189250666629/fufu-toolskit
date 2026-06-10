import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';

const packageJson = JSON.parse(
  await readFile(new URL('../package.json', import.meta.url), 'utf8')
);

async function readPackageJson(path) {
  return JSON.parse(await readFile(new URL(path, import.meta.url), 'utf8'));
}

test('root npm test includes workspace Go checks', () => {
  const scripts = packageJson.scripts || {};

  assert.equal(scripts['test:workspace'], 'go test -count=1 .');
  assert.equal(scripts['test:shared'], 'go test -count=1 ./packages/go/fufu/...');
  assert.match(scripts.test || '', /npm run test:workspace/);
});

test('root build scripts cover all deployable apps', () => {
  const scripts = packageJson.scripts || {};

  assert.equal(scripts['build:network'], 'npm --prefix apps/network-detect run build');
  assert.equal(scripts['build:act'], 'npm --prefix apps/fufu-act run build');
  assert.equal(scripts['build:y2k'], 'npm --prefix apps/y2k-nav run build');
  assert.equal(scripts['build:all'], 'npm run build:network && npm run build:act && npm run build:y2k');
});

test('app npm test Go checks are uncached', async () => {
  for (const path of [
    '../apps/network-detect/package.json',
    '../apps/fufu-act/package.json',
    '../apps/y2k-nav/package.json'
  ]) {
    const appPackage = await readPackageJson(path);
    assert.equal(appPackage.scripts?.['test:go'], 'go test -count=1 ./...', `${path} should rerun Go tests instead of using cache`);
  }
});
