import test from 'node:test';
import assert from 'node:assert/strict';
import { existsSync, readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));

test('styles.css is a small ordered partial manifest', () => {
  const expected = [
    'styles/tokens.css',
    'styles/base.css',
    'styles/layout.css',
    'styles/connectivity.css',
    'styles/models.css',
    'styles/responsive.css'
  ];
  const css = readFileSync(join(here, 'styles.css'), 'utf8');
  const imports = [...css.matchAll(/@import url\("\.\/(styles\/[^"]+)"\);/g)].map((match) => match[1]);

  assert.deepEqual(imports, expected);
  assert.ok(css.split(/\r?\n/).filter(Boolean).length <= expected.length + 1);

  for (const partial of expected) {
    const path = join(here, partial);
    assert.equal(existsSync(path), true, `${partial} should exist`);
    assert.ok(readFileSync(path, 'utf8').trim().length > 0, `${partial} should not be empty`);
  }
});

test('styles/models.css is an ordered model partial manifest', () => {
  const expected = [
    'styles/models/scope.css',
    'styles/models/overview.css',
    'styles/models/results.css',
    'styles/models/cards.css',
    'styles/models/table.css',
    'styles/models/environment.css'
  ];
  const css = readFileSync(join(here, 'styles/models.css'), 'utf8');
  const imports = [...css.matchAll(/@import url\("\.\/(models\/[^"]+)"\);/g)]
    .map((match) => `styles/${match[1]}`);

  assert.deepEqual(imports, expected);
  assert.ok(css.split(/\r?\n/).filter(Boolean).length <= expected.length + 1);

  for (const partial of expected) {
    const path = join(here, partial);
    assert.equal(existsSync(path), true, `${partial} should exist`);
    const lines = readFileSync(path, 'utf8').split(/\r?\n/).filter(Boolean);
    assert.ok(lines.length > 0, `${partial} should not be empty`);
    assert.ok(lines.length <= 260, `${partial} should stay focused`);
  }
});

test('styles/layout.css is an ordered layout partial manifest', () => {
  const expected = [
    'styles/layout/shell.css',
    'styles/layout/header.css',
    'styles/layout/controls.css',
    'styles/layout/alerts.css',
    'styles/layout/tabs.css',
    'styles/layout/verdict.css'
  ];
  const css = readFileSync(join(here, 'styles/layout.css'), 'utf8');
  const imports = [...css.matchAll(/@import url\("\.\/(layout\/[^"]+)"\);/g)]
    .map((match) => `styles/${match[1]}`);

  assert.deepEqual(imports, expected);
  assert.ok(css.split(/\r?\n/).filter(Boolean).length <= expected.length + 1);

  for (const partial of expected) {
    const path = join(here, partial);
    assert.equal(existsSync(path), true, `${partial} should exist`);
    const lines = readFileSync(path, 'utf8').split(/\r?\n/).filter(Boolean);
    assert.ok(lines.length > 0, `${partial} should not be empty`);
    assert.ok(lines.length <= 160, `${partial} should stay focused`);
  }
});
