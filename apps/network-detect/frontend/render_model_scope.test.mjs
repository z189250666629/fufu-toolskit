import test from 'node:test';
import assert from 'node:assert/strict';

import {
  renderLockedTokenGroupValue,
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
  assert.match(html, /data-token-group-static/);
  assert.match(html, /aria-readonly="true"/);
  assert.doesNotMatch(html, /data-token-group-option="vip"/);
});

test('renderModelScopeControls renders group select for custom grouped sites', () => {
  const html = renderModelScopeControls(
    {
      sites: [
        { site: { name: 'custom-site' } },
        { site: { name: 'token-fufu' } }
      ]
    },
    { siteName: 'custom-site', groups: ['vip', 'default'], group: 'vip' },
    { groupSelectOpen: true }
  );

  assert.match(html, /data-model-site="custom-site"/);
  assert.match(html, /data-token-group-option="default"/);
  assert.doesNotMatch(html, /is-placeholder/);
});

test('renderLockedTokenGroupValue keeps fixed groups non-interactive', () => {
  const html = renderLockedTokenGroupValue('mix');

  assert.match(html, /data-token-group-static/);
  assert.match(html, /mix/);
  assert.doesNotMatch(html, /button/);
  assert.doesNotMatch(html, /aria-haspopup/);
});
