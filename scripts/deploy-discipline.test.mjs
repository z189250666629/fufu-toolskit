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

test('dashboard-key discipline scans unified deploy docs env and agent files', async () => {
  const paths = await deployDisciplinePaths();
  for (const expected of [
    '.env.example',
    'README.md',
    'docs/CI_CD.md',
    'docs/merge-notes.md',
    'apps/fufu-act/AGENTS.md',
    'apps/network-detect/README.md',
    '.github/workflows/deploy-fufu-tool-site.yml',
    'infra/deploy/fufu-tool-site/docker-compose.yml',
    'scripts/deploy-docker-app.sh'
  ]) {
    assert.equal(paths.includes(expected), true, `${expected} should be covered by deploy discipline scans`);
  }
});

test('only the unified fufu-tool-site deploy workflow remains production deployable', async () => {
  assert.deepEqual(await deployWorkflowPaths(), [
    '.github/workflows/deploy-fufu-tool-site.yml'
  ]);

  const workflowFiles = await readdir(new URL('../.github/workflows/', import.meta.url));
  for (const retired of ['deploy-network.yml', 'deploy-act.yml', 'deploy-y2k-nav.yml', 'deploy-combine.yml']) {
    assert.equal(workflowFiles.includes(retired), false, `${retired} should be retired as an independent production entry`);
  }
});

test('legacy deploy directives now target the unified fufu-tool-site workflow and are documented', async () => {
  const ciDocs = await readRepoFile('docs/CI_CD.md');
  const workflow = await readRepoFile('.github/workflows/deploy-fufu-tool-site.yml');
  for (const directive of [
    '[deploy all]',
    '[deploy fufu-tool-site]',
    '[deploy tool-site]',
    '[deploy network]',
    '[deploy combine]',
    '[deploy activity]',
    '[deploy nav]'
  ]) {
    assert.match(workflow, new RegExp(directive.slice(1, -1).replace(/[.*+?^${}()|[\]\\]/g, '\\$&'), 'i'), `workflow should accept ${directive}`);
    assert.match(ciDocs, new RegExp(directive.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'), 'i'), `docs/CI_CD.md should document ${directive}`);
  }
});

test('deploy workflows use the canonical toolskit GitHub environment', async () => {
  for (const path of await deployWorkflowPaths()) {
    const source = await readRepoFile(path);
    assert.match(source, /^\s*environment:\s*toolskit\s*$/m, `${path} should deploy through the toolskit environment`);
    assert.doesNotMatch(source, /^\s*environment:\s*docker\s*$/m, `${path} must not deploy through a docker environment`);
  }
});

test('unified deploy workflow verify job runs uncached backend and frontend checks before packaging', async () => {
  const source = await readRepoFile('.github/workflows/deploy-fufu-tool-site.yml');
  assert.doesNotMatch(source, /^\s*-\s*run:\s*go test \.\/\.\.\.\s*$/m, 'workflow must not use cache-prone go test ./...');
  assert.match(source, /^\s*-\s*run:\s*go test -count=1 \.\s*$/m, 'workflow should run the workspace Go guard test uncached');
  assert.match(source, /actions\/setup-node@v4/, 'workflow should install Node in deploy verification');
  assert.match(source, /^\s*-\s*run:\s*npm run test:scripts\s*$/m, 'workflow should run root script discipline tests');
  assert.match(source, /^\s*-\s*run:\s*npm --prefix apps\/fufu-tool-site run test:frontend\s*$/m, 'workflow should run unified frontend/module tests');
});

test('docker context excludes non-production static assets from unified runtime app roots', async () => {
  const dockerignore = await readRepoFile('.dockerignore');
  const patterns = dockerignore
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter((line) => line && !line.startsWith('#'));

  for (const staticRoot of [
    'apps/fufu-tool-site/frontend',
    'apps/fufu-tool-site/combine',
    'apps/y2k-nav',
    'apps/fufu-act/public'
  ]) {
    for (const suffix of ['*.test.*', '**/*.test.*', '.*', '**/.*']) {
      const pattern = `${staticRoot}/${suffix}`;
      assert.equal(patterns.includes(pattern), true, `.dockerignore should exclude ${pattern}`);
    }
  }
  for (const pattern of [
    'apps/**/*.test.*',
    'apps/**/*_test.go',
    'scripts/*.test.mjs'
  ]) {
    assert.equal(patterns.includes(pattern), true, `.dockerignore should exclude ${pattern}`);
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
    assert.match(source, /go test -count=1 \.\/\.\.\.|go test -count=1 \./, `${path} should recommend uncached Go tests when showing direct Go commands`);
  }
});

test('fufu-tool-site compose uses the canonical deployed host port and unified service name', async () => {
  const source = await readRepoFile('infra/deploy/fufu-tool-site/docker-compose.yml');
  assert.match(source, /fufu-tool-site:/, 'compose should define the unified service');
  assert.match(source, /\$\{HOST_PORT:-38473\}:8080/, 'compose should default the external host port to 38473');
  assert.doesNotMatch(source, /18820|33148/, 'unified compose should not expose retired activity/nav host ports');
});

test('fufu-tool-site deploy reuses existing NewAPI, activity and MCY variables', async () => {
  const workflow = await readRepoFile('.github/workflows/deploy-fufu-tool-site.yml');
  const compose = await readRepoFile('infra/deploy/fufu-tool-site/docker-compose.yml');
  const deployScript = await readRepoFile('scripts/deploy-docker-app.sh');
  const unifiedCase = deployScript.match(/fufu-tool-site\)([\s\S]*?);;\n\s*\*\)/)?.[1] ?? '';
  assert.notEqual(unifiedCase, '', 'deploy script should contain a fufu-tool-site case block');

  for (const variable of [
    'NEWAPI_MANAGED_API_CONFIG',
    'NEWAPI_API_SITE_URL',
    'NEWAPI_API_SITE_TOKEN',
    'NEWAPI_TOKEN_SITE_URL',
    'NEWAPI_TOKEN_SITE_TOKEN',
    'CONNECTIVITY_API_URLS',
    'CONNECTIVITY_TOKEN_URLS',
    'FUFU_API_BASE_URL',
    'FUFU_API_TOKEN',
    'FUFU_API_USER_ID',
    'FUFU_QUOTA_UNIT',
    'MCY_BASE_URL',
    'MCY_USERNAME',
    'MCY_PASSWORD',
    'MCY_LOGIN_ENDPOINT',
    'MCY_UPLOAD_ENDPOINT',
    'ADMIN_TOKEN'
  ]) {
    assert.match(workflow, new RegExp(`\\b${variable}\\b`), `${variable} should be passed by unified workflow`);
    assert.match(compose, new RegExp(`\\b${variable}\\b`), `${variable} should be passed by unified compose`);
    assert.match(unifiedCase, new RegExp(`\\b${variable}\\b`), `${variable} should be written by deploy script`);
  }
  assert.doesNotMatch(workflow + compose + unifiedCase, /NEWAPI_TOKEN_SITE_ACCESS_TOKEN/, 'do not invent alternate token-site secret names');
});
