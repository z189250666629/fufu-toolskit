export function modelCellKey(siteName, model) {
  return `${siteName}\u0000${model}`;
}

export function updateModelCell(modelStatus, siteName, model, cell) {
  const row = modelStatus?.models?.find((item) => item.model === model);
  if (!row?.perSite || !cell) return;
  row.perSite[siteName] = cell;
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
    updateModelCell(state.modelStatus, siteName, model, result.cell);
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
