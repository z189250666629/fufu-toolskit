(function(root) {
  const PRIZE_SYMBOLS = {1:'🍋',5:'🍒',10:'🍊',20:'🍇',50:'💎',100:'⭐',200:'👑',500:'🔥',1000:'🏆'};
  const ALLOWED_STATUS_CLASSES = new Set([
    'pending', 'done', 'paid', 'failed',
    'playing', 'won', 'lost', 'cashout',
    'active', 'completed', 'cashed_out', 'bust', 'reset'
  ]);

  function escapeHtml(value) {
    return String(value ?? '')
      .replaceAll('&', '&amp;')
      .replaceAll('<', '&lt;')
      .replaceAll('>', '&gt;')
      .replaceAll('"', '&quot;')
      .replaceAll("'", '&#039;');
  }

  function safeRows(rows) {
    return Array.isArray(rows) ? rows : [];
  }

  function numberValue(value) {
    const number = Number(value);
    return Number.isFinite(number) ? number : 0;
  }

  function n(value) {
    return numberValue(value).toLocaleString();
  }

  function safePlans(plans) {
    return Array.isArray(plans) ? plans : [];
  }

  function scheduleJobs(schedule) {
    const jobs = schedule && Array.isArray(schedule.jobs) ? schedule.jobs : [];
    const byPlan = new Map();
    for (const job of jobs) {
      byPlan.set(String(job.plan ?? ''), job);
    }
    return byPlan;
  }

  function checked(value) {
    return value ? ' checked' : '';
  }

  function statusClass(status) {
    const value = String(status ?? '').trim().toLowerCase();
    return ALLOWED_STATUS_CLASSES.has(value) ? value : 'unknown';
  }

  function statusLabel(status) {
    const value = String(status ?? 'unknown').trim() || 'unknown';
    return escapeHtml(value.toUpperCase());
  }

  function emptyRow(cols) {
    return `<tr><td colspan="${cols}"><div class="empty">NO DATA</div></td></tr>`;
  }

  function buildSummary(data) {
    return `
      <div class="px-panel">
        <div class="panel-title">OVERVIEW</div>
        <div class="summary-row">
          <div class="stat-item">
            <div class="stat-label">总抽奖次数</div>
            <div class="stat-value">${n(data.totalSpins)}</div>
          </div>
          <div class="stat-item">
            <div class="stat-label">总中奖金额</div>
            <div class="stat-value green">$${n(data.totalWon)}</div>
          </div>
          <div class="stat-item">
            <div class="stat-label">实际期望值 / 次</div>
            <div class="stat-value cyan">$${escapeHtml(data.ev ?? '')}</div>
          </div>
        </div>
      </div>`;
  }

  function buildPrizeRows(data) {
    const rows = safeRows(data.prizeRows);
    if (!rows.length) return emptyRow(4);
    const totalSpins = numberValue(data.totalSpins);
    return rows.map((row) => {
      const dollars = numberValue(row.prize_dollars);
      const count = numberValue(row.count);
      const percent = totalSpins > 0 ? ((count / totalSpins) * 100).toFixed(2) + '%' : '-';
      return `
        <tr>
          <td>${escapeHtml(PRIZE_SYMBOLS[dollars] || '🎁')} $${escapeHtml(dollars)}</td>
          <td>${n(row.count)}</td>
          <td>$${n(row.total)}</td>
          <td>${escapeHtml(percent)}</td>
        </tr>`;
    }).join('');
  }

  function buildTierRows(data) {
    const rows = safeRows(data.tierRows);
    if (!rows.length) return emptyRow(5);
    return rows.map((row) => {
      const usedSpins = numberValue(row.used_spins);
      const ev = usedSpins > 0 ? '$' + (numberValue(row.total_won) / usedSpins).toFixed(2) : '-';
      return `<tr>
        <td>$${n(row.dollars)}</td>
        <td>${n(row.cards)}</td>
        <td>${n(row.used_spins)}/${n(row.total_spins)}</td>
        <td>$${n(row.total_won)}</td>
        <td>${escapeHtml(ev)}</td>
      </tr>`;
    }).join('');
  }

  function buildStatusRows(rows, totalLabel) {
    const safe = safeRows(rows);
    if (!safe.length) return emptyRow(3);
    return safe.map((row) => {
      const status = row.status;
      return `
        <tr>
          <td><span class="badge badge-${statusClass(status)}">${statusLabel(status)}</span></td>
          <td>${n(row.count)}</td>
          <td>$${n(row[totalLabel])}</td>
        </tr>`;
    }).join('');
  }

  function buildStatsGridHtml(data = {}) {
    const summary = buildSummary(data);
    const prizes = `
      <div class="px-panel">
        <div class="panel-title">PRIZE DIST</div>
        <div class="table-wrap"><table>
          <tr><th>奖项</th><th>次数</th><th>合计</th><th>占比</th></tr>
          ${buildPrizeRows(data)}
        </table></div>
      </div>`;
    const tiers = `
      <div class="px-panel">
        <div class="panel-title">CARD TIERS</div>
        <div class="table-wrap"><table>
          <tr><th>面额</th><th>卡数</th><th>已用/总次</th><th>总中奖</th><th>期望值</th></tr>
          ${buildTierRows(data)}
        </table></div>
      </div>`;
    const queue = `
      <div class="px-panel">
        <div class="panel-title">CREDIT QUEUE</div>
        <div class="table-wrap"><table>
          <tr><th>状态</th><th>笔数</th><th>金额</th></tr>
          ${buildStatusRows(data.queueRows, 'total')}
        </table></div>
      </div>`;
    const scratch = `
      <div class="px-panel">
        <div class="panel-title">SCRATCH CARD</div>
        <div class="table-wrap"><table>
          <tr><th>状态</th><th>局数</th><th>总奖励</th></tr>
          ${buildStatusRows(data.scratchRows, 'total')}
        </table></div>
      </div>`;

    return `<div class="stats-grid">${summary}${prizes}${tiers}${queue}${scratch}</div>`;
  }

  function buildSaleCardManagerHtml(config = {}, result = {}) {
    const plans = safePlans(config.plans);
    const schedule = config.schedule || {};
    const jobs = scheduleJobs(schedule);
    const selectedPlan = plans[0]?.id || '';
    const options = plans.length ? plans.map((plan) => `
      <option value="${escapeHtml(plan.id)}">${escapeHtml(plan.name || plan.id)} · $${escapeHtml(plan.quota ?? '')}</option>`).join('') : '<option value="">NO PLANS</option>';
    const rows = plans.length ? plans.map((plan) => {
      const job = jobs.get(String(plan.id)) || {};
      const enabled = Boolean(job.enabled);
      const count = numberValue(job.count);
      return `
        <tr data-sale-card-plan="${escapeHtml(plan.id)}">
          <td>
            <div class="sale-card-name">${escapeHtml(plan.name || plan.id)}</div>
            <div class="sale-card-meta">ITEM ${escapeHtml(plan.itemId)} / SKU ${escapeHtml(plan.skuId)} / ${escapeHtml(plan.group || 'mix')}</div>
          </td>
          <td>$${escapeHtml(plan.quota ?? '')}</td>
          <td>${escapeHtml(plan.intervalUnit ?? '')}</td>
          <td><input class="sale-card-job-count px-input mini-input" type="number" min="1" max="100" value="${count > 0 ? escapeHtml(count) : ''}" data-plan="${escapeHtml(plan.id)}"></td>
          <td><input class="sale-card-job-enabled" type="checkbox" data-plan="${escapeHtml(plan.id)}"${checked(enabled)}></td>
        </tr>`;
    }).join('') : emptyRow(5);
    const uploaded = numberValue(result.uploaded);
    const resultHtml = uploaded > 0 ? `<div class="sale-card-result ok">上架成功：${n(uploaded)} 张</div>` : '';

    return `
      <div class="px-panel sale-card-panel">
        <div class="panel-title">SALE CARD</div>
        <div class="sale-card-note">每日自动任务：在这里维护启用状态、执行时间和每个计划的上架数量；后续定时器会读取这份配置执行。</div>
        <div class="sale-card-form">
          <label class="sale-card-check"><input id="sale-card-enabled" type="checkbox"${checked(schedule.enabled)}> 启用每日自动上架</label>
          <label>时间 <input id="sale-card-time" class="px-input mini-input" value="${escapeHtml(schedule.time || '09:00')}" placeholder="09:00"></label>
          <label>时区 <input id="sale-card-timezone" class="px-input mini-input" value="${escapeHtml(schedule.timezone || 'Asia/Shanghai')}" placeholder="Asia/Shanghai"></label>
          <button class="px-btn btn-cyan" id="sale-card-save" type="button">SAVE</button>
        </div>
        <div class="table-wrap"><table>
          <tr><th>计划</th><th>额度</th><th>周期</th><th>每日数量</th><th>启用</th></tr>
          ${rows}
        </table></div>
        <div class="sale-card-actions">
          <select id="sale-card-plan" class="px-input">${options}</select>
          <input id="sale-card-run-count" class="px-input mini-input" type="number" min="1" max="100" value="1">
          <button class="px-btn" id="sale-card-run" type="button" data-default-plan="${escapeHtml(selectedPlan)}">RUN NOW</button>
        </div>
        ${resultHtml}
        <div class="sale-card-message" id="sale-card-message"></div>
      </div>`;
  }

  root.adminRender = {
    escapeHtml,
    statusClass,
    buildStatsGridHtml,
    buildSaleCardManagerHtml
  };
})(globalThis);
