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

async function runDeployPortValidationCase(t, { hostPort }) {
  const tmpName = `scripts/.tmp/deploy-script-port-${Date.now()}-${Math.random().toString(16).slice(2)}`;
  const tmpUrl = new URL(`${tmpName}/`, repoRootUrl);
  const fakeBinRel = `${tmpName}/bin`;
  const composeRel = `${tmpName}/docker-compose.yml`;
  const composeEnvRel = `${tmpName}/compose.env`;
  const sshMarkerRel = `${tmpName}/ssh-called.txt`;
  const sshMarkerUrl = new URL('ssh-called.txt', tmpUrl);

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
      'printf "called\\n" > "$SSH_CALLED_MARKER"',
      'exit 44',
      ''
    ].join('\n')
  })) {
    const commandUrl = new URL(`bin/${name}`, tmpUrl);
    await writeFile(commandUrl, body);
    await chmod(commandUrl, 0o755);
  }

  const envAssignments = {
    APP_NAME: 'fufu-tool-site',
    APP_IMAGE: 'example/tool-site',
    APP_TAG: 'test',
    SSH_HOST: 'example.test',
    SSH_USER: 'deploy',
    SSH_PRIVATE_KEY: 'fake-key',
    DEPLOY_PATH: '/srv/fufu',
    COMPOSE_FILE: composeRel,
    COMPOSE_SERVICE_NAME: 'app',
    COMPOSE_ENV_FILE: composeEnvRel,
    HOME: `${tmpName}/home`,
    SSH_CALLED_MARKER: sshMarkerRel
  };
  if (hostPort !== undefined) {
    envAssignments.HOST_PORT = hostPort;
  }
  const command = [
    `PATH="$PWD/${fakeBinRel}:$PATH"`,
    ...Object.entries(envAssignments).map(([name, value]) => `${name}=${shellQuote(value)}`),
    'bash scripts/deploy-docker-app.sh'
  ].join(' ');
  const result = spawnSync('bash', ['-c', command], { cwd: repoRoot, encoding: 'utf8' });
  let sshWasCalled = true;
  try {
    await access(sshMarkerUrl);
  } catch (error) {
    if (error.code !== 'ENOENT') throw error;
    sshWasCalled = false;
  }
  return { result, sshWasCalled };
}

test('deploy script fails before ssh when fufu-tool-site HOST_PORT is missing or invalid', { skip: bashProbe.error ? 'bash is not available' : false }, async (t) => {
  for (const tc of [
    { name: 'missing', hostPort: undefined },
    { name: 'non numeric', hostPort: 'abc' },
    { name: 'zero', hostPort: '0' },
    { name: 'too high', hostPort: '65536' }
  ]) {
    await t.test(tc.name, async (t) => {
      const { result, sshWasCalled } = await runDeployPortValidationCase(t, tc);

      assert.equal(result.status, 1, `deployment should fail quickly\nstdout:\n${result.stdout}\nstderr:\n${result.stderr}`);
      assert.match(result.stderr, /HOST_PORT/);
      assert.equal(sshWasCalled, false, 'invalid HOST_PORT should fail before ssh is called');
    });
  }
});

test('deploy script cleans compose env file when deployment fails', { skip: bashProbe.error ? 'bash is not available' : false }, async (t) => {
  const tmpName = `scripts/.tmp/deploy-script-${Date.now()}-${Math.random().toString(16).slice(2)}`;
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
    APP_NAME: 'fufu-tool-site',
    APP_IMAGE: 'example/tool-site',
    APP_TAG: 'test',
    SSH_HOST: 'example.test',
    SSH_USER: 'deploy',
    SSH_PRIVATE_KEY: 'fake-key',
    DEPLOY_PATH: '/srv/fufu',
    COMPOSE_FILE: composeRel,
    COMPOSE_SERVICE_NAME: 'app',
    COMPOSE_ENV_FILE: composeEnvRel,
    HOST_PORT: '38473',
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

test('deploy script uploads managed-site config file for fufu-tool-site', { skip: bashProbe.error ? 'bash is not available' : false }, async (t) => {
  const tmpName = `scripts/.tmp/deploy-script-managed-config-${Date.now()}-${Math.random().toString(16).slice(2)}`;
  const tmpUrl = new URL(`${tmpName}/`, repoRootUrl);
  const fakeBinRel = `${tmpName}/bin`;
  const composeRel = `${tmpName}/docker-compose.yml`;
  const composeEnvRel = `${tmpName}/compose.env`;
  const composeEnvSnapshotRel = `${tmpName}/snapshot.env`;
  const managedConfigRel = `${tmpName}/managed-sites.json`;
  const scpLogRel = `${tmpName}/scp.log`;

  t.after(() => rm(tmpUrl, { recursive: true, force: true }));
  await mkdir(new URL('bin/', tmpUrl), { recursive: true });
  await mkdir(new URL('home/', tmpUrl), { recursive: true });
  await writeFile(new URL('docker-compose.yml', tmpUrl), 'services:\n  fufu-tool-site:\n    image: ${APP_IMAGE}:${APP_TAG}\n');
  await writeFile(new URL('managed-sites.json', tmpUrl), '[{"name":"api","url":"https://api.example.test","tokenEnv":"NEWAPI_API_SITE_TOKEN"}]\n');

  for (const [name, body] of Object.entries({
    'ssh-keygen': '#!/usr/bin/env bash\nexit 0\n',
    'ssh-keyscan': '#!/usr/bin/env bash\nprintf "example ssh-rsa test\\n"\n',
    ssh: '#!/usr/bin/env bash\nexit 0\n',
    scp: [
      '#!/usr/bin/env bash',
      'printf "%s\\n" "$*" >> "$SCP_LOG"',
      'for arg in "$@"; do',
      '  if [ "$arg" = "$EXPECTED_COMPOSE_ENV_FILE" ]; then',
      '    cp "$arg" "$DEPLOY_ENV_SNAPSHOT"',
      '  fi',
      'done',
      'exit 0',
      ''
    ].join('\n')
  })) {
    const commandUrl = new URL(`bin/${name}`, tmpUrl);
    await writeFile(commandUrl, body);
    await chmod(commandUrl, 0o755);
  }

  const envAssignments = {
    APP_NAME: 'fufu-tool-site',
    APP_IMAGE: 'example/tool-site',
    APP_TAG: 'test',
    SSH_HOST: 'example.test',
    SSH_USER: 'deploy',
    SSH_PRIVATE_KEY: 'fake-key',
    DEPLOY_PATH: '/srv/fufu/fufu-tool-site',
    COMPOSE_FILE: composeRel,
    COMPOSE_SERVICE_NAME: 'fufu-tool-site',
    COMPOSE_ENV_FILE: composeEnvRel,
    HOST_PORT: '38473',
    HOME: `${tmpName}/home`,
    NEWAPI_MANAGED_API_CONFIG: managedConfigRel,
    NEWAPI_API_SITE_TOKEN: 'secret-api-token',
    EXPECTED_COMPOSE_ENV_FILE: composeEnvRel,
    DEPLOY_ENV_SNAPSHOT: composeEnvSnapshotRel,
    SCP_LOG: scpLogRel
  };
  const command = [
    `PATH="$PWD/${fakeBinRel}:$PATH"`,
    ...Object.entries(envAssignments).map(([name, value]) => `${name}=${shellQuote(value)}`),
    'bash scripts/deploy-docker-app.sh'
  ].join(' ');
  const result = spawnSync('bash', ['-c', command], { cwd: repoRoot, encoding: 'utf8' });

  assert.equal(result.status, 0, `deployment should complete with fake ssh/scp\nstdout:\n${result.stdout}\nstderr:\n${result.stderr}`);
  const envSnapshot = await readFile(new URL('snapshot.env', tmpUrl), 'utf8');
  assert.match(envSnapshot, /^NEWAPI_MANAGED_API_CONFIG=config\.json$/m, 'container env should use the uploaded config path');
  assert.doesNotMatch(envSnapshot, new RegExp(managedConfigRel.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')), 'container env must not keep the runner-local config path');

  const scpLog = await readFile(new URL('scp.log', tmpUrl), 'utf8');
  assert.match(scpLog, new RegExp(`${managedConfigRel.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')} .*:/srv/fufu/fufu-tool-site/config\\.json`), 'managed-site config JSON should be uploaded beside compose .env');
});

test('deploy script preserves ssh key path with spaces', { skip: bashProbe.error ? 'bash is not available' : false }, async (t) => {
  const tmpName = `scripts/.tmp/deploy-script-spaces-${Date.now()}-${Math.random().toString(16).slice(2)}`;
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
    APP_NAME: 'fufu-tool-site',
    APP_IMAGE: 'example/tool-site',
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
    HOST_PORT: '38473',
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

test('deploy script quotes deploy path inside remote ssh commands', { skip: bashProbe.error ? 'bash is not available' : false }, async (t) => {
  const tmpName = `scripts/.tmp/deploy-script-remote-quote-${Date.now()}-${Math.random().toString(16).slice(2)}`;
  const tmpUrl = new URL(`${tmpName}/`, repoRootUrl);
  const fakeBinRel = `${tmpName}/bin`;
  const composeRel = `${tmpName}/docker-compose.yml`;
  const composeEnvRel = `${tmpName}/compose.env`;
  const injectionMarkerRel = `${tmpName}/injected.txt`;

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
      'exit 42',
      ''
    ].join('\n')
  })) {
    const commandUrl = new URL(`bin/${name}`, tmpUrl);
    await writeFile(commandUrl, body);
    await chmod(commandUrl, 0o755);
  }

  const envAssignments = {
    APP_NAME: 'fufu-tool-site',
    APP_IMAGE: 'example/tool-site',
    APP_TAG: 'test',
    SSH_HOST: 'example.test',
    SSH_USER: 'deploy',
    SSH_PRIVATE_KEY: 'fake-key',
    DEPLOY_PATH: `${tmpName}/remote'; printf pwned > ${injectionMarkerRel}; #`,
    COMPOSE_FILE: composeRel,
    COMPOSE_SERVICE_NAME: 'app',
    COMPOSE_ENV_FILE: composeEnvRel,
    HOST_PORT: '38473',
    HOME: `${tmpName}/home`
  };
  const command = [
    `PATH="$PWD/${fakeBinRel}:$PATH"`,
    ...Object.entries(envAssignments).map(([name, value]) => `${name}=${shellQuote(value)}`),
    'bash scripts/deploy-docker-app.sh'
  ].join(' ');
  const result = spawnSync('bash', ['-c', command], { cwd: repoRoot, encoding: 'utf8' });

  assert.equal(result.status, 42, `fake ssh should stop after evaluating first remote command\nstdout:\n${result.stdout}\nstderr:\n${result.stderr}`);
  await assert.rejects(
    () => access(new URL('injected.txt', tmpUrl)),
    { code: 'ENOENT' },
    'DEPLOY_PATH must not be executable as remote shell code'
  );
});

test('deploy script prints diagnostics when health inspect fails', { skip: bashProbe.error ? 'bash is not available' : false }, async (t) => {
  const tmpName = `scripts/.tmp/deploy-script-health-${Date.now()}-${Math.random().toString(16).slice(2)}`;
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
    APP_NAME: 'fufu-tool-site',
    APP_IMAGE: 'example/tool-site',
    APP_TAG: 'test',
    SSH_HOST: 'example.test',
    SSH_USER: 'deploy',
    SSH_PRIVATE_KEY: 'fake-key',
    DEPLOY_PATH: remoteRel,
    COMPOSE_FILE: composeRel,
    COMPOSE_SERVICE_NAME: 'app',
    COMPOSE_ENV_FILE: composeEnvRel,
    HOST_PORT: '38473',
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
