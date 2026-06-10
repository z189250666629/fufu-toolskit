import test from 'node:test';
import assert from 'node:assert/strict';

import {
  renderEnvironment,
  renderHeader,
  renderMonitorPanel,
  renderPanelToggle
} from './app_shell.js';

test('renderHeader prefers model generated time and escapes metadata', () => {
  const html = renderHeader({
    modelStatus: { generatedAt: 1_735_689_600 },
    client: { serverTime: 1_735_689_600_000 }
  });

  assert.match(html, /fufu API 状态面板/);
  assert.match(html, /2025/);
  assert.match(html, /更新时间/);
});

test('renderPanelToggle marks the active panel tab', () => {
  const html = renderPanelToggle('models');

  assert.match(html, /aria-label="状态视图"/);
  assert.match(html, /data-panel="url"/);
  assert.match(html, /data-panel="models"/);
  assert.match(html, /style="--tab-count: 2; --active-tab-index: 1;"/);
});

test('renderMonitorPanel delegates to active panel renderer', () => {
  const urlHtml = renderMonitorPanel({
    state: {
      activePanel: 'url',
      connectivity: {
        mode: 'pending',
        tone: '',
        icon: '?',
        title: '等待测试',
        text: '准备',
        success: '-',
        testedAt: '-',
        running: false,
        results: [],
        progressText: '尚未开始',
        progress: 0,
        currentUrl: '-'
      },
      connectivityTargetError: 'targets failed'
    },
    groups: [{ id: 'api', name: 'API', urls: ['https://api.test'] }],
    panelMotionClass: ' motion-enter'
  });

  assert.match(urlHtml, /当前浏览器网络的 URL 连通性/);
  assert.match(urlHtml, /id="urlPanel"/);
  assert.match(urlHtml, /targets failed/);

  const modelHtml = renderMonitorPanel({
    state: {
      activePanel: 'models',
      loading: false,
      error: '',
      modelStatus: { configured: false, sites: [] },
      testingCells: new Set(),
      groupSelectOpen: false,
      modelFilter: ''
    },
    groups: [],
    panelMotionClass: '',
    scopeMotionClass: ' motion-enter'
  });

  assert.match(modelHtml, /已配置管理站点的模型可用性/);
  assert.match(modelHtml, /暂无模型状态数据/);
});

test('renderEnvironment renders provided browser context without globals', () => {
  const html = renderEnvironment({
    client: { ip: '<127.0.0.1>', serverTime: 1_735_689_600_000 },
    browserTime: '2026/06/10 10:00:00',
    online: true,
    timezone: 'Asia/Shanghai',
    networkType: '4g / 10Mbps'
  });

  assert.match(html, /访问环境/);
  assert.match(html, /&lt;127.0.0.1&gt;/);
  assert.match(html, /2026\/06\/10 10:00:00/);
  assert.match(html, /Asia\/Shanghai/);
  assert.match(html, /4g \/ 10Mbps/);
});

test('renderEnvironment surfaces client context load errors', () => {
  const html = renderEnvironment({
    client: null,
    clientLoadError: 'client endpoint <html>',
    browserTime: '2026/06/10 10:00:00',
    online: true,
    timezone: 'Asia/Shanghai',
    networkType: '4g'
  });

  assert.match(html, /role="alert"/);
  assert.match(html, /访问环境读取失败/);
  assert.match(html, /client endpoint &lt;html&gt;/);
  assert.match(html, /客户端 IP/);
});
