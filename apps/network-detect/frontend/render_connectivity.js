import {
  escapeHtml
} from './utils.js';
import {
  renderChip,
  renderMetric
} from './render_components.js';
import {
  buildConnectivityGroupView
} from './connectivity_result_view.js';

export function connectivityTagClass(status) {
  if (status === 'ok') return 'ok';
  if (status === 'warn') return 'warn';
  if (status === 'bad') return 'bad';
  return 'idle';
}

export function renderConnectivityRow(result) {
  const best = result.starred ? '<span class="url-star" aria-label="最优">⭐</span>' : '';
  const bestClass = result.starred ? ' is-best' : '';
  const safeUrl = escapeHtml(result.url);
  return `
    <div class="result-row${bestClass}">
      <div class="url-cell">
        <button class="url-copy" type="button" value="${safeUrl}" data-copy-value="${safeUrl}" title="点击复制 URL" aria-label="复制 ${safeUrl}">
          <span class="url-text">${safeUrl}</span>${best}
          <span class="copy-tip" aria-hidden="true">已复制</span>
        </button>
      </div>
      <div class="result-field">
        <span class="result-label">状态</span>
        <b>${renderChip(result.label, connectivityTagClass(result.status), 'tag')}</b>
      </div>
      <div class="result-field">
        <span class="result-label">成功率</span>
        <b>${escapeHtml(result.rate)}</b>
      </div>
      <div class="result-field">
        <span class="result-label">平均延迟</span>
        <b>${escapeHtml(result.latency)}</b>
      </div>
    </div>
  `;
}

export function renderConnectivityResults({ results = [], groups = [] }) {
  return groups.map((group) => {
    const view = buildConnectivityGroupView(group, results);

    return `
      <div class="connectivity-group" data-slot="card">
        <div class="group-head" data-slot="card-header">
          <h3 data-slot="card-title">${escapeHtml(view.title)}</h3>
          <span data-slot="card-description">${escapeHtml(view.description)}</span>
        </div>
        <div class="result-list" data-slot="card-content">
          ${view.rows.map(renderConnectivityRow).join('')}
        </div>
      </div>
    `;
  }).join('');
}

export function renderUrlStatus({ connectivity, groups, panelMotionClass = '' }) {
  const current = connectivity;
  return `
    <div class="url-monitor-grid${panelMotionClass}" id="urlPanel" role="tabpanel" aria-labelledby="urlTab" data-slot="tab-panel">
      <article class="verdict-card ${escapeHtml(current.mode)}" id="verdictCard" data-slot="card">
        <div class="verdict-icon ${escapeHtml(current.tone)}" id="verdictIcon" data-slot="card-media">${escapeHtml(current.icon)}</div>
        <div class="verdict-body">
          <div class="verdict-copy" data-slot="card-header">
            <h2 id="verdictTitle" data-slot="card-title">${escapeHtml(current.title)}</h2>
            <p id="verdictText" data-slot="card-description">${escapeHtml(current.text)}</p>
          </div>
          <div class="verdict-main" data-slot="card-content">
          <div class="connectivity-metrics">
            ${renderMetric('可达率', current.success, '固定 Base URL', '')}
            ${renderMetric('最后测试', current.testedAt, '当前浏览器网络', '')}
          </div>
          <div class="verdict-actions">
            <button class="button primary" id="runConnectivityBtn" type="button" ${current.running ? 'disabled' : ''}>${current.running ? '测试中' : (current.results.length ? '重新测试' : '开始测试')}</button>
          </div>
          <div class="progress-panel">
            <div class="progress-line">
              <span id="progressText">${escapeHtml(current.progressText)}</span>
              <b id="progressPct">${escapeHtml(`${current.progress}%`)}</b>
            </div>
            <div
              class="progress-bar"
              role="progressbar"
              aria-valuemin="0"
              aria-valuemax="100"
              aria-valuenow="${escapeHtml(current.progress)}"
              aria-labelledby="progressText"
              data-slot="progressbar"
            >
              <div class="track progress-bar-track" data-slot="progress-track">
                <div class="bar progress-bar-fill" id="progressBar" data-slot="progress-fill" style="width: ${escapeHtml(`${current.progress}%`)}"></div>
              </div>
            </div>
            <div class="current-url" id="currentUrl">${escapeHtml(current.currentUrl)}</div>
          </div>
          </div>
        </div>
      </article>
      <div class="results-block" data-slot="card">
        <div class="section-head" data-slot="card-header">
          <h2 data-slot="card-title">检测结果</h2>
          <span data-slot="card-description">浏览器直接访问</span>
        </div>
        <div class="groups" id="connectivityResultGroups" data-slot="card-content">
          ${renderConnectivityResults({ results: current.results || [], groups })}
        </div>
      </div>
    </div>
  `;
}
