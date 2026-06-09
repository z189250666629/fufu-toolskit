import {
  compactNumber,
  escapeHtml,
  formatCooldown,
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
  renderPriceCell,
  renderStateCard,
  renderStatusPill
} from './render_components.js';

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

export function renderModelTestAction(cell, stateLike = {}) {
  const key = `${cell.siteName}\u0000${cell.model}`;
  const testing = stateLike.testingCells?.has(key);
  const cooldown = formatCooldown(cell.nextTestAllowedAt);

  return `
    <div class="model-action">
      <button
        class="button small model-test-button"
        type="button"
        data-model-test="1"
        data-site="${escapeHtml(cell.siteName)}"
        data-model="${escapeHtml(cell.model)}"
        data-group="${escapeHtml(cell.groups?.[0] || '')}"
        ${testing || Boolean(cooldown) ? 'disabled' : ''}
      >${testing ? '测试中' : (cooldown || '测试')}</button>
    </div>
  `;
}

export function manualTestRowClass(cell) {
  if (cell.manualTestTone === 'ok') return 'is-manual-ok';
  if (cell.manualTestTone === 'bad') return 'is-manual-bad';
  return '';
}

export function renderTokenGroupSelect(groups, selectedGroup, stateLike = {}) {
  const isOpen = stateLike.groupSelectOpen;
  const current = selectedGroup || groups[0] || '';

  return `
    <div class="field group-select heroui-select" data-slot="select" data-token-group-select>
      <span id="tokenGroupSelectLabel" data-slot="select-label">分组</span>
      <button
        class="heroui-select-trigger"
        id="tokenGroupSelect"
        type="button"
        data-slot="select-trigger"
        aria-haspopup="listbox"
        aria-expanded="${isOpen ? 'true' : 'false'}"
        ${isOpen ? 'aria-controls="tokenGroupListbox"' : ''}
        aria-labelledby="tokenGroupSelectLabel tokenGroupSelectValue"
        data-token-group-trigger
      >
        <span class="heroui-select-value" id="tokenGroupSelectValue" data-slot="select-value">${escapeHtml(current || '选择分组')}</span>
        <span class="heroui-select-indicator" data-slot="select-indicator" aria-hidden="true">
          <svg viewBox="0 0 20 20" focusable="false">
            <path d="M5.5 7.5 10 12l4.5-4.5" />
          </svg>
        </span>
      </button>
      ${isOpen ? `
        <div class="heroui-select-popover" data-slot="select-popover">
          <div class="heroui-select-listbox" id="tokenGroupListbox" role="listbox" data-slot="listbox" aria-labelledby="tokenGroupSelectLabel">
            ${groups.map((group) => `
              <button
                class="heroui-select-item"
                type="button"
                role="option"
                data-slot="listbox-item"
                data-selected="${group === current ? 'true' : 'false'}"
                aria-selected="${group === current ? 'true' : 'false'}"
                data-token-group-option="${escapeHtml(group)}"
              >
                <span>${escapeHtml(group)}</span>
                <span class="heroui-select-item-indicator" data-slot="select-item-indicator" aria-hidden="true">
                  ${group === current ? `
                    <svg viewBox="0 0 20 20" focusable="false">
                      <path d="m4.75 10.25 3.25 3.25 7.25-7.25" />
                    </svg>
                  ` : ''}
                </span>
              </button>
            `).join('')}
          </div>
        </div>
      ` : ''}
    </div>
  `;
}

function renderModelScopeControls(modelStatus, scope, stateLike) {
  const sites = modelStatus.sites || [];
  const activeIndex = Math.max(0, sites.findIndex((item) => item.site.name === scope.siteName));
  const hasTokenGroups = scope.siteName === 'token-fufu';
  return `
    <div class="model-scope-bar">
      <div
        class="model-scope-tabs tabs__list"
        role="tablist"
        aria-label="模型站点"
        data-slot="tab-list"
        data-tab-motion-key="model-site"
        data-orientation="horizontal"
        style="--tab-count: ${Math.max(1, sites.length)}; --active-tab-index: ${activeIndex};"
      >
        <span class="tab-indicator tabs__indicator" data-slot="tab-indicator" aria-hidden="true"></span>
        ${sites.map((item) => {
          const active = item.site.name === scope.siteName;
          return `
          <button
            class="scope-button tabs__tab ${active ? 'active' : ''}"
            type="button"
            role="tab"
            aria-selected="${active ? 'true' : 'false'}"
            aria-controls="modelScopePanel"
            id="${modelScopeTabId(item.site.name)}"
            tabindex="${active ? '0' : '-1'}"
            data-slot="tab"
            data-selected="${active ? 'true' : 'false'}"
            data-model-site="${escapeHtml(item.site.name)}"
          >
            <span class="tab-label" data-slot="tab-label">${escapeHtml(modelSiteDisplayName(item.site.name))}</span>
          </button>
        `;
        }).join('')}
      </div>
      <div class="model-scope-group-slot${hasTokenGroups ? '' : ' is-placeholder'}" ${hasTokenGroups ? '' : 'aria-hidden="true"'}>
        ${hasTokenGroups ? renderTokenGroupSelect(scope.groups || [], scope.group, stateLike) : ''}
      </div>
    </div>
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
                ${models.map(({ row, cell }) => `
                  <tr class="${manualTestRowClass(cell)}">
                    <td class="sticky-col model-name" data-label="模型">${escapeHtml(row.model)}</td>
                    <td data-label="价格">${renderPriceCell(cell.pricing)}</td>
                    <td data-label="状态">${renderStatusPill(cell.status, true)}</td>
                    <td data-label="成功率">${escapeHtml(formatNullableRate(cell.successRate))}</td>
                    <td data-label="最近">${escapeHtml(formatShortTime(cell.lastSuccessAt || cell.lastFailureAt || cell.lastSeenAt))}</td>
                    <td data-label="操作">${renderModelTestAction(cell, state)}</td>
                  </tr>
                `).join('')}
              </tbody>
            </table>
          </div>
          ${models.length === 0 ? '<div class="empty-inline">没有匹配的模型</div>' : ''}
        </div>
      </div>
    </div>
  `;
}
