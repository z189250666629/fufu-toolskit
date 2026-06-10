import {
  statusFromCounts,
  statusFromSummary,
  successRate
} from './utils.js';

export function modelSortRank(row) {
  const rank = { down: 0, degraded: 1, operational: 2, unknown: 3 };
  return rank[row.status] ?? 4;
}

export function activeModelScope(modelStatus, stateLike = {}) {
  const sites = modelStatus?.sites || [];
  const preferredSite = sites.find((item) => item.site.name === stateLike.selectedModelSite) || sites[0] || null;
  const siteName = preferredSite?.site.name || '';
  const groups = preferredSite?.groups || [];
  const selectedGroup = stateLike.selectedTokenGroup && groups.includes(stateLike.selectedTokenGroup) ? stateLike.selectedTokenGroup : '';
  const group = selectedGroup || (siteName === '次数fufu' && groups.includes('mix') ? 'mix' : groups[0] || '');
  return { site: preferredSite, siteName, group, groups };
}

export function groupCellFor(row, siteName, group) {
  const cell = row.perSite?.[siteName];
  if (!cell?.configured || !group) return null;
  const groupCell = cell.groupStats?.[group];
  if (!groupCell?.configured) return null;
  return applyManualTestDisplay({
    ...cell,
    ...groupCell,
    siteName,
    model: row.model,
    configured: true,
    groups: [group],
    manualTest: groupCell.manualTest,
    nextTestAllowedAt: groupCell.nextTestAllowedAt
  });
}

function manualTestAlreadyCounted(cell, testedAt, passed) {
  const count = Number(passed ? cell.successCount : cell.failureCount) || 0;
  const lastAt = Number(passed ? cell.lastSuccessAt : cell.lastFailureAt) || 0;
  return count > 0 && testedAt > 0 && lastAt >= testedAt;
}

export function applyManualTestDisplay(cell) {
  const manual = cell.manualTest;
  if (!manual?.testedAt) return cell;

  const passed = manual.ok === true || manual.status === 'operational';
  const manualTestTone = passed ? 'ok' : 'bad';
  const testedAt = Number(manual.testedAt) || 0;
  const successCount = Number(cell.successCount) || 0;
  const failureCount = Number(cell.failureCount) || 0;
  const hasLogData = successCount + failureCount > 0;

  if (manualTestAlreadyCounted(cell, testedAt, passed)) {
    return {
      ...cell,
      manualTestTone
    };
  }

  if (!passed && hasLogData) return cell;

  const nextSuccessCount = successCount + (passed ? 1 : 0);
  const nextFailureCount = failureCount + (passed ? 0 : 1);
  return {
    ...cell,
    manualTestTone,
    status: statusFromCounts(nextSuccessCount, nextFailureCount),
    successRate: nextSuccessCount / (nextSuccessCount + nextFailureCount),
    requestCount: nextSuccessCount + nextFailureCount,
    successCount: nextSuccessCount,
    failureCount: nextFailureCount,
    lastSuccessAt: passed ? Math.max(Number(cell.lastSuccessAt) || 0, testedAt) : cell.lastSuccessAt,
    lastFailureAt: passed ? cell.lastFailureAt : Math.max(Number(cell.lastFailureAt) || 0, testedAt),
    lastSeenAt: testedAt || cell.lastSeenAt
  };
}

export function scopedModelRows(modelStatus, scope, stateLike = {}, applyTextFilter = true) {
  const filter = String(stateLike.modelFilter || '').trim().toLowerCase();
  return (modelStatus?.models || [])
    .map((row) => ({ row, cell: groupCellFor(row, scope.siteName, scope.group) }))
    .filter((item) => item.cell)
    .filter((item) => item.cell.enabledChannelCount > 0)
    .filter((item) => !applyTextFilter || !filter || item.row.model.toLowerCase().includes(filter))
    .sort((a, b) => {
      const statusRank = modelSortRank({ status: a.cell.status }) - modelSortRank({ status: b.cell.status });
      if (statusRank !== 0) return statusRank;
      return a.row.model.localeCompare(b.row.model);
    });
}

export function scopedSummary(rows) {
  const summary = rows.reduce(
    (acc, item) => {
      acc.modelCount += 1;
      acc.requestCount += item.cell.requestCount;
      acc.successCount += item.cell.successCount;
      acc.failureCount += item.cell.failureCount;
      acc[item.cell.status] += 1;
      return acc;
    },
    { modelCount: 0, requestCount: 0, successCount: 0, failureCount: 0, operational: 0, degraded: 0, down: 0, unknown: 0 }
  );
  summary.successRate = successRate(summary.successCount, summary.failureCount);
  summary.modelAvailabilityRate = summary.modelCount > 0 ? summary.operational / summary.modelCount : null;
  summary.status = statusFromSummary(summary);
  return summary;
}
