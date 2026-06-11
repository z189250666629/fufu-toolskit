import test from 'node:test';
import assert from 'node:assert/strict';

import { createDashboardApp } from './app_controller.js';

function createDeferred() {
  let resolve;
  const promise = new Promise((done) => {
    resolve = done;
  });
  return { promise, resolve };
}

async function flushMicrotasks(count = 6) {
  for (let index = 0; index < count; index += 1) {
    await Promise.resolve();
  }
}

function createDomHarness() {
  const appElement = {
    innerHTML: '',
    querySelectorAll: () => []
  };
  const listeners = [];
  const documentRef = {
    title: '',
    getElementById: (id) => (id === 'app' ? appElement : null),
    querySelector: () => null,
    querySelectorAll: () => [],
    addEventListener: (type, handler) => listeners.push([type, handler])
  };
  return { appElement, documentRef, listeners };
}

test('boot renders shell before loading data and schedules connectivity once', async () => {
  const { appElement, documentRef, listeners } = createDomHarness();
  const client = createDeferred();
  const targets = createDeferred();
  const modelStatus = createDeferred();
  const calls = [];
  const timeouts = [];
  const intervals = [];

  const app = createDashboardApp({
    documentRef,
    windowRef: {
      scrollX: 0,
      scrollY: 0,
      scrollTo: () => {},
      setTimeout: (handler, delay) => {
        timeouts.push({ handler, delay });
        return timeouts.length;
      },
      clearTimeout: () => {},
      setInterval: (handler, delay) => {
        intervals.push({ handler, delay });
        return intervals.length;
      },
      requestAnimationFrame: (callback) => callback()
    },
    navigatorRef: { onLine: true },
    fetchJsonImpl: (url) => {
      calls.push(url);
      if (url === '/api/client') return client.promise;
      if (url === '/api/connectivity/targets') return targets.promise;
      if (url === '/api/newapi/model-status') return modelStatus.promise;
      throw new Error(`unexpected URL ${url}`);
    },
    postJsonImpl: async () => ({}),
    now: () => new Date('2026-06-10T10:00:00+08:00')
  });

  const bootPromise = app.boot();

  assert.match(appElement.innerHTML, /fufu API 状态面板/);
  assert.match(appElement.innerHTML, /当前浏览器网络的 URL 连通性/);
  assert.deepEqual(calls, ['/api/client', '/api/connectivity/targets']);
  assert.deepEqual(timeouts, []);
  assert.equal(documentRef.title, 'fufu API 状态面板');

  client.resolve({ ip: '127.0.0.1', serverTime: 1_735_689_600_000 });
  targets.resolve({ groups: [{ id: 'api', name: 'API', urls: ['https://api.example.test'] }] });
  await flushMicrotasks();

  assert.deepEqual(calls, ['/api/client', '/api/connectivity/targets', '/api/newapi/model-status']);
  assert.equal(timeouts.length, 1);
  assert.equal(timeouts[0].delay, 180);
  assert.match(appElement.innerHTML, /https:\/\/api\.example\.test/);

  modelStatus.resolve({ configured: false, sites: [] });
  await bootPromise;

  assert.equal(timeouts.length, 1);
  assert.deepEqual(intervals.map((timer) => timer.delay), [10 * 60 * 1000, 60 * 1000]);
  assert.deepEqual(listeners.map(([type]) => type), ['pointerdown', 'keydown']);
});

test('boot surfaces model status load failure while URL panel is active', async () => {
  const { appElement, documentRef } = createDomHarness();
  const timeouts = [];

  const app = createDashboardApp({
    documentRef,
    windowRef: {
      scrollX: 0,
      scrollY: 0,
      scrollTo: () => {},
      setTimeout: (handler, delay) => {
        timeouts.push({ handler, delay });
        return timeouts.length;
      },
      clearTimeout: () => {},
      setInterval: () => 1,
      requestAnimationFrame: (callback) => callback()
    },
    navigatorRef: { onLine: true },
    fetchJsonImpl: async (url) => {
      if (url === '/api/client') return { ip: '127.0.0.1' };
      if (url === '/api/connectivity/targets') return { groups: [] };
      if (url === '/api/newapi/model-status') throw new Error('model status returned HTML');
      throw new Error(`unexpected URL ${url}`);
    },
    postJsonImpl: async () => ({}),
    now: () => new Date('2026-06-10T10:00:00+08:00')
  });

  await app.boot();

  assert.equal(app.state.activePanel, 'url');
  assert.match(appElement.innerHTML, /id="urlPanel"/);
  assert.match(appElement.innerHTML, /模型状态加载失败/);
  assert.match(appElement.innerHTML, /model status returned HTML/);
});

test('activatePanelTab applies panel motion during render', () => {
  const { appElement, documentRef } = createDomHarness();
  const app = createDashboardApp({
    documentRef,
    windowRef: {
      scrollX: 0,
      scrollY: 0,
      scrollTo: () => {},
      requestAnimationFrame: (callback) => callback()
    },
    navigatorRef: { onLine: true },
    loadModelStatusStateImpl: async () => {},
    now: () => new Date('2026-06-10T10:00:00+08:00')
  });

  app.render();
  app.activatePanelTab('models');

  assert.match(appElement.innerHTML, /model-state-empty motion-enter/);
});
