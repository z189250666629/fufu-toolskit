import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { ROOT_START_SCRIPT, START_COMMANDS } from './start-config.mjs';
import { NODE_TEST_FILES, ROOT_TEST_SCRIPT, TEST_ENTRYPOINT } from './test-config.mjs';

const packageJson = JSON.parse(
  await readFile(new URL('../package.json', import.meta.url), 'utf8')
);

async function readPackageJson(path) {
  return JSON.parse(await readFile(new URL(path, import.meta.url), 'utf8'));
}

test('root scripts expose one centralized test entry', () => {
  const scripts = packageJson.scripts || {};
  const testScriptNames = Object.keys(scripts)
    .filter((name) => name === 'test' || name.startsWith('test:'))
    .sort();

  assert.deepEqual(testScriptNames, ['test']);
  assert.equal(scripts.test, ROOT_TEST_SCRIPT);
  assert.equal(TEST_ENTRYPOINT, 'scripts/test-suite.mjs');
  assert.doesNotMatch(scripts.test, /npm --prefix|go test|test:/);
});

test('root test suite runs through one Node entrypoint without Go test executables', () => {
  assert.equal(NODE_TEST_FILES.includes('scripts/package-scripts.test.mjs'), true);
  assert.equal(NODE_TEST_FILES.includes('apps/fufu-tool-site/web/status/api.test.mjs'), true);
  assert.equal(NODE_TEST_FILES.includes('apps/y2k-nav/theme.test.mjs'), true);
  assert.equal(NODE_TEST_FILES.includes('apps/fufu-act/public/activity-api.test.mjs'), true);
  assert.equal(NODE_TEST_FILES.some((file) => file.includes('network-detect')), false);
  assert.doesNotMatch(ROOT_TEST_SCRIPT, /apps\/fufu-act|legacy\/network-detect|apps\/y2k-nav|go test/);
});

test('root scripts expose one centralized startup entry', () => {
  const scripts = packageJson.scripts || {};
  const startupScriptNames = Object.keys(scripts)
    .filter((name) => name === 'start' || name.startsWith('start:') || name.startsWith('dev:'))
    .sort();

  assert.deepEqual(startupScriptNames, ['start']);
  assert.equal(scripts.start, ROOT_START_SCRIPT);
  assert.deepEqual(START_COMMANDS, [
    { name: 'tool-site', command: 'npm', args: ['--prefix', 'apps/fufu-tool-site', 'start'] }
  ]);
  assert.equal(scripts['build:tool-site'], 'npm --prefix apps/fufu-tool-site run build');
  assert.equal(scripts['build:all'], 'npm run build:tool-site');
  for (const retired of ['start:all', 'start:tool-site', 'dev:tool-site', 'start:network', 'start:act', 'start:y2k', 'build:network', 'build:act', 'build:y2k']) {
    assert.equal(scripts[retired], undefined, `${retired} should not remain a root production entry`);
  }
});

test('tool-site package does not expose local test scripts', async () => {
  const appPackage = await readPackageJson('../apps/fufu-tool-site/package.json');

  assert.equal(appPackage.scripts?.build, 'npm run build:ui && go build -o fufu-tool-site .');
  assert.equal(appPackage.scripts?.['build:ui'], 'vite build --config ui/vite.config.ts');
  assert.equal(appPackage.scripts?.['typecheck:ui'], 'tsc -p ui/tsconfig.json --noEmit');
  assert.equal(appPackage.scripts?.test, undefined);
  assert.equal(appPackage.scripts?.['test:go'], undefined);
  assert.equal(appPackage.scripts?.['test:frontend'], undefined);
});

test('embedded and legacy module packages do not expose local test scripts', async () => {
  for (const path of [
    '../legacy/network-detect/package.json',
    '../apps/fufu-act/package.json',
    '../apps/y2k-nav/package.json'
  ]) {
    const appPackage = await readPackageJson(path);
    assert.equal(appPackage.scripts?.test, undefined, `${path} should not expose package-local test`);
    assert.equal(appPackage.scripts?.['test:go'], undefined, `${path} should not expose split test:go`);
    assert.equal(appPackage.scripts?.['test:frontend'], undefined, `${path} should not expose split test:frontend`);
  }
});
