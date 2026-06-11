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

test('root scripts promote fufu-tool-site as the only runnable deploy app', () => {
  const scripts = packageJson.scripts || {};

  assert.equal(scripts['start:tool-site'], 'npm --prefix apps/fufu-tool-site start');
  assert.equal(scripts['dev:tool-site'], 'npm --prefix apps/fufu-tool-site run dev');
  assert.equal(scripts['build:tool-site'], 'npm --prefix apps/fufu-tool-site run build');
  assert.equal(scripts['test:tool-site'], 'npm --prefix apps/fufu-tool-site test');
  assert.equal(scripts['build:all'], 'npm run build:tool-site');
  for (const retired of ['start:network', 'start:act', 'start:y2k', 'build:network', 'build:act', 'build:y2k']) {
    assert.equal(scripts[retired], undefined, `${retired} should not remain a root production entry`);
  }
});

test('tool-site npm test covers embedded module frontend assets', async () => {
  const appPackage = await readPackageJson('../apps/fufu-tool-site/package.json');

  assert.equal(appPackage.scripts?.build, 'npm run build:ui && go build -o fufu-tool-site .');
  assert.equal(appPackage.scripts?.['build:ui'], 'vite build --config ui/vite.config.ts');
  assert.equal(appPackage.scripts?.['typecheck:ui'], 'tsc -p ui/tsconfig.json --noEmit');
  assert.equal(appPackage.scripts?.['test:go'], 'go test -count=1 ./...');
  assert.match(appPackage.scripts?.['test:frontend'] || '', /\*\.test\.mjs/);
  assert.match(appPackage.scripts?.['test:frontend'] || '', /frontend\/\*\.test\.mjs/);
  assert.match(appPackage.scripts?.['test:frontend'] || '', /\.\.\/y2k-nav\/\*\.test\.mjs/);
  assert.match(appPackage.scripts?.['test:frontend'] || '', /\.\.\/fufu-act\/public\/\*\.test\.mjs/);
});

test('embedded modules keep uncached Go test scripts but are not root-started', async () => {
  for (const path of [
    '../apps/network-detect/package.json',
    '../apps/fufu-act/package.json',
    '../apps/y2k-nav/package.json'
  ]) {
    const appPackage = await readPackageJson(path);
    assert.equal(appPackage.scripts?.['test:go'], 'go test -count=1 ./...', `${path} should rerun Go tests instead of using cache`);
  }
});
