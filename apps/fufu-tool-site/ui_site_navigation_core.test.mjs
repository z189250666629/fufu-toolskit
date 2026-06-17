import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import ts from 'typescript';

const appRoot = new URL('.', import.meta.url);

async function importTypeScript(path) {
  const source = await readFile(new URL(path, appRoot), 'utf8');
  const transpiled = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.ES2022,
      target: ts.ScriptTarget.ES2022,
      isolatedModules: true
    },
    fileName: path
  });
  return import(`data:text/javascript;base64,${Buffer.from(transpiled.outputText).toString('base64')}`);
}

test('site navigation editor core updates managed sites as plain objects', async () => {
  const core = await importTypeScript('ui/src/admin/siteNavigationConfigCore.ts');
  const cases = [
    {
      name: 'missing api site is created before patching token',
      run: () => core.updateManagedSite([], 'api', { token: 'api-token' }),
      expect: [{ name: '次数fufu', category: 'api', token: 'api-token', urls: [], userId: '1' }]
    },
    {
      name: 'missing token site uses token default name',
      run: () => core.addManagedSiteURL([], 'token'),
      expect: [{ name: 'token-fufu', category: 'token', token: '', urls: [{ name: '线路 1', url: '' }], userId: '1' }]
    },
    {
      name: 'existing site keeps its fields and appends the next line number',
      run: () => core.addManagedSiteURL([
        { name: 'custom-api', category: 'api', urls: [{ name: 'main', url: 'https://a.test' }], userId: '7' }
      ], 'api'),
      expect: [{ name: 'custom-api', category: 'api', urls: [{ name: 'main', url: 'https://a.test' }, { name: '线路 2', url: '' }], userId: '7' }]
    }
  ];

  for (const row of cases) {
    assert.deepEqual(row.run().map(({ name, category, token, urls, userId }) => ({ name, category, token, urls, userId })).map((site) => {
      Object.keys(site).forEach((key) => site[key] === undefined && delete site[key]);
      return site;
    }), row.expect, row.name);
  }
});

test('site navigation editor core updates home cards as plain objects', async () => {
  const core = await importTypeScript('ui/src/admin/siteNavigationConfigCore.ts');
  const cards = [{
    id: 'status',
    stamp: '状态',
    title: '状态页',
    accent: 'moss',
    href: '/status',
    links: [{ label: '备用', href: 'https://backup.test' }]
  }];

  const cases = [
    {
      name: 'switching to runtime api lines clears static href and links',
      run: () => core.setNavigationCardLineKind(cards, 0, 'api')[0],
      expect: { id: 'status', lineKind: 'api', href: '', links: [] }
    },
    {
      name: 'switching back to static keeps the current href and links',
      run: () => core.setNavigationCardLineKind([{ ...cards[0], lineKind: 'api', href: '/kept', links: [{ label: 'x', href: '/x' }] }], 0, '')[0],
      expect: { id: 'status', lineKind: '', href: '/kept', links: [{ label: 'x', href: '/x' }] }
    },
    {
      name: 'adding a static link names it by visible line order',
      run: () => core.addNavigationLink(cards, 0)[0],
      expect: { id: 'status', href: '/status', links: [{ label: '备用', href: 'https://backup.test' }, { label: '线路 2', href: '' }] }
    }
  ];

  for (const row of cases) {
    const got = row.run();
    const actual = { id: got.id, lineKind: got.lineKind, href: got.href, links: got.links };
    Object.keys(actual).forEach((key) => actual[key] === undefined && delete actual[key]);
    assert.deepEqual(actual, row.expect, row.name);
  }
});

test('site navigation save payload core strips empty runtime config without external calls', async () => {
  const core = await importTypeScript('ui/src/admin/siteNavigationConfigCore.ts');
  const config = {
    newapi: {
      sites: [
        { name: 'api', category: 'api', urls: [{ name: 'empty', url: '   ' }, { name: 'main', url: 'https://api.test' }] },
        { name: 'token', category: 'token', urls: [{ name: 'empty', url: '' }] }
      ]
    },
    navigation: {
      cards: [
        { title: 'No Target', stamp: 'N', accent: 'moss', href: '', links: [] },
        { title: 'Static', stamp: 'S', accent: 'clay', href: '/status', links: [{ label: 'empty', href: '' }, { label: 'docs', href: '/docs', ping: '/ping' }] },
        { title: 'Runtime', stamp: 'R', accent: 'stone', lineKind: 'api', href: '/old', links: [{ label: 'old', href: '/old' }] }
      ]
    }
  };

  const payload = core.buildAdminNavigationSavePayload(config);
  assert.deepEqual(payload.newapi.sites, [
    { name: 'api', category: 'api', urls: [{ name: 'main', url: 'https://api.test' }] }
  ]);
  assert.deepEqual(payload.navigation.cards, [
    { title: 'Static', stamp: 'S', accent: 'clay', lineKind: '', href: '/status', links: [{ label: 'docs', href: '/docs' }] },
    { title: 'Runtime', stamp: 'R', accent: 'stone', lineKind: 'api', href: '', links: [] }
  ]);
});
