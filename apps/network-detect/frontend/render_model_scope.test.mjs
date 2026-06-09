import test from 'node:test';
import assert from 'node:assert/strict';

import {
  renderModelScopeControls,
  renderTokenGroupSelect
} from './render_model_scope.js';

test('renderTokenGroupSelect escapes selected open options', () => {
  const html = renderTokenGroupSelect(['vip', '<default>'], '<default>', { groupSelectOpen: true });

  assert.match(html, /aria-expanded="true"/);
  assert.match(html, /data-token-group-option="&lt;default&gt;"/);
  assert.doesNotMatch(html, /data-token-group-option="<default>"/);
  assert.match(html, /aria-selected="true"/);
});

test('renderModelScopeControls renders active site tabs and token group slot', () => {
  const html = renderModelScopeControls(
    {
      sites: [
        { site: { name: 'site-a' } },
        { site: { name: 'token-fufu' } }
      ]
    },
    { siteName: 'token-fufu', groups: ['vip'], group: 'vip' },
    { groupSelectOpen: true }
  );

  assert.match(html, /role="tablist"/);
  assert.match(html, /data-model-site="token-fufu"/);
  assert.match(html, /aria-selected="true"/);
  assert.match(html, /data-token-group-option="vip"/);
});