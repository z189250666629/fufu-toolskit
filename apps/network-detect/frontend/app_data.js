export async function loadStaticContextState(state, fetchJsonImpl) {
  const [clientResult, targetsResult] = await Promise.all([
    fetchJsonImpl('/api/client')
      .then((client) => ({ client }))
      .catch((error) => ({ error })),
    fetchJsonImpl('/api/connectivity/targets')
      .then((targets) => ({ targets }))
      .catch((error) => ({ error }))
  ]);
  if (clientResult.error) {
    state.client = null;
    state.clientLoadError = clientResult.error.message || '访问环境读取失败';
  } else {
    state.client = clientResult.client || null;
    state.clientLoadError = '';
  }
  if (targetsResult.error) {
    state.targets = [];
    state.connectivityTargetError = targetsResult.error.message || '连通性目标配置读取失败';
    return;
  }
  state.targets = Array.isArray(targetsResult.targets?.groups) ? targetsResult.targets.groups : [];
  state.connectivityTargetError = '';
}

export async function loadModelStatusState(state, {
  refresh = false,
  renderStart = true,
  fetchJsonImpl,
  render
}) {
  const requestSeq = (Number.isFinite(state.modelStatusRequestSeq) ? state.modelStatusRequestSeq : 0) + 1;
  state.modelStatusRequestSeq = requestSeq;
  const isLatestRequest = () => state.modelStatusRequestSeq === requestSeq;
  state.loading = true;
  state.error = '';
  state.modelTestMessage = '';
  if (renderStart) render();

  try {
    const modelStatus = await fetchJsonImpl(`/api/newapi/model-status${refresh ? '?refresh=1' : ''}`);
    if (!isLatestRequest()) return;
    state.modelStatus = modelStatus;
    state.initialized = true;
  } catch (error) {
    if (!isLatestRequest()) return;
    state.error = error.message;
    if (error.data && typeof error.data === 'object' && Number.isFinite(Number(error.data.generatedAt))) {
      state.modelStatus = error.data;
    }
  } finally {
    if (!isLatestRequest()) return;
    state.loading = false;
    render();
  }
}
