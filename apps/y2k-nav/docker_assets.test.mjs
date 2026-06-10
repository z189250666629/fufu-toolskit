import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';

const appDir = new URL('./', import.meta.url);
const indexHtml = await readFile(new URL('index.html', appDir), 'utf8');
const dockerfile = await readFile(new URL('Dockerfile', appDir), 'utf8');

function localModuleImports(html) {
  return [...html.matchAll(/<script\b[^>]*type=["']module["'][^>]*>([\s\S]*?)<\/script>/gi)]
    .flatMap(([, script]) => [...script.matchAll(/\bimport\s+(?:[\s\S]*?\s+from\s+)?["'](\.[^"']+)["']/g)])
    .map(([, specifier]) => specifier.replace(/^\.\//, ''))
    .filter((specifier) => !specifier.includes('/'));
}

function runtimeCopySources(file) {
  const runtimeStage = file.split(/\r?\nFROM\b/i).at(-1);
  return runtimeStage
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter((line) => line && !line.startsWith('#') && /^COPY\b/i.test(line))
    .flatMap((line) => {
      const parts = line.split(/\s+/).slice(1).filter((part) => !part.startsWith('--'));
      return parts.slice(0, -1);
    })
    .map((source) => source.replaceAll('\\', '/').replace(/^\.\//, '').replace(/\/$/, ''));
}

test('dockerfile copies all y2k module assets referenced by index', () => {
  const imports = localModuleImports(indexHtml);
  assert.deepEqual(imports, ['theme.mjs']);

  const sources = runtimeCopySources(dockerfile);
  for (const asset of imports) {
    const expected = `apps/y2k-nav/${asset}`;
    assert.ok(
      sources.some((source) => source === expected || expected.startsWith(`${source}/`)),
      `${expected} must be copied into the runtime image; COPY sources were ${JSON.stringify(sources)}`
    );
  }
});
