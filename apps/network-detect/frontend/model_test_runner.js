export function modelCellKey(siteName, model, group = '') {
  return `${siteName}\u0000${model}\u0000${group || ''}`;
}

function findModelCell(modelStatus, siteName, model, group = '') {
  const row = modelStatus?.models?.find((item) => item.model === model);
  const cell = row?.perSite?.[siteName];
  if (!cell) return null;
  if (group && cell.groupStats?.[group]) return cell.groupStats[group];
  return cell;
}

export function updateModelCell(modelStatus, siteName, model, cell, group = '') {
  const row = modelStatus?.models?.find((item) => item.model === model);
  if (!row?.perSite || !cell) return;
  if (group) {
    const siteCell = row.perSite[siteName];
    if (!siteCell) return;
    siteCell.groupStats ||= {};
    siteCell.groupStats[group] = cell;
    return;
  }
  row.perSite[siteName] = cell;
}

export function buildManualTestCellPatch(testRecord) {
  if (!testRecord) return null;
  return {
    manualTest: testRecord,
    nextTestAllowedAt: testRecord.nextAllowedAt || 0
  };
}

export function applyModelTestResultToState(modelStatus, siteName, model, result, group = '') {
  const targetSite = result?.siteName || siteName;
  const targetModel = result?.model || model;
  const targetGroup = result?.group || group || '';
  if (result?.cell) {
    updateModelCell(modelStatus, targetSite, targetModel, result.cell, result?.group || '');
    return true;
  }
  const cell = findModelCell(modelStatus, targetSite, targetModel, targetGroup);
  const patch = buildManualTestCellPatch(result?.test);
  if (!cell || !patch) return false;
  Object.assign(cell, patch);
  return true;
}

export async function runModelCellTest({
  state,
  siteName,
  model,
  group = '',
  postJsonImpl,
  render
}) {
  if (!siteName || !model) return false;
  const key = modelCellKey(siteName, model, group);
  if (state.testingCells.has(key)) return false;

  state.testingCells.add(key);
  state.modelTestMessage = '';
  render();

  try {
    const result = await postJsonImpl('/api/newapi/model-status/test', { siteName, model, group });
    applyModelTestResultToState(state.modelStatus, siteName, model, result, group);
    state.modelTestMessage = `${siteName} / ${model} 测试完成：${result.test?.message || '测试完成'}`;
  } catch (error) {
    const row = state.modelStatus?.models?.find((item) => item.model === model);
    const cell = group && row?.perSite?.[siteName]?.groupStats?.[group]
      ? row.perSite[siteName].groupStats[group]
      : row?.perSite?.[siteName];
    if (cell && error.data?.nextAllowedAt) cell.nextTestAllowedAt = error.data.nextAllowedAt;
    state.modelTestMessage = `${siteName} / ${model} 测试失败：${error.message}`;
  } finally {
    state.testingCells.delete(key);
    render();
  }

  return true;
}
