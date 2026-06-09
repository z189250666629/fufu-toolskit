export function modelCellKey(siteName, model) {
  return `${siteName}\u0000${model}`;
}

export function updateModelCell(modelStatus, siteName, model, cell) {
  const row = modelStatus?.models?.find((item) => item.model === model);
  if (!row?.perSite || !cell) return;
  row.perSite[siteName] = cell;
}

export function buildManualTestCellPatch(testRecord) {
  if (!testRecord) return null;
  return {
    manualTest: testRecord,
    nextTestAllowedAt: testRecord.nextAllowedAt || 0
  };
}

export function applyModelTestResultToState(modelStatus, siteName, model, result) {
  const targetSite = result?.siteName || siteName;
  const targetModel = result?.model || model;
  if (result?.cell) {
    updateModelCell(modelStatus, targetSite, targetModel, result.cell);
    return true;
  }
  const row = modelStatus?.models?.find((item) => item.model === targetModel);
  const cell = row?.perSite?.[targetSite];
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
  const key = modelCellKey(siteName, model);
  if (state.testingCells.has(key)) return false;

  state.testingCells.add(key);
  state.modelTestMessage = '';
  render();

  try {
    const result = await postJsonImpl('/api/newapi/model-status/test', { siteName, model, group });
    applyModelTestResultToState(state.modelStatus, siteName, model, result);
    state.modelTestMessage = `${siteName} / ${model} 测试完成：${result.test?.message || '测试完成'}`;
  } catch (error) {
    const row = state.modelStatus?.models?.find((item) => item.model === model);
    const cell = row?.perSite?.[siteName];
    if (cell && error.data?.nextAllowedAt) cell.nextTestAllowedAt = error.data.nextAllowedAt;
    state.modelTestMessage = `${siteName} / ${model} 测试失败：${error.message}`;
  } finally {
    state.testingCells.delete(key);
    render();
  }

  return true;
}
