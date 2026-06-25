(function registerCombineTraceResults(global) {
  const DEFAULT_UNIT_NAMES = { 3: '天卡', 8: '周不刷新卡', 9: '月不刷新卡' };
  const DEFAULT_QUOTA_UNIT = 500000;

  function escapeHtml(value) {
    return String(value ?? '')
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#39;');
  }

  function formatQuota(value, quotaUnit = DEFAULT_QUOTA_UNIT) {
    const unit = Number(quotaUnit) || DEFAULT_QUOTA_UNIT;
    return '$' + ((Number(value) || 0) / unit).toFixed(2);
  }

  function formatTraceTime(ms) {
    if (!ms) return '未知时间';
    try {
      return new Date(ms).toLocaleString();
    } catch {
      return String(ms);
    }
  }

  function traceStatusText(status) {
    const map = {
      done: '合并完成',
      error: '合并失败',
      rollback: '回滚中',
      deleting: '删除旧卡中',
      renaming: '整理新卡中',
      creating: '创建新卡中',
      verifying: '校验中',
      resolving: '解析中',
      started: '已开始',
    };
    return map[status] || status || '未知状态';
  }

  function traceDirectionText(direction) {
    if (direction === 'result') return '输入的是合成后的 key';
    if (direction === 'source') return '输入的是合成前的 key';
    if (direction === 'both') return '输入命中了合成前后 key';
    return '相关合卡记录';
  }

  function maskTraceKey(key) {
    const value = String(key || '');
    if (value.length <= 18) return value;
    return `${value.slice(0, 10)}…${value.slice(-6)}`;
  }

  function normalizeKeyForCompare(key) {
    const value = String(key || '').trim();
    if (!value) return '';
    return value.startsWith('sk-') ? value : `sk-${value}`;
  }

  function traceTokenKey(token) {
    return normalizeKeyForCompare(token?.key);
  }

  function buildTraceModel(records) {
    const unique = [];
    const seen = new Set();
    (records || []).forEach((record) => {
      const id = String(record?.mergeId || `${record?.createdAt || 0}-${unique.length}`);
      if (seen.has(id)) return;
      seen.add(id);
      unique.push(record);
    });

    const resultOwner = new Map();
    const sourceKeys = new Set();
    const resultKeys = new Set();
    unique.forEach((record) => {
      const resultKey = traceTokenKey(record.resultKey);
      if (resultKey) {
        resultOwner.set(resultKey, record.mergeId);
        resultKeys.add(resultKey);
      }
      (record.sourceKeys || []).forEach((token) => {
        const key = traceTokenKey(token);
        if (key) sourceKeys.add(key);
      });
    });

    const chainKeys = new Set();
    resultKeys.forEach((key) => {
      if (sourceKeys.has(key)) chainKeys.add(key);
    });

    const edges = new Map();
    const indegree = new Map();
    unique.forEach((record) => {
      edges.set(record.mergeId, new Set());
      indegree.set(record.mergeId, 0);
    });
    unique.forEach((record) => {
      (record.sourceKeys || []).forEach((token) => {
        const parentID = resultOwner.get(traceTokenKey(token));
        if (!parentID || parentID === record.mergeId || !edges.has(parentID)) return;
        const children = edges.get(parentID);
        if (!children.has(record.mergeId)) {
          children.add(record.mergeId);
          indegree.set(record.mergeId, (indegree.get(record.mergeId) || 0) + 1);
        }
      });
    });

    const byID = new Map(unique.map((record) => [record.mergeId, record]));
    const byTime = (a, b) => (a.createdAt || 0) - (b.createdAt || 0) || (a.mergeId || 0) - (b.mergeId || 0);
    const queue = unique.filter((record) => (indegree.get(record.mergeId) || 0) === 0).sort(byTime);
    const ordered = [];
    while (queue.length) {
      const record = queue.shift();
      ordered.push(record);
      (edges.get(record.mergeId) || []).forEach((childID) => {
        indegree.set(childID, (indegree.get(childID) || 0) - 1);
        if ((indegree.get(childID) || 0) === 0) {
          queue.push(byID.get(childID));
          queue.sort(byTime);
        }
      });
    }
    if (ordered.length < unique.length) {
      const orderedIDs = new Set(ordered.map((record) => record.mergeId));
      unique
        .filter((record) => !orderedIDs.has(record.mergeId))
        .sort(byTime)
        .forEach((record) => ordered.push(record));
    }

    return { ordered, chainKeys };
  }

  function renderTraceKey(token, chainKeys = new Set(), options = {}) {
    if (!token?.key) return '<div class="trace-key">未记录</div>';
    const key = traceTokenKey(token);
    const classes = ['trace-key'];
    if (chainKeys.has(key)) {
      classes.push('chain-key');
    }
    const unitNames = options.unitNames || DEFAULT_UNIT_NAMES;
    const meta = [
      token.name ? escapeHtml(token.name) : '',
      unitNames[token.interval_unit] || '',
      token.group ? escapeHtml(token.group) : '',
      token.remain_quota ? formatQuota(token.remain_quota, options.quotaUnit) : '',
    ].filter(Boolean).join(' · ');
    return `
      <div class="${classes.join(' ')}" data-key="${escapeHtml(token.key)}" onclick="copyTraceKey(this)" title="点击复制">
        ${escapeHtml(token.key)}
        ${meta ? `<div class="token-meta">${meta}</div>` : ''}
      </div>
    `;
  }

  function renderTraceResultsHtml(records, options = {}) {
    const { ordered, chainKeys } = buildTraceModel(records);
    const summary = ordered.length > 1
      ? `<div class="trace-chain-summary">检测到多步合卡路径，已按先后顺序整理；最后一步合成在最下面，紫色边框表示承接上下游合成的中间 key。</div>`
      : '';

    return summary + ordered.map((record, index) => {
      const sourceKeys = record.sourceKeys?.length
        ? record.sourceKeys.map((token) => renderTraceKey(token, chainKeys, options)).join('')
        : '<div class="trace-key">未记录来源 key</div>';
      const resultKey = record.resultKey
        ? renderTraceKey(record.resultKey, chainKeys, options)
        : '<div class="trace-key">未记录合成后 key</div>';
      const sourceFlow = (record.sourceKeys || [])
        .map((token) => `<span class="trace-flow-key">${escapeHtml(token.keyMask || maskTraceKey(token.key))}</span>`)
        .join('<span>+</span>');
      const resultFlow = record.resultKey
        ? `<span class="trace-flow-key">${escapeHtml(record.resultKey.keyMask || maskTraceKey(record.resultKey.key))}</span>`
        : '<span class="trace-flow-key">未记录</span>';
      const flow = `<div class="trace-flow"><span>第 ${index + 1} 步</span>${sourceFlow || '<span class="trace-flow-key">未记录</span>'}<span class="trace-arrow">→</span>${resultFlow}</div>`;
      const finalBadge = ordered.length > 1 && index === ordered.length - 1
        ? '<span class="trace-final-badge">最后一步</span>'
        : '';
      const warning = record.error || record.rollbackNote
        ? `<div class="trace-row"><div class="trace-label">异常 / 回滚</div><div class="token-meta" style="color:var(--warn)">${escapeHtml([record.error, record.rollbackNote].filter(Boolean).join(' '))}</div></div>`
        : '';
      return `
        <div class="trace-card ${ordered.length > 1 && index === ordered.length - 1 ? 'final-step' : ''}">
          <div class="trace-title"><span class="trace-step-badge">第 ${index + 1} 步</span><span>检测到合卡记录 · ${escapeHtml(traceStatusText(record.status))}</span>${finalBadge}</div>
          <div class="token-meta">${escapeHtml(traceDirectionText(record.direction))} · ${escapeHtml(formatTraceTime(record.createdAt))}</div>
          ${flow}
          <div class="trace-row">
            <div class="trace-label">合成前 key</div>
            ${sourceKeys}
          </div>
          <div class="trace-row">
            <div class="trace-label">合成后 key</div>
            ${resultKey}
          </div>
          ${warning}
        </div>
      `;
    }).join('');
  }

  function collectTraceKeys(records) {
    const set = new Set();
    (records || []).forEach((record) => {
      (record.sourceKeys || []).forEach((token) => {
        const key = normalizeKeyForCompare(token?.key);
        if (key) set.add(key);
      });
      const resultKey = normalizeKeyForCompare(record.resultKey?.key);
      if (resultKey) set.add(resultKey);
    });
    return set;
  }

  global.combineTraceResults = {
    buildTraceModel,
    collectTraceKeys,
    escapeHtml,
    maskTraceKey,
    normalizeKeyForCompare,
    renderTraceResultsHtml,
    traceDirectionText,
    traceStatusText
  };
})(typeof window !== 'undefined' ? window : globalThis);
