import test from 'node:test';
import assert from 'node:assert/strict';

import { summarizeProbeAttempts } from './connectivity_probe.js';

test('summarizeProbeAttempts reports partial success averages and last error', () => {
  const summary = summarizeProbeAttempts([
    { ok: false, ms: 120, error: 'timeout' },
    { ok: true, ms: 80, error: '' },
    { ok: true, ms: 40, error: '' }
  ]);

  assert.deepEqual(summary, {
    ok: true,
    successRate: 2 / 3,
    averageMs: 60,
    lastError: ''
  });
});

test('summarizeProbeAttempts reports all failures with last error', () => {
  const summary = summarizeProbeAttempts([
    { ok: false, ms: 100, error: 'timeout' },
    { ok: false, ms: 120, error: 'network down' }
  ]);

  assert.deepEqual(summary, {
    ok: false,
    successRate: 0,
    averageMs: null,
    lastError: 'network down'
  });
});
