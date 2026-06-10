export function latencyClass(ms) {
  if (ms < 200) return 'latency good';
  if (ms < 500) return 'latency slow';
  return 'latency bad';
}

export async function measureLatency(url, {
  fetchImpl = fetch,
  AbortControllerImpl = AbortController,
  setTimeoutImpl = setTimeout,
  clearTimeoutImpl = clearTimeout,
  performance = globalThis.performance,
  timeoutMs = 5000
} = {}) {
  const controller = new AbortControllerImpl();
  const start = performance.now();
  let timedOut = false;
  const timer = setTimeoutImpl(() => {
    timedOut = true;
    controller.abort();
  }, timeoutMs);

  try {
    await fetchImpl(url, {
      method: 'HEAD',
      mode: 'no-cors',
      cache: 'no-store',
      signal: controller.signal
    });
    const ms = Math.round(performance.now() - start);
    return { ok: true, text: `${ms}ms`, className: latencyClass(ms), ms };
  } catch {
    return { ok: false, text: timedOut ? '超时' : '失败', className: 'latency bad', timedOut };
  } finally {
    clearTimeoutImpl(timer);
  }
}

export function applyLatencyResult(element, result) {
  if (!element) return;
  element.textContent = result.text;
  element.className = result.className;
}

export function initLatencyProbes({
  document = globalThis.document,
  measure = measureLatency
} = {}) {
  document.querySelectorAll('a[data-ping]').forEach((anchor) => {
    const url = anchor.getAttribute('data-ping');
    const element = anchor.querySelector('.latency');
    Promise.resolve()
      .then(() => measure(url))
      .then((result) => applyLatencyResult(element, result))
      .catch(() => applyLatencyResult(element, {
        ok: false,
        text: '失败',
        className: 'latency bad'
      }));
  });
}
