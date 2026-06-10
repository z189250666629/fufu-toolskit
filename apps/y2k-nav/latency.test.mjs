import test from 'node:test';
import assert from 'node:assert/strict';

import { measureLatency } from './latency.mjs';

test('latency probe marks timeout when fetch never settles', async () => {
  let timeoutCallback;
  let clearedTimer;
  let abortListener;
  class FakeAbortController {
    signal = {
      addEventListener(type, listener) {
        assert.equal(type, 'abort');
        abortListener = listener;
      }
    };

    abort() {
      abortListener();
    }
  }

  const pendingProbe = measureLatency('https://example.test', {
    fetchImpl: (_url, options) => new Promise((_resolve, reject) => {
      options.signal.addEventListener('abort', () => reject(new Error('aborted')));
    }),
    AbortControllerImpl: FakeAbortController,
    setTimeoutImpl: (callback, ms) => {
      assert.equal(ms, 3000);
      timeoutCallback = callback;
      return 'timer-1';
    },
    clearTimeoutImpl: (timer) => {
      clearedTimer = timer;
    },
    performance: { now: () => 100 },
    timeoutMs: 3000
  });

  timeoutCallback();
  const result = await pendingProbe;

  assert.deepEqual(result, { ok: false, text: '超时', className: 'latency bad', timedOut: true });
  assert.equal(clearedTimer, 'timer-1');
});
