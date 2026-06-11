import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';

const source = await readFile(new URL('./index.html', import.meta.url), 'utf8');
const tokens = await readFile(new URL('./nav-ui-tokens.css', import.meta.url), 'utf8');

test('navigation page extracts UI tokens into a shared stylesheet', () => {
  assert.match(source, /<link\s+rel="stylesheet"\s+href="\.\/nav-ui-tokens\.css">/);
  assert.doesNotMatch(source, /--ink:\s*#2e312a/);
  assert.doesNotMatch(source, /--paper:\s*#f2f4ed/);
});

test('navigation UI tokens map the original nav language to fufu wabi tokens without Sage Design', () => {
  for (const marker of [
    'fufu-navigation-ui-tokens',
    ':root, .light',
    '[data-theme="dark"], .dark',
    '--fufu-paper: #f2f4ed',
    '--fufu-ink: #2e312a',
    '--fufu-moss: #6b7b55',
    '--fufu-clay: #b89578',
    '--fufu-stone: #8e9484',
    '--background: var(--fufu-paper)',
    '--foreground: var(--fufu-ink)',
    '--surface-secondary: var(--fufu-paper-warm)',
    '--border: var(--fufu-border)',
    '--accent: var(--fufu-moss)',
    '--focus: var(--fufu-moss)',
    '--radius: var(--fufu-radius)',
    '--font-family-display: \'Fraunces\'',
    '--fufu-nav-bg: var(--background)',
    '--fufu-nav-fg: var(--foreground)',
    '--fufu-nav-surface-muted: var(--surface-secondary)',
    '--fufu-nav-border: var(--border)',
    '--fufu-nav-accent-stone: var(--accent)',
    '--fufu-nav-card-hover-bg'
  ]) {
    assert.match(tokens, new RegExp(marker.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')));
  }
  assert.doesNotMatch(tokens, /--sage-/i);
  assert.doesNotMatch(tokens, /sage-design/i);
});
