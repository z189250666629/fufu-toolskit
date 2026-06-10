import test from 'node:test';
import assert from 'node:assert/strict';
import { execFileSync, spawnSync } from 'node:child_process';
import { access, chmod, mkdir, readdir, readFile, rm, writeFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';

const scriptsDirUrl = new URL('./', import.meta.url);
const scriptsDir = fileURLToPath(scriptsDirUrl);
const repoRootUrl = new URL('../', scriptsDirUrl);
const repoRoot = fileURLToPath(repoRootUrl);
const shellQuote = (value) => `'${String(value).replaceAll("'", "'\\''")}'`;

async function shellScripts() {
  return (await readdir(scriptsDirUrl))
    .filter((name) => name.endsWith('.sh'))
    .map((name) => ({ name, path: fileURLToPath(new URL(name, scriptsDirUrl)) }));
}

test('deploy shell scripts are LF-only for bash compatibility', async () => {
  for (const script of await shellScripts()) {
    const bytes = await readFile(script.path);
    assert.equal(
      bytes.includes(Buffer.from('\r\n')),
      false,
      `${script.path} must use LF line endings; CRLF makes bash report misleading syntax errors`
    );
  }
});

const bashProbe = spawnSync('bash', ['--version'], { encoding: 'utf8' });

test('deploy shell scripts pass bash syntax checks', { skip: bashProbe.error ? 'bash is not available' : false }, async () => {
  for (const script of await shellScripts()) {
    assert.doesNotThrow(
      () => execFileSync('bash', ['-n', script.name], { cwd: scriptsDir, encoding: 'utf8', stdio: 'pipe' }),
      `${script.path} should pass bash -n`
    );
  }
});

test('deploy script cleans compose env file when deployment fails', { skip: bashProbe.error ? 'bash is not available' : false }, async (t) => {
  const tmpName = `.tmp/deploy-script-${Date.now()}-${Math.random().toString(16).slice(2)}`;
  const tmpUrl = new URL(`${tmpName}/`, repoRootUrl);
  const fakeBinRel = `${tmpName}/bin`;
  const composeRel = `${tmpName}/docker-compose.yml`;
  const composeEnvRel = `${tmpName}/leaky.env`;
  const composeEnvSnapshotRel = `${tmpName}/snapshot.env`;

  t.after(() => rm(tmpUrl, { recursive: true, force: true }));
  await mkdir(new URL('bin/', tmpUrl), { recursive: true });
  await mkdir(new URL('home/', tmpUrl), { recursive: true });
  await writeFile(new URL('docker-compose.yml', tmpUrl), 'services:\n  app:\n    image: ${APP_IMAGE}:${APP_TAG}\n');

  for (const [name, body] of Object.entries({
    'ssh-keygen': '#!/usr/bin/env bash\nexit 0\n',
    'ssh-keyscan': '#!/usr/bin/env bash\nprintf "example ssh-rsa test\\n"\n',
    scp: '#!/usr/bin/env bash\nexit 0\n',
    ssh: [
      '#!/usr/bin/env bash',
      'if [ ! -f "$EXPECTED_COMPOSE_ENV_FILE" ]; then',
      '  printf "compose env was not written before ssh\\n" >&2',
      '  exit 43',
      'fi',
      'cp "$EXPECTED_COMPOSE_ENV_FILE" "$DEPLOY_ENV_SNAPSHOT"',
      'exit 42',
      ''
    ].join('\n')
  })) {
    const commandUrl = new URL(`bin/${name}`, tmpUrl);
    await writeFile(commandUrl, body);
    await chmod(commandUrl, 0o755);
  }

  const envAssignments = {
    APP_NAME: 'fufu-act',
    APP_IMAGE: 'example/app',
    APP_TAG: 'test',
    SSH_HOST: 'example.test',
    SSH_USER: 'deploy',
    SSH_PRIVATE_KEY: 'fake-key',
    DEPLOY_PATH: '/srv/fufu',
    COMPOSE_FILE: composeRel,
    COMPOSE_SERVICE_NAME: 'app',
    COMPOSE_ENV_FILE: composeEnvRel,
    EXPECTED_COMPOSE_ENV_FILE: composeEnvRel,
    DEPLOY_ENV_SNAPSHOT: composeEnvSnapshotRel,
    HOME: `${tmpName}/home`,
    ADMIN_TOKEN: 'secret-admin-token',
    MCY_PASSWORD: 'secret-password'
  };
  const command = [
    `PATH="$PWD/${fakeBinRel}:$PATH"`,
    ...Object.entries(envAssignments).map(([name, value]) => `${name}=${shellQuote(value)}`),
    'bash scripts/deploy-docker-app.sh'
  ].join(' ');
  const result = spawnSync('bash', ['-c', command], { cwd: repoRoot, encoding: 'utf8' });

  assert.equal(result.status, 42, `fake ssh should force deployment failure after env creation\nstdout:\n${result.stdout}\nstderr:\n${result.stderr}`);
  const snapshot = await readFile(new URL('snapshot.env', tmpUrl), 'utf8');
  assert.match(snapshot, /ADMIN_TOKEN=secret-admin-token/);
  assert.match(snapshot, /MCY_PASSWORD=secret-password/);
  await assert.rejects(
    () => access(new URL('leaky.env', tmpUrl)),
    { code: 'ENOENT' },
    'compose env file should be removed after failed deployment'
  );
});

test('deploy script preserves ssh key path with spaces', { skip: bashProbe.error ? 'bash is not available' : false }, async (t) => {
  const tmpName = `.tmp/deploy-script-spaces-${Date.now()}-${Math.random().toString(16).slice(2)}`;
  const tmpUrl = new URL(`${tmpName}/`, repoRootUrl);
  const fakeBinRel = `${tmpName}/bin`;
  const composeRel = `${tmpName}/docker-compose.yml`;
  const composeEnvRel = `${tmpName}/compose.env`;
  const keyRel = `${tmpName}/home/key with spaces/id_ed25519`;

  t.after(() => rm(tmpUrl, { recursive: true, force: true }));
  await mkdir(new URL('bin/', tmpUrl), { recursive: true });
  await mkdir(new URL('home/key with spaces/', tmpUrl), { recursive: true });
  await writeFile(new URL('docker-compose.yml', tmpUrl), 'services:\n  app:\n    image: ${APP_IMAGE}:${APP_TAG}\n');

  const checkKeyArg = [
    '#!/usr/bin/env bash',
    'while [ "$#" -gt 0 ]; do',
    '  if [ "$1" = "-i" ]; then',
    '    if [ "${2:-}" != "$EXPECTED_SSH_KEY_PATH" ]; then',
    '      printf "bad ssh key path argument: <%s>\\n" "${2:-}" >&2',
    '      exit 44',
    '    fi',
    '    exit 0',
    '  fi',
    '  shift',
    'done',
    'printf "missing -i ssh key argument\\n" >&2',
    'exit 45',
    ''
  ].join('\n');
  for (const [name, body] of Object.entries({
    'ssh-keygen': '#!/usr/bin/env bash\nexit 0\n',
    'ssh-keyscan': '#!/usr/bin/env bash\nprintf "example ssh-rsa test\\n"\n',
    ssh: checkKeyArg,
    scp: checkKeyArg
  })) {
    const commandUrl = new URL(`bin/${name}`, tmpUrl);
    await writeFile(commandUrl, body);
    await chmod(commandUrl, 0o755);
  }

  const envAssignments = {
    APP_NAME: 'y2k-nav',
    APP_IMAGE: 'example/y2k',
    APP_TAG: 'test',
    SSH_HOST: 'example.test',
    SSH_USER: 'deploy',
    SSH_PRIVATE_KEY: 'fake-key',
    SSH_KEY_PATH: keyRel,
    EXPECTED_SSH_KEY_PATH: keyRel,
    DEPLOY_PATH: '/srv/fufu',
    COMPOSE_FILE: composeRel,
    COMPOSE_SERVICE_NAME: 'app',
    COMPOSE_ENV_FILE: composeEnvRel,
    HOME: `${tmpName}/home`
  };
  const command = [
    `PATH="$PWD/${fakeBinRel}:$PATH"`,
    ...Object.entries(envAssignments).map(([name, value]) => `${name}=${shellQuote(value)}`),
    'bash scripts/deploy-docker-app.sh'
  ].join(' ');
  const result = spawnSync('bash', ['-c', command], { cwd: repoRoot, encoding: 'utf8' });

  assert.equal(result.status, 0, `ssh/scp should receive SSH_KEY_PATH as one argument\nstdout:\n${result.stdout}\nstderr:\n${result.stderr}`);
});

test('deploy script prints diagnostics when health inspect fails', { skip: bashProbe.error ? 'bash is not available' : false }, async (t) => {
  const tmpName = `.tmp/deploy-script-health-${Date.now()}-${Math.random().toString(16).slice(2)}`;
  const tmpUrl = new URL(`${tmpName}/`, repoRootUrl);
  const fakeBinRel = `${tmpName}/bin`;
  const composeRel = `${tmpName}/docker-compose.yml`;
  const composeEnvRel = `${tmpName}/compose.env`;
  const remoteRel = `${tmpName}/remote`;
  const markerRel = `${tmpName}/diagnostics.txt`;

  t.after(() => rm(tmpUrl, { recursive: true, force: true }));
  await mkdir(new URL('bin/', tmpUrl), { recursive: true });
  await mkdir(new URL('home/', tmpUrl), { recursive: true });
  await writeFile(new URL('docker-compose.yml', tmpUrl), 'services:\n  app:\n    image: ${APP_IMAGE}:${APP_TAG}\n');

  for (const [name, body] of Object.entries({
    'ssh-keygen': '#!/usr/bin/env bash\nexit 0\n',
    'ssh-keyscan': '#!/usr/bin/env bash\nprintf "example ssh-rsa test\\n"\n',
    scp: '#!/usr/bin/env bash\nexit 0\n',
    ssh: [
      '#!/usr/bin/env bash',
      'command="${@: -1}"',
      'export FAKE_REPO_ROOT="$PWD"',
      'bash -c "$command"',
      ''
    ].join('\n'),
    docker: [
      '#!/usr/bin/env bash',
      'if [ "$1" = "compose" ]; then',
      '  args=" $* "',
      '  if [[ "$args" == *" ps -q "* ]]; then',
      '    printf "container-1\\n"',
      '    exit 0',
      '  fi',
      '  if [[ "$args" == *" logs "* ]]; then',
      '    printf "logs captured\\n" >> "$FAKE_REPO_ROOT/$HEALTH_LOG_MARKER"',
      '    exit 0',
      '  fi',
      '  exit 0',
      'fi',
      'if [ "$1" = "inspect" ]; then',
      '  if [ "${2:-}" = "--format" ]; then',
      '    exit 1',
      '  fi',
      '  printf "inspect captured\\n" >> "$FAKE_REPO_ROOT/$HEALTH_LOG_MARKER"',
      '  exit 0',
      'fi',
      'exit 0',
      ''
    ].join('\n')
  })) {
    const commandUrl = new URL(`bin/${name}`, tmpUrl);
    await writeFile(commandUrl, body);
    await chmod(commandUrl, 0o755);
  }

  const envAssignments = {
    APP_NAME: 'y2k-nav',
    APP_IMAGE: 'example/y2k',
    APP_TAG: 'test',
    SSH_HOST: 'example.test',
    SSH_USER: 'deploy',
    SSH_PRIVATE_KEY: 'fake-key',
    DEPLOY_PATH: remoteRel,
    COMPOSE_FILE: composeRel,
    COMPOSE_SERVICE_NAME: 'app',
    COMPOSE_ENV_FILE: composeEnvRel,
    HOME: `${tmpName}/home`,
    HEALTH_LOG_MARKER: markerRel
  };
  const command = [
    `PATH="$PWD/${fakeBinRel}:$PATH"`,
    ...Object.entries(envAssignments).map(([name, value]) => `${name}=${shellQuote(value)}`),
    'bash scripts/deploy-docker-app.sh'
  ].join(' ');
  const result = spawnSync('bash', ['-c', command], { cwd: repoRoot, encoding: 'utf8' });

  assert.notEqual(result.status, 0, `health inspect failure should fail deployment\nstdout:\n${result.stdout}\nstderr:\n${result.stderr}`);
  const diagnostics = await readFile(new URL('diagnostics.txt', tmpUrl), 'utf8');
  assert.match(diagnostics, /logs captured/);
  assert.match(diagnostics, /inspect captured/);
});
