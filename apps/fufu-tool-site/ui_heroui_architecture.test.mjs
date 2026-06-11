import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile, readdir } from 'node:fs/promises';
import { join } from 'node:path';

const appRoot = new URL('.', import.meta.url);

async function readText(path) {
  return readFile(new URL(path, appRoot), 'utf8');
}

async function readSourceTree(dir) {
  const root = new URL(dir, appRoot);
  const entries = await readdir(root, { withFileTypes: true });
  const parts = [];
  for (const entry of entries) {
    const child = join(dir, entry.name).replaceAll('\\', '/');
    if (entry.isDirectory()) {
      parts.push(await readSourceTree(`${child}/`));
    } else if (/\.(tsx|ts|css|html)$/.test(entry.name)) {
      parts.push(await readText(child));
    }
  }
  return parts.join('\n');
}

test('tool-site frontend declares a real React + HeroUI build chain', async () => {
  const pkg = JSON.parse(await readText('package.json'));
  for (const dependency of [
    '@heroui/react',
    '@heroui/styles',
    '@vitejs/plugin-react',
    'vite',
    'typescript',
    'react',
    'react-dom',
    'tailwindcss'
  ]) {
    assert.ok(
      pkg.dependencies?.[dependency] || pkg.devDependencies?.[dependency],
      `missing ${dependency} dependency`
    );
  }
  for (const script of ['dev:ui', 'build:ui', 'typecheck:ui']) {
    assert.ok(pkg.scripts?.[script], `missing ${script} script`);
  }
});

test('tool-site UI uses HeroUI components and theme hook instead of static slot shims', async () => {
  const source = await readSourceTree('ui/src/');
  assert.match(source, /from ['"]@heroui\/react['"]/);
  assert.match(source, /useTheme/);
  for (const component of ['Button', 'Card', 'Tabs', 'Input', 'Textarea', 'Table', 'Chip']) {
    assert.match(source, new RegExp(`\\b${component}\\b`), `missing HeroUI ${component} usage`);
  }
  assert.doesNotMatch(source, /<[A-Za-z][^>]*\sdata-slot=/);
});

test('tool-site UI imports HeroUI styles and keeps a blueprint-design theme without Sage or wabi tokens', async () => {
  const styles = await readSourceTree('ui/src/');
  assert.match(styles, /@import ['"]@heroui\/styles['"]/);
  assert.match(styles, /--blueprint-canvas/);
  assert.match(styles, /--blueprint-panel/);
  assert.match(styles, /--blueprint-accent/);
  assert.match(styles, /--blueprint-grid-line/);
  assert.doesNotMatch(styles, /--sage-/i);
  assert.doesNotMatch(styles, /sage-design/i);
  assert.doesNotMatch(styles, /--fufu-/i);
  assert.doesNotMatch(styles, /wabi/i);
});

test('tool-site blueprint theme centralizes HeroUI-compatible tokens and radius decisions', async () => {
  const styles = await readText('ui/src/styles.css');
  for (const token of [
    '--background',
    '--foreground',
    '--surface',
    '--surface-secondary',
    '--border',
    '--separator',
    '--focus',
    '--field-background',
    '--radius',
    '--blueprint-canvas',
    '--blueprint-panel',
    '--blueprint-text-primary',
    '--blueprint-text-muted',
    '--blueprint-accent',
    '--blueprint-grid-line',
    '--blueprint-radius-control',
    '--blueprint-radius-panel',
    '--blueprint-radius-stamp',
    '--blueprint-radius-nav'
  ]) {
    assert.match(styles, new RegExp(`${token.replaceAll('-', '\\-')}\\s*:`), `missing token ${token}`);
  }

  const radiusDeclarations = [...styles.matchAll(/border-radius:\s*([^;]+);/g)].map((match) => match[1].trim());
  assert.ok(radiusDeclarations.length > 8, 'expected component radius declarations to be explicit');
  for (const value of radiusDeclarations) {
    assert.match(value, /^var\(--blueprint-radius-[^)]+\)(?:\s*!important)?$/, `raw radius declaration is not tokenized: ${value}`);
  }

  for (const [token, value] of [
    ['--blueprint-radius-control', '2px'],
    ['--blueprint-radius-panel', '2px'],
    ['--blueprint-radius-stamp', '1px'],
    ['--blueprint-radius-nav', '2px']
  ]) {
    assert.match(styles, new RegExp(`${token.replaceAll('-', '\\-')}\\s*:\\s*${value.replace('.', '\\.')};`), `unexpected ${token}`);
  }
});

test('tool-site navigation and admin share blueprint-design primitives', async () => {
  const source = await readSourceTree('ui/src/');
  for (const marker of [
    './blueprint',
    '../blueprint',
    'BlueprintHeader',
    'BlueprintStamp',
    'blueprint-page',
    'blueprint-top-actions',
    'blueprint-primary-button'
  ]) {
    assert.match(source, new RegExp(marker.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')), `missing shared blueprint marker ${marker}`);
  }
  assert.doesNotMatch(source, /Wabi|wabi/);
});

test('tool-site admin layout keeps business tabs primary and utility actions secondary', async () => {
  const adminSource = await readText('ui/src/admin/AdminPage.tsx');
  const styles = await readText('ui/src/styles.css');

  assert.match(adminSource, /className="admin-utility-bar"/, 'admin actions should live in a lightweight utility bar');
  assert.match(adminSource, /className="admin-action-group"/, 'admin action buttons should be visually grouped as utilities');
  assert.doesNotMatch(adminSource, /orientation="vertical"/, 'business tabs should not use the old left vertical rail');
  assert.doesNotMatch(adminSource, /className="command-bar"/, 'old full-width command bar should not remain');
  assert.doesNotMatch(styles, /grid-template-columns:\s*300px\s+minmax\(0,\s*1fr\)/, 'admin tabs should not reserve a left rail');
  assert.doesNotMatch(styles, /\.admin-tab-list\s*\{[^}]*position:\s*sticky/s, 'tab list should not be a sticky sidebar');
});

test('tool-site UI preserves actual business API wiring in React source', async () => {
  const source = await readSourceTree('ui/src/');
  for (const marker of [
    '/api/admin/session',
    '/api/admin/config',
    '/api/admin/stats',
    '/api/admin/sale-cards/config',
    '/api/admin/sale-cards/run',
    '/api/prizes',
    '/api/newapi/sites'
  ]) {
    assert.match(source, new RegExp(marker.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')));
  }
  assert.doesNotMatch(source, /Authorization:\s*['"`]Bearer/);
  assert.doesNotMatch(source, /ADMIN_TOKEN/);
});
