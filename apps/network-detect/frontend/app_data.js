export async function loadStaticContextState(state, fetchJsonImpl) {
  const [client, targetsResult] = await Promise.all([
    fetchJsonImpl('/api/client').catch(() => null),
    fetchJsonImpl('/api/connectivity/targets')
      .then((targets) => ({ targets }))
      .catch((error) => ({ error }))
  ]);
  state.client = client;
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
  state.loading = true;
  state.error = '';
  state.modelTestMessage = '';
  if (renderStart) render();

  try {
    state.modelStatus = await fetchJsonImpl(`/api/newapi/model-status${refresh ? '?refresh=1' : ''}`);
    state.initialized = true;
  } catch (error) {
    state.error = error.message;
    if (error.data && typeof error.data === 'object' && Number.isFinite(Number(error.data.generatedAt))) {
      state.modelStatus = error.data;
    }
  } finally {
    state.loading = false;
    render();
  }
}
