(function(root) {
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

  function normalizeDollars(value) {
    const number = Number(value);
    return Number.isFinite(number) ? number : 0;
  }

  function prizeSymbol(symbols, dollars) {
    return escapeHtml(symbols?.[dollars] || '🎁');
  }

  function buildPrizeTableHtml(rows, symbols) {
    return safeRows(rows).map((row) => {
      const dollars = normalizeDollars(row?.dollars);
      const jackpot = dollars === 1000;
      return `
        <div class="prize-row ${jackpot ? 'jackpot' : ''}">
          <div class="symbol">${prizeSymbol(symbols, dollars)}</div>
          <div class="prize-main">
            <div class="amount">$${escapeHtml(dollars)}${jackpot ? ' JP' : ''}</div>
            <div class="odds">${escapeHtml(row?.pct)}%</div>
          </div>
        </div>
      `;
    }).join('');
  }

  function buildHistoryHtml(history, symbols) {
    const rows = safeRows(history);
    if (rows.length === 0) {
      return '<div class="history-empty">NO RECORDS</div>';
    }
    return rows.map((row) => {
      const dollars = normalizeDollars(row?.prize_dollars);
      return `
        <div class="history-item">
          <span class="prize">${prizeSymbol(symbols, dollars)} $${escapeHtml(dollars)}</span>
          <span class="time">${escapeHtml(row?.created_at)}</span>
        </div>
      `;
    }).join('');
  }

  root.activityRender = {
    escapeHtml,
    buildPrizeTableHtml,
    buildHistoryHtml
  };
})(globalThis);
