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

  root.adminRender = {
    escapeHtml,
    statusClass,
    buildStatsGridHtml
  };
})(globalThis);
