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

  function prizeSymbol(symbols, dollars, rank) {
    const rankSymbols = {
      jackpot: '🏆',
      second: '👑',
      third: '⭐',
    };
    return escapeHtml(symbols?.[dollars] || rankSymbols[rank] || '🎁');
  }

  function prizeRankClass(rank) {
    return ['jackpot', 'second', 'third'].includes(rank) ? rank : '';
  }

  function prizeLabel(row, jackpot) {
    const label = String(row?.label || '').trim();
    const dollars = normalizeDollars(row?.dollars);
    return `${label ? `${escapeHtml(label)} ` : ''}$${escapeHtml(dollars)}${jackpot ? ' JP' : ''}`;
  }

  function buildPrizeTableHtml(rows, symbols) {
    return safeRows(rows).map((row) => {
      const dollars = normalizeDollars(row?.dollars);
      const rank = String(row?.rank || '').trim();
      const jackpot = rank === 'jackpot';
      const rankClass = prizeRankClass(rank);
      return `
        <div class="prize-row ${rankClass}">
          <div class="symbol">${prizeSymbol(symbols, dollars, rank)}</div>
          <div class="prize-main">
            <div class="amount">${prizeLabel(row, jackpot)}</div>
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
