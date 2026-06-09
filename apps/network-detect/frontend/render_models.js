import {
  compactNumber,
  escapeHtml,
  formatNullableRate,
  formatShortTime,
  formatWindow,
  modelScopeTabId,
  modelSiteDisplayName,
  modelStatusMeta
} from './utils.js';
import {
  activeModelScope,
  scopedModelRows,
  scopedSummary
} from './model_selectors.js';
import {
  renderAlert,
  renderChip,
  renderMetric,
  renderStateCard,
} from './render_components.js';
import {
  renderModelTableRows
} from './render_model_table.js';
import {
  renderModelScopeControls
} from './render_model_scope.js';

export {
  manualTestRowClass,
  renderModelTestAction
} from './render_model_table.js';
export {
  renderTokenGroupSelect
} from './render_model_scope.js';

function renderSiteStatusCard(site, group, summary, windowLabel) {
  const meta = modelStatusMeta(summary.status, true);
  return `
    <article class="model-site-card" data-slot="card">
      <div class="model-site-head" data-slot="card-header">
        <div>
          <h3 data-slot="card-title">${escapeHtml(modelSiteDisplayName(site.site.name))}</h3>
          <p data-slot="card-description">${escapeHtml(site.site.url)} · ${escapeHtml(group || '-')}</p>
        </div>
        ${renderChip(meta.label, meta.tone, 'status')}
      </div>
      <div class="model-site-stats" data-slot="card-content">
        ${renderMetric('请求成功率', formatNullableRate(summary.successRate), `${compactNumber(summary.requestCount)} 次请求`, '')}
        ${renderMetric('模型可用率', formatNullableRate(summary.modelAvailabilityRate), `${summary.operational}/${summary.modelCount} 正常`, 'success')}
        ${renderMetric('失败数', compactNumber(summary.failureCount), `最近 ${windowLabel}`, summary.failureCount ? 'danger' : '')}
        ${renderMetric('未知模型', compactNumber(summary.unknown), '无调用记录', '')}
      </div>
      ${site.logError || site.channelsError ? `<p class="site-error" data-slot="card-footer">${escapeHtml(site.logError || site.channelsError)}</p>` : ''}
    </article>
  `;
}

export function renderModelAvailability({ state, panelMotionClass = '', scopeMotionClass = '' }) {
  const modelStatus = state.modelStatus;
  const sites = modelStatus?.sites || [];

  if (state.loading && !modelStatus) {
    return `
      <div class="model-state-empty${panelMotionClass || scopeMotionClass}" id="modelsPanel" role="tabpanel" aria-labelledby="modelsTab" data-slot="tab-panel">
        ${renderStateCard('正在读取模型状态', '等待服务端返回管理站点模型数据', '加载中')}
      </div>
    `;
  }

  if (!modelStatus?.configured || !sites.length) {
    const reason = state.error || modelStatus?.configError || '当前没有可展示的管理站点或模型统计';
    return `
      <div class="model-state-empty${panelMotionClass || scopeMotionClass}" id="modelsPanel" role="tabpanel" aria-labelledby="modelsTab" data-slot="tab-panel">
        ${renderStateCard('暂无模型状态数据', reason, '未配置')}
      </div>
    `;
  }

  const scope = activeModelScope(modelStatus, state);
  const allScopedRows = scopedModelRows(modelStatus, scope, state, false);
  const models = scopedModelRows(modelStatus, scope, state, true);
  const summary = scopedSummary(allScopedRows);
  const windowLabel = formatWindow(modelStatus.windowSeconds);
  const scopeTabId = modelScopeTabId(scope.siteName);

  return `
    <div class="model-status-panel${panelMotionClass}" id="modelsPanel" role="tabpanel" aria-labelledby="modelsTab" data-slot="tab-panel">
      <div class="section-head model-status-head">
        <h2>模型状态</h2>
        <span>最近 ${escapeHtml(windowLabel)} · 下次刷新 ${escapeHtml(formatShortTime(modelStatus.expiresAt))}</span>
      </div>
      <div class="tabs model-scope-tabs-wrap" data-slot="tabs" data-orientation="horizontal">
        ${renderModelScopeControls(modelStatus, scope, state)}
        <div class="model-scope-content${scopeMotionClass}" id="modelScopePanel" role="tabpanel" aria-labelledby="${scopeTabId}" data-slot="tab-panel">
          ${scope.group ? renderSiteStatusCard(scope.site, scope.group, summary, windowLabel) : renderStateCard('请选择分组', '选择一个分组后查看模型状态', '等待选择')}
          <div class="model-toolbar">
            <label class="field model-filter">
              <span>模型筛选</span>
              <input id="modelStatusFilter" data-slot="input" value="${escapeHtml(state.modelFilter)}" placeholder="输入模型名过滤状态列表" />
            </label>
            <div class="model-status-counts">
              ${renderChip(`不可用 ${compactNumber(summary.down)}`, 'bad', 'status')}
              ${renderChip(`部分异常 ${compactNumber(summary.degraded)}`, 'warn', 'status')}
              ${renderChip(`未知 ${compactNumber(summary.unknown)}`, 'muted', 'status')}
              ${renderChip(`正常 ${compactNumber(summary.operational)}`, 'ok', 'status')}
            </div>
          </div>
          ${state.modelTestMessage ? renderAlert(state.modelTestMessage, 'info', 'model-test-notice') : ''}
          ${state.error ? renderAlert(state.error, 'danger') : ''}
          <div class="table-wrap availability-wrap">
            <table class="data-table availability-table">
              <thead>
                <tr>
                  <th class="sticky-col">模型</th>
                  <th>价格</th>
                  <th>状态</th>
                  <th>成功率</th>
                  <th>最近</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                ${renderModelTableRows(models, state)}
              </tbody>
            </table>
          </div>
          ${models.length === 0 ? '<div class="empty-inline">没有匹配的模型</div>' : ''}
        </div>
      </div>
    </div>
  `;
}
