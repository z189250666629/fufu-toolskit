import test from 'node:test';
import assert from 'node:assert/strict';

import {
  connectivityTagClass,
  renderConnectivityResults,
  renderUrlStatus
} from './render_connectivity.js';

const groups = [
  { id: 'api', name: 'API', urls: ['https://api-a.test', 'https://api-b.test'] }
];

test('connectivityTagClass maps statuses to tones', () => {
  assert.equal(connectivityTagClass('ok'), 'ok');
  assert.equal(connectivityTagClass('bad'), 'bad');
  assert.equal(connectivityTagClass('idle'), 'idle');
  assert.equal(connectivityTagClass('other'), 'idle');
});

test('renderConnectivityResults renders waiting and tested rows', () => {
  const html = renderConnectivityResults({
    results: [{ groupId: 'api', url: 'https://api-a.test', reachable: true, successRate: 1, averageMs: 120 }],
    groups
  });

  assert.match(html, /API/);
  assert.match(html, /可达 1\/2/);
  assert.match(html, /https:\/\/api-a.test/);
  assert.match(html, /最优/);
  assert.match(html, /https:\/\/api-b.test/);
  assert.match(html, /等待/);
});

test('renderUrlStatus renders progress and action state', () => {
  const html = renderUrlStatus({
    connectivity: {
      mode: 'running',
      tone: 'warn',
      icon: '...',
      title: '测试中',
      text: '正在测试',
      success: '-',
      testedAt: '-',
      running: true,
      results: [],
      progressText: '采样中',
      progress: 50,
      currentUrl: 'https://api-a.test'
    },
    groups,
    panelMotionClass: ' motion-enter'
  });

  assert.match(html, /url-monitor-grid motion-enter/);
  assert.match(html, /disabled/);
  assert.match(html, /采样中/);
});
