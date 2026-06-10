import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';

const repoRoot = new URL('../', import.meta.url);

async function readRepoFile(path) {
  return readFile(new URL(path, repoRoot), 'utf8');
}

function envExampleNames(source) {
  return new Set(
    source
      .split(/\r?\n/)
      .map((line) => line.match(/^([A-Z0-9_]+)=/)?.[1])
      .filter(Boolean)
  );
}

test('.env.example includes existing config variables passed by deploy', async () => {
  const requiredVariables = [
    {
      name: 'NEWAPI_MANAGED_API_CONFIG',
      docs: ['README.md', 'docs/CI_CD.md'],
      deploy: ['.github/workflows/deploy-network.yml', 'infra/deploy/network-detect/docker-compose.yml']
    },
    {
      name: 'ADMIN_TOKEN',
      docs: ['docs/CI_CD.md'],
      deploy: ['.github/workflows/deploy-act.yml', 'infra/deploy/fufu-act/docker-compose.yml']
    }
  ];
  const envExample = await readRepoFile('.env.example');
  const envNames = envExampleNames(envExample);

  for (const variable of requiredVariables) {
    for (const path of variable.docs) {
      assert.match(await readRepoFile(path), new RegExp(`\\b${variable.name}\\b`), `${variable.name} is documented in ${path}`);
    }
    for (const path of variable.deploy) {
      assert.match(await readRepoFile(path), new RegExp(`\\b${variable.name}\\b`), `${variable.name} is passed by ${path}`);
    }
    assert.equal(
      envNames.has(variable.name),
      true,
      `${variable.name} already exists in docs/deploy paths and must be present in .env.example for local setup parity`
    );
  }
});

test('CONNECTIVITY overrides are documented as optional fallbacks to NewAPI site URLs', async () => {
  const [envExample, readme, workflow, compose] = await Promise.all([
    readRepoFile('.env.example'),
    readRepoFile('README.md'),
    readRepoFile('.github/workflows/deploy-network.yml'),
    readRepoFile('infra/deploy/network-detect/docker-compose.yml')
  ]);
  const ciDocs = await readRepoFile('docs/CI_CD.md');

  for (const variable of ['CONNECTIVITY_API_URLS', 'CONNECTIVITY_TOKEN_URLS']) {
    assert.match(workflow, new RegExp(`\\b${variable}\\b`), `${variable} is passed by deploy-network.yml`);
    assert.match(compose, new RegExp(`\\b${variable}\\b`), `${variable} is passed by network-detect compose`);
    assert.match(envExample, new RegExp(`\\b${variable}=`), `${variable} remains listed for local optional overrides`);
  }

  for (const source of [
    ['README.md', readme],
    ['docs/CI_CD.md', ciDocs],
    ['.env.example', envExample]
  ]) {
    const [path, text] = source;
    assert.match(text, /CONNECTIVITY_API_URLS[\s\S]*可选/, `${path} should describe CONNECTIVITY_API_URLS as optional`);
    assert.match(text, /CONNECTIVITY_TOKEN_URLS[\s\S]*可选/, `${path} should describe CONNECTIVITY_TOKEN_URLS as optional`);
    assert.match(text, /CONNECTIVITY_API_URLS[\s\S]*NEWAPI_API_SITE_URL/, `${path} should document API URL fallback`);
    assert.match(text, /CONNECTIVITY_TOKEN_URLS[\s\S]*NEWAPI_TOKEN_SITE_URL/, `${path} should document token URL fallback`);
  }
});
