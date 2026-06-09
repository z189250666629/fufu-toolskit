import {
  escapeHtml,
  formatServerTime,
  formatTime
} from './utils.js';
import {
  renderUrlStatus
} from './render_connectivity.js';
import {
  renderModelAvailability
} from './render_models.js';
import {
  renderSegmentedTabs
} from './render_tabs.js';

export function renderHeader({ modelStatus, client }) {
  const generatedAt = modelStatus?.generatedAt
    ? formatTime(modelStatus.generatedAt)
    : formatServerTime(client?.serverTime);
  return `
    <header class="app-header">
      <div class="brand-block">
        <div class="brand-mark">API</div>
        <div>
          <h1>fufu API 状态面板</h1>
          <p>固定展示 Base URL 连通性和管理站模型可用性</p>
        </div>
      </div>
      <div class="header-meta">
        <span>更新时间</span>
        <b>${escapeHtml(generatedAt)}</b>
      </div>
    </header>
  `;
}

export function renderPanelToggle(activePanel) {
  const options = [
    { value: 'url', label: 'URL 检测' },
    { value: 'models', label: '模型状态' }
  ];

  return renderSegmentedTabs({
    className: 'panel-toggle tabs__list',
    ariaLabel: '状态视图',
    motionKey: 'panel',
    activeValue: activePanel,
    options,
    buttonClassName: 'toggle-button tabs__tab',
    getControls: (value) => `${value}Panel`,
    getId: (value) => `${value}Tab`,
    dataAttribute: 'panel'
  });
}

export function renderMonitorPanel({
  state,
  groups,
  panelMotionClass = '',
  scopeMotionClass = ''
}) {
  return `
    <section class="monitor-panel" data-slot="card">
      <div class="tabs panel-tabs" data-slot="tabs" data-orientation="horizontal">
        <div class="monitor-head" data-slot="card-header">
          <div>
            <h2 data-slot="card-title">状态面板</h2>
            <p data-slot="card-description">${state.activePanel === 'url' ? '当前浏览器网络的 URL 连通性' : '已配置管理站点的模型可用性'}</p>
          </div>
          ${renderPanelToggle(state.activePanel)}
        </div>
        <div class="monitor-content" data-slot="card-content">
          ${state.activePanel === 'url' ? renderUrlStatus({ connectivity: state.connectivity, groups, panelMotionClass }) : renderModelAvailability({ state, panelMotionClass, scopeMotionClass })}
        </div>
      </div>
    </section>
  `;
}

export function renderEnvironment({
  client,
  browserTime,
  online,
  timezone,
  networkType
}) {
  return `
    <section class="section environment-section">
      <div class="section-head">
        <h2>访问环境</h2>
        <span>客户端与服务端</span>
      </div>
      <div class="info-list">
        <div><span>客户端 IP</span><b>${escapeHtml(client?.ip || '-')}</b></div>
        <div><span>服务器时间</span><b>${escapeHtml(formatServerTime(client?.serverTime))}</b></div>
        <div><span>浏览器时间</span><b>${escapeHtml(browserTime)}</b></div>
        <div><span>浏览器在线</span><b>${online ? '是' : '否'}</b></div>
        <div><span>时区</span><b>${escapeHtml(timezone || '未知')}</b></div>
        <div><span>网络类型</span><b>${escapeHtml(networkType)}</b></div>
      </div>
    </section>
  `;
}
