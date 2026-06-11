import test from 'node:test';
import assert from 'node:assert/strict';

import { buildConnectivityGroupView } from './connectivity_result_view.js';

test('buildConnectivityGroupView maps waiting and best reachable rows', () => {
  const view = buildConnectivityGroupView(
    { id: 'api', name: 'API', urls: ['https://a.test', 'https://b.test', 'https://c.test'] },
    [
      { groupId: 'api', url: 'https://a.test', reachable: true, successRate: 1, averageMs: 120 },
      { groupId: 'api', url: 'https://b.test', reachable: true, successRate: 0.5, averageMs: 80 },
      { groupId: 'other', url: 'https://c.test', reachable: true, successRate: 1, averageMs: 10 }
    ]
  );

  assert.equal(view.title, 'API');
  assert.equal(view.description, '可达 2/3');
  assert.deepEqual(view.rows.map((row) => [row.url, row.status, row.label, row.starred]), [
    ['https://a.test', 'ok', '可达', false],
    ['https://b.test', 'ok', '可达', true],
    ['https://c.test', 'idle', '等待', false]
  ]);
});