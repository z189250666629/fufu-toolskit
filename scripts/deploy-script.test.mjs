import test from 'node:test';
import assert from 'node:assert/strict';
import { execFileSync, spawnSync } from 'node:child_process';
import { readdir, readFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';

const scriptsDirUrl = new URL('./', import.meta.url);
const scriptsDir = fileURLToPath(scriptsDirUrl);

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
