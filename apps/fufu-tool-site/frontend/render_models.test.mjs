import test from 'node:test';
import assert from 'node:assert/strict';

import {
  manualTestRowClass,
  renderModelAvailability,
  renderModelTestAction,
  renderTokenGroupSelect
} from './render_models.js';
import { modelCellKey } from './model_test_runner.js';

test('renderModelTestAction reflects testing state and cooldown', () => {
  const key = modelCellKey('site', 'model-a', 'vip');
  const html = renderModelTestAction(
    { siteName: 'site', model: 'model-a', groups: ['vip'] },
    { testingCells: new Set([key]) }
  );

  assert.match(html, /disabled/);
  assert.match(html, /测试中/);
  assert.match(html, /data-group="vip"/);
});

test('manualTestRowClass maps manual result tone', () => {
  assert.equal(manualTestRowClass({ manualTestTone: 'ok' }), 'is-manual-ok');
  assert.equal(manualTestRowClass({ manualTestTone: 'bad' }), 'is-manual-bad');
  assert.equal(manualTestRowClass({}), '');
});

test('renderTokenGroupSelect opens selected group list', () => {
  const html = renderTokenGroupSelect(['vip', 'default'], 'vip', { groupSelectOpen: true });
  assert.match(html, /aria-expanded="true"/);
  assert.match(html, /data-token-group-option="vip"/);
  assert.match(html, /aria-selected="true"/);
});

test('renderModelAvailability handles empty configured state', () => {
  const html = renderModelAvailability({
    state: {
      loading: false,
      error: '',
      modelStatus: { configured: false, configError: 'missing config', sites: [] },
      testingCells: new Set(),
      groupSelectOpen: false,
      modelFilter: ''
    }
  });

  assert.match(html, /暂无模型状态数据/);
  assert.match(html, /missing config/);
});

test('renderModelAvailability distinguishes load failure from unconfigured model status', () => {
  const html = renderModelAvailability({
    state: {
      loading: false,
      error: 'model status upstream <html>',
      modelStatus: null,
      testingCells: new Set(),
      groupSelectOpen: false,
      modelFilter: ''
    }
  });

  assert.match(html, /模型状态加载失败/);
  assert.match(html, /model status upstream &lt;html&gt;/);
  assert.doesNotMatch(html, /暂无模型状态数据/);
  assert.doesNotMatch(html, /未配置/);
});

test('renderModelAvailability surfaces pricing errors on site card', () => {
  const html = renderModelAvailability({
    state: {
      loading: false,
      error: '',
      modelFilter: '',
      testingCells: new Set(),
      groupSelectOpen: false,
      modelStatus: {
        configured: true,
        windowSeconds: 3600,
        expiresAt: 0,
        sites: [{
          site: { name: 'token-fufu', url: 'https://token.example.test' },
          groups: ['vip'],
          pricingError: 'pricing failed <script>'
        }],
        models: [{
          model: 'model-a',
          perSite: {
            'token-fufu': {
              configured: true,
              groupStats: {
                vip: {
                  configured: true,
                  status: 'operational',
                  requestCount: 1,
                  successCount: 1,
                  failureCount: 0,
                  enabledChannelCount: 1
                }
              }
            }
          }
        }]
      }
    }
  });

  assert.match(html, /pricing failed &lt;script&gt;/);
  assert.doesNotMatch(html, /pricing failed <script>/);
});

test('renderModelAvailability hides raw managed site URLs', () => {
  const html = renderModelAvailability({
    state: {
      loading: false,
      error: '',
      modelFilter: '',
      testingCells: new Set(),
      groupSelectOpen: false,
      modelStatus: {
        configured: true,
        windowSeconds: 3600,
        expiresAt: 0,
        sites: [{
          site: {
            name: 'private-site',
            displayUrl: '地址已隐藏',
            url: 'http://10.0.0.5:3000/admin'
          },
          groups: ['vip']
        }],
        models: [{
          model: 'model-a',
          perSite: {
            'private-site': {
              configured: true,
              groupStats: {
                vip: {
                  configured: true,
                  status: 'operational',
                  requestCount: 1,
                  successCount: 1,
                  failureCount: 0,
                  enabledChannelCount: 1
                }
              }
            }
          }
        }]
      }
    }
  });

  assert.match(html, /地址已隐藏/);
  assert.doesNotMatch(html, /10\.0\.0\.5/);
  assert.doesNotMatch(html, /http:\/\/10\.0\.0\.5:3000\/admin/);
});
