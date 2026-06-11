import test from 'node:test';
import assert from 'node:assert/strict';
import { existsSync, readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));

const manifests = [
  {
    title: 'styles.css is a small ordered partial manifest',
    file: 'styles.css',
    importPattern: /@import url\("\.\/(styles\/[^\"]+)"\);/g,
    expected: [
      'styles/tokens.css',
      'styles/base.css',
      'styles/layout.css',
      'styles/connectivity.css',
      'styles/models.css',
      'styles/responsive.css'
    ]
  },
  {
    title: 'styles/models.css is an ordered model partial manifest',
    file: 'styles/models.css',
    importPattern: /@import url\("\.\/(models\/[^\"]+)"\);/g,
    prefix: 'styles/',
    maxPartialLines: 260,
    expected: [
      'styles/models/scope.css',
      'styles/models/overview.css',
      'styles/models/results.css',
      'styles/models/cards.css',
      'styles/models/table.css',
      'styles/models/environment.css'
    ]
  },
  {
    title: 'styles/layout.css is an ordered layout partial manifest',
    file: 'styles/layout.css',
    importPattern: /@import url\("\.\/(layout\/[^\"]+)"\);/g,
    prefix: 'styles/',
    maxPartialLines: 160,
    expected: [
      'styles/layout/shell.css',
      'styles/layout/header.css',
      'styles/layout/controls.css',
      'styles/layout/alerts.css',
      'styles/layout/tabs.css',
      'styles/layout/verdict.css'
    ]
  },
  {
    title: 'styles/responsive.css is an ordered responsive partial manifest',
    file: 'styles/responsive.css',
    importPattern: /@import url\("\.\/(responsive\/[^\"]+)"\);/g,
    prefix: 'styles/',
    maxPartialLines: 160,
    expected: [
      'styles/responsive/wide.css',
      'styles/responsive/mobile.css',
      'styles/responsive/motion.css'
    ]
  },
  {
    title: 'styles/models/scope.css is an ordered scope partial manifest',
    file: 'styles/models/scope.css',
    importPattern: /@import url\("\.\/(scope\/[^\"]+)"\);/g,
    prefix: 'styles/models/',
    maxPartialLines: 120,
    expected: [
      'styles/models/scope/panel.css',
      'styles/models/scope/tabs.css',
      'styles/models/scope/select-trigger.css',
      'styles/models/scope/select-listbox.css'
    ]
  },
  {
    title: 'styles/models/cards.css is an ordered cards partial manifest',
    file: 'styles/models/cards.css',
    importPattern: /@import url\("\.\/(cards\/[^\"]+)"\);/g,
    prefix: 'styles/models/',
    maxPartialLines: 90,
    expected: [
      'styles/models/cards/dashboard.css',
      'styles/models/cards/metrics.css',
      'styles/models/cards/sections.css',
      'styles/models/cards/site.css',
      'styles/models/cards/status.css'
    ]
  },
  {
    title: 'styles/models/results.css is an ordered results partial manifest',
    file: 'styles/models/results.css',
    importPattern: /@import url\("\.\/(results\/[^\"]+)"\);/g,
    prefix: 'styles/models/',
    maxPartialLines: 80,
    expected: [
      'styles/models/results/groups.css',
      'styles/models/results/rows.css',
      'styles/models/results/url-copy.css',
      'styles/models/results/fields.css',
      'styles/models/results/tags.css'
    ]
  },
  {
    title: 'styles/models/table.css is an ordered table partial manifest',
    file: 'styles/models/table.css',
    importPattern: /@import url\("\.\/(table\/[^\"]+)"\);/g,
    prefix: 'styles/models/',
    maxPartialLines: 80,
    expected: [
      'styles/models/table/wrappers.css',
      'styles/models/table/cells.css',
      'styles/models/table/base.css',
      'styles/models/table/sticky.css',
      'styles/models/table/states.css'
    ]
  },
  {
    title: 'styles/tokens.css is an ordered token partial manifest',
    file: 'styles/tokens.css',
    importPattern: /@import url\("\.\/(tokens\/[^\"]+)"\);/g,
    prefix: 'styles/',
    maxPartialLines: 130,
    expected: [
      'styles/tokens/light.css',
      'styles/tokens/dark.css',
      'styles/tokens/dark-overrides.css'
    ]
  }
];

function assertCSSManifest({ file, importPattern, prefix = '', expected, maxPartialLines }) {
  const css = readFileSync(join(here, file), 'utf8');
  const imports = [...css.matchAll(importPattern)].map((match) => `${prefix}${match[1]}`);

  assert.deepEqual(imports, expected);
  assert.ok(css.split(/\r?\n/).filter(Boolean).length <= expected.length + 1);

  for (const partial of expected) {
    const path = join(here, partial);
    assert.equal(existsSync(path), true, `${partial} should exist`);
    const lines = readFileSync(path, 'utf8').split(/\r?\n/).filter(Boolean);
    assert.ok(lines.length > 0, `${partial} should not be empty`);
    if (maxPartialLines) {
      assert.ok(lines.length <= maxPartialLines, `${partial} should stay focused`);
    }
  }
}

for (const manifest of manifests) {
  test(manifest.title, () => assertCSSManifest(manifest));
}