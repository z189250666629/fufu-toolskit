import test from 'node:test';
import assert from 'node:assert/strict';

test('renderTokenSections escapes tokens and keeps delete action role-gated', async () => {
  const { renderTokenSections } = await import('./combine_token_sections.js');
  const tokens = [
    {
      id: 1,
      key: 'sk-valid',
      name: '<Valid>',
      group: 'A & B',
      interval_unit: 8,
      remain_quota: 1000000,
      status: 1
    },
    {
      id: 2,
      key: 'sk-bad"><script>alert(1)</script>',
      name: '<Disabled>',
      group: 'X "Y"',
      interval_unit: 3,
      remain_quota: 0,
      status: 2
    },
    {
      id: 3,
      key: 'sk-empty',
      name: 'Empty',
      group: '<none>',
      interval_unit: 9,
      remain_quota: 0,
      status: 1
    }
  ];

  const userHtml = renderTokenSections(tokens, {
    userRole: 'user',
    quotaUnit: 500000,
    unitNames: { 3: '天卡', 8: '周不刷新卡', 9: '月不刷新卡' }
  });
  assert.match(userHtml, /可合成 \(1\)/);
  assert.match(userHtml, /不可合成 \(2\)/);
  assert.match(userHtml, /&lt;Valid&gt;/);
  assert.match(userHtml, /A &amp; B/);
  assert.match(userHtml, /X &quot;Y&quot;/);
  assert.doesNotMatch(userHtml, /<script>/);
  assert.match(userHtml, /deleteDisabledToken\(2, &quot;sk-bad\\&quot;&gt;&lt;script&gt;alert\(1\)&lt;\/script&gt;&quot;\)/);
  assert.match(userHtml, />删除<\/button>/);
  assert.doesNotMatch(userHtml, /deleteDisabledToken\(3,/);

  const guestHtml = renderTokenSections(tokens, {
    userRole: 'guest',
    quotaUnit: 500000,
    unitNames: { 3: '天卡', 8: '周不刷新卡', 9: '月不刷新卡' }
  });
  assert.doesNotMatch(guestHtml, /deleteDisabledToken/);
  assert.doesNotMatch(guestHtml, />删除<\/button>/);
});
