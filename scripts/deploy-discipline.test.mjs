import test from 'node:test';
import assert from 'node:assert/strict';
import { readdir, readFile } from 'node:fs/promises';

const repoRoot = new URL('../', import.meta.url);

async function readRepoFile(path) {
  return readFile(new URL(path, repoRoot), 'utf8');
}

async function listRepoFiles(dir, accept) {
  const root = new URL(`${dir.replace(/\/$/, '')}/`, repoRoot);
  const out = [];
  async function walk(url, prefix) {
    let entries;
    try {
      entries = await readdir(url, { withFileTypes: true });
    } catch (error) {
      if (error?.code === 'ENOENT') return;
      throw error;
    }
    for (const entry of entries) {
      const rel = `${prefix}${entry.name}`;
      if (entry.isDirectory()) {
        await walk(new URL(`${entry.name}/`, url), `${rel}/`);
        continue;
      }
      if (entry.isFile() && accept(rel)) {
        out.push(rel);
      }
    }
  }
  await walk(root, `${dir.replace(/\/$/, '')}/`);
  return out;
}

async function deployDisciplinePaths() {
  const paths = new Set([
    '.env.example',
    'README.md',
    'scripts/deploy-docker-app.sh'
  ]);
  for (const path of await listRepoFiles('docs', (path) => path.endsWith('.md'))) {
    paths.add(path);
  }
  for (const path of await listRepoFiles('apps', (path) => /\/(?:README|AGENTS|CLAUDE)\.md$/i.test(path) || path.endsWith('/docker-compose.yml'))) {
    paths.add(path);
  }
  for (const path of await listRepoFiles('.github/workflows', (path) => /\/deploy-.*\.ya?ml$/i.test(path))) {
    paths.add(path);
  }
  for (const path of await listRepoFiles('infra/deploy', (path) => path.endsWith('/docker-compose.yml'))) {
    paths.add(path);
  }
  return [...paths].sort();
}

async function deployWorkflowPaths() {
  return (await listRepoFiles('.github/workflows', (path) => /\/deploy-.*\.ya?ml$/i.test(path))).sort();
}

function deployDirectivesFromWorkflow(source, path) {
  const gateLine = source
    .split(/\r?\n/)
    .find((line) => line.includes('grep -qiE') && line.includes('deploy'));
  assert.ok(gateLine, `${path} should contain a deploy directive gate`);
  const group = gateLine.match(/\(([^)]+)\)/)?.[1];
  assert.ok(group, `${path} should keep deploy directives in a regex group`);
  return group.split('|').map((directive) => `[${directive.trim()}]`);
}

test('deploy docs and workflows do not reintroduce dashboard-key auth', async () => {
  const forbidden = [
    /NEWAPI[_-]?DASHBOARD[\w-]*KEY/i,
    /NEWAPI[_-]?LOG[\w-]*KEY/i,
    /api-dashboard-key/i,
    /x-dashboard-key/i,
    /requiresKey/i,
    /dashboard-key/i
  ];

  for (const path of await deployDisciplinePaths()) {
    const source = await readRepoFile(path);
    for (const pattern of forbidden) {
      assert.doesNotMatch(source, pattern, `${path} must not match forbidden dashboard-key auth pattern ${pattern}`);
    }
  }
});

test('dashboard-key discipline scans deploy docs env and agent files', async () => {
  const paths = await deployDisciplinePaths();
  for (const expected of [
    '.env.example',
    'README.md',
    'docs/CI_CD.md',
    'docs/merge-notes.md',
    'apps/fufu-act/AGENTS.md',
    'apps/network-detect/README.md',
    '.github/workflows/deploy-y2k-nav.yml',
    'infra/deploy/y2k-nav/docker-compose.yml',
    'scripts/deploy-docker-app.sh'
  ]) {
    assert.equal(paths.includes(expected), true, `${expected} should be covered by deploy discipline scans`);
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

test('deploy workflow gates cover every deploy workflow', async () => {
  assert.deepEqual(await deployWorkflowPaths(), [
    '.github/workflows/deploy-act.yml',
    '.github/workflows/deploy-network.yml',
    '.github/workflows/deploy-y2k-nav.yml'
  ]);
});

test('deploy directive aliases match the CI/CD runbook', async () => {
  const ciDocs = await readRepoFile('docs/CI_CD.md');
  for (const path of await deployWorkflowPaths()) {
    const source = await readRepoFile(path);
    for (const directive of deployDirectivesFromWorkflow(source, path)) {
      assert.match(ciDocs, new RegExp(directive.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')), `${path} accepts ${directive}, so docs/CI_CD.md should document it`);
    }
  }
});

test('deploy workflows use the canonical toolskit GitHub environment', async () => {
  for (const path of await deployWorkflowPaths()) {
    const source = await readRepoFile(path);
    assert.match(source, /^\s*environment:\s*toolskit\s*$/m, `${path} should deploy through the toolskit environment`);
    assert.doesNotMatch(source, /^\s*environment:\s*docker\s*$/m, `${path} must not deploy through a docker environment`);
  }
});

test('deploy workflow verify jobs run uncached Go tests', async () => {
  for (const path of await deployWorkflowPaths()) {
    const source = await readRepoFile(path);
    assert.doesNotMatch(source, /^\s*-\s*run:\s*go test \.\/\.\.\.\s*$/m, `${path} must not use cache-prone go test ./...`);
    assert.match(source, /^\s*-\s*run:\s*go test -count=1 \.\/\.\.\.\s*$/m, `${path} should disable Go test cache in deploy verification`);
  }
});

test('deploy workflow verify jobs run frontend tests before packaging images', async () => {
  const expectations = new Map([
    ['.github/workflows/deploy-act.yml', 'npm --prefix apps/fufu-act run test:frontend'],
    ['.github/workflows/deploy-network.yml', 'npm --prefix apps/network-detect run test:frontend'],
    ['.github/workflows/deploy-y2k-nav.yml', 'npm --prefix apps/y2k-nav run test:frontend']
  ]);

  for (const path of await deployWorkflowPaths()) {
    const frontendTestCommand = expectations.get(path);
    assert.ok(frontendTestCommand, `${path} should be assigned an app frontend test command`);
    const source = await readRepoFile(path);
    assert.match(source, /actions\/setup-node@v4/, `${path} should install Node in deploy verification`);
    assert.match(source, new RegExp(`^\\s*-\\s*run:\\s*${frontendTestCommand.replaceAll('/', '\\/')}\\s*$`, 'm'), `${path} should run app frontend tests in deploy verification`);
  }
});

test('deploy workflow verify jobs run root script discipline tests', async () => {
  for (const path of await deployWorkflowPaths()) {
    const source = await readRepoFile(path);
    assert.match(
      source,
      /^\s*-\s*run:\s*npm run test:scripts\s*$/m,
      `${path} should run root script discipline tests before packaging images`
    );
  }
});

test('repo docs and agent instructions do not recommend cache-prone Go tests', async () => {
  for (const path of [
    'README.md',
    'apps/fufu-act/AGENTS.md',
    'apps/fufu-act/CLAUDE.md'
  ]) {
    const source = await readRepoFile(path);
    assert.doesNotMatch(source, /(^|`|\s)go test \.\/\.\.\.(`|\s|$)/, `${path} must not recommend cache-prone go test ./...`);
    assert.match(source, /go test -count=1 \.\/\.\.\./, `${path} should recommend uncached Go tests when showing direct Go commands`);
  }
});

test('network-detect compose files use the canonical deployed host port', async () => {
  for (const path of [
    'infra/deploy/network-detect/docker-compose.yml',
    'apps/network-detect/docker-compose.yml'
  ]) {
    const source = await readRepoFile(path);
    assert.match(
      source,
      /\$\{HOST_PORT:-38473\}:8080/,
      `${path} should default the network-detect external host port to 38473`
    );
  }
});

test('fufu-act deploy only forwards activity-consumed NewAPI site variables', async () => {
  const actSources = [
    '.github/workflows/deploy-act.yml',
    'infra/deploy/fufu-act/docker-compose.yml'
  ];
  for (const path of actSources) {
    const source = await readRepoFile(path);
    assert.doesNotMatch(source, /\bNEWAPI_TOKEN_SITE_(URL|TOKEN|ACCESS_TOKEN)\b/, `${path} should not make token-site config look required for fufu-act`);
  }

  const deployScript = await readRepoFile('scripts/deploy-docker-app.sh');
  const actCase = deployScript.match(/fufu-act\)([\s\S]*?);;\n\s*y2k-nav\)/)?.[1] ?? '';
  assert.notEqual(actCase, '', 'deploy script should contain a fufu-act case block');
  assert.doesNotMatch(actCase, /\bNEWAPI_TOKEN_SITE_(URL|TOKEN|ACCESS_TOKEN)\b/, 'deploy script fufu-act block should not forward token-site config');
});
