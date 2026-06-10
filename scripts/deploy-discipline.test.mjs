import test from 'node:test';
import assert from 'node:assert/strict';
import { readdir, readFile } from 'node:fs/promises';

const repoRoot = new URL('../', import.meta.url);
const deployScope = [
  '.env.example',
  'README.md',
  'docs/CI_CD.md',
  '.github/workflows/deploy-act.yml',
  '.github/workflows/deploy-network.yml',
  '.github/workflows/deploy-y2k-nav.yml',
  'infra/deploy/fufu-act/docker-compose.yml',
  'infra/deploy/network-detect/docker-compose.yml',
  'scripts/deploy-docker-app.sh'
];

async function readRepoFile(path) {
  return readFile(new URL(path, repoRoot), 'utf8');
}

test('deploy docs and workflows do not reintroduce dashboard-key auth', async () => {
  const forbidden = [
    'NEWAPI_DASHBOARD_VIEW_KEY',
    'NEWAPI_LOG_VIEW_KEY',
    'api-dashboard-key',
    'X-Dashboard-Key',
    'x-dashboard-key',
    'requiresKey',
    'dashboard-key'
  ];

  for (const path of deployScope) {
    const source = await readRepoFile(path);
    for (const token of forbidden) {
      assert.equal(source.includes(token), false, `${path} must not contain ${token}`);
    }
  }
});

test('combine deploy remains a network-detect alias, not stale fufu-combine deploy vocabulary', async () => {
  const workflowFiles = await readdir(new URL('../.github/workflows/', import.meta.url));
  assert.equal(workflowFiles.includes('deploy-combine.yml'), false, 'standalone combine workflow should not exist');

  const networkWorkflow = await readRepoFile('.github/workflows/deploy-network.yml');
  assert.match(networkWorkflow, /\[.*deploy combine.*\]/i, 'deploy-network keeps [deploy combine] compatibility alias');
  assert.doesNotMatch(networkWorkflow, /deploy fufu-combine/i, 'deploy-network should not accept stale [deploy fufu-combine] directive');
  assert.doesNotMatch(networkWorkflow, /apps\/fufu-combine/i, 'deploy-network should not reference the removed standalone app path');
});

test('deploy workflows use the canonical toolskit GitHub environment', async () => {
  for (const path of [
    '.github/workflows/deploy-act.yml',
    '.github/workflows/deploy-network.yml',
    '.github/workflows/deploy-y2k-nav.yml'
  ]) {
    const source = await readRepoFile(path);
    assert.match(source, /^\s*environment:\s*toolskit\s*$/m, `${path} should deploy through the toolskit environment`);
    assert.doesNotMatch(source, /^\s*environment:\s*docker\s*$/m, `${path} must not deploy through a docker environment`);
  }
});
