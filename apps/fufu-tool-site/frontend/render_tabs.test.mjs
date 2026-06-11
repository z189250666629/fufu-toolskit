import test from 'node:test';
import assert from 'node:assert/strict';

import {
  renderSegmentedTabs
} from './render_tabs.js';

test('renderSegmentedTabs renders escaped tablist markup with active metadata', () => {
  const html = renderSegmentedTabs({
    className: 'panel-toggle tabs__list',
    ariaLabel: '状态视图',
    motionKey: 'panel',
    activeValue: 'models',
    options: [
      { value: 'url', label: 'URL <检测>' },
      { value: 'models', label: '模型状态' }
    ],
    buttonClassName: 'toggle-button tabs__tab',
    getControls: (value) => `${value}Panel`,
    getId: (value) => `${value}Tab`,
    dataAttribute: 'panel'
  });

  assert.match(html, /class="panel-toggle tabs__list"/);
  assert.match(html, /aria-label="状态视图"/);
  assert.match(html, /data-tab-motion-key="panel"/);
  assert.match(html, /style="--tab-count: 2; --active-tab-index: 1;"/);
  assert.match(html, /data-panel="models"/);
  assert.match(html, /aria-selected="true"/);
  assert.match(html, /URL &lt;检测&gt;/);
  assert.doesNotMatch(html, /URL <检测>/);
});

test('renderSegmentedTabs falls back to first option when active value is missing', () => {
  const html = renderSegmentedTabs({
    className: 'panel-toggle',
    ariaLabel: '状态视图',
    motionKey: 'panel',
    activeValue: 'missing',
    options: [
      { value: 'url', label: 'URL 检测' },
      { value: 'models', label: '模型状态' }
    ],
    buttonClassName: 'toggle-button',
    getControls: (value) => `${value}Panel`,
    getId: (value) => `${value}Tab`,
    dataAttribute: 'panel'
  });

  assert.match(html, /style="--tab-count: 2; --active-tab-index: 0;"/);
  assert.match(html, /aria-selected="true"[\s\S]*?tabindex="0"[\s\S]*?data-panel="url"/);
  assert.match(html, /aria-selected="false"[\s\S]*?data-panel="models"/);
});
