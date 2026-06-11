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

function localStylesheets(html) {
  return [...html.matchAll(/<link\b[^>]*rel=["']stylesheet["'][^>]*href=["'](\.[^"']+)["'][^>]*>/gi)]
    .map(([, specifier]) => specifier.replace(/^\.\//, ''))
    .filter((specifier) => !specifier.includes('/'));
}

function runtimeCopySources(file) {
  return runtimeCopyInstructions(file).flatMap(({ sources }) => sources);
}

function runtimeCopyInstructions(file) {
  const runtimeStage = file.split(/\r?\nFROM\b/i).at(-1);
  return runtimeStage
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter((line) => line && !line.startsWith('#') && /^COPY\b/i.test(line))
    .map((line) => {
      const parts = line.split(/\s+/).slice(1).filter((part) => !part.startsWith('--'));
      return { sources: parts.slice(0, -1), destination: parts.at(-1) ?? '' };
    })
    .map(({ sources, destination }) => ({
      sources: sources.map(normalizeDockerPath),
      destination: normalizeDockerPath(destination)
    }));
}

function normalizeDockerPath(value) {
  return value.replaceAll('\\', '/').replace(/^\.\//, '').replace(/\/$/, '');
}

test('dockerfile copies all y2k module assets referenced by index', () => {
  const imports = [...localModuleImports(indexHtml), ...localStylesheets(indexHtml)];
  assert.ok(imports.length > 0, 'expected index.html to reference local browser assets');

  const sources = runtimeCopySources(dockerfile);
  for (const asset of imports) {
    const expected = `apps/y2k-nav/${asset}`;
    assert.ok(
      sources.some((source) => source === expected || expected.startsWith(`${source}/`)),
      `${expected} must be copied into the runtime image; COPY sources were ${JSON.stringify(sources)}`
    );
  }
});

test('dockerfile keeps server binary outside the static document root', () => {
  const instructions = runtimeCopyInstructions(dockerfile);
  const binaryCopies = instructions.filter(({ sources }) => sources.includes('/out/y2k-nav'));
  assert.ok(binaryCopies.length > 0, 'expected runtime image to copy the built y2k-nav binary');
  for (const copy of binaryCopies) {
    assert.notEqual(copy.destination, '/app/y2k-nav', 'server binary must not be copied into the /app static root');
    assert.notEqual(copy.destination, 'y2k-nav', 'server binary must not be copied into the relative static root');
  }
});
