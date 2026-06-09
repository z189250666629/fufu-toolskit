export function activatePanelState(state, nextPanel) {
  const panel = nextPanel || 'url';
  if (state.activePanel === panel) {
    return { changed: false, shouldLoadModelStatus: false, panel };
  }

  const shouldLoadModelStatus = panel === 'models' && !state.modelStatus && !state.loading;
  state.activePanel = panel;
  state.groupSelectOpen = false;
  if (shouldLoadModelStatus) {
    state.loading = true;
    state.error = '';
    state.modelTestMessage = '';
  }

  return { changed: true, shouldLoadModelStatus, panel };
}

export function activateModelSiteState(state, siteName) {
  if (!siteName || state.selectedModelSite === siteName) return false;
  state.selectedModelSite = siteName;
  state.modelFilter = '';
  state.modelTestMessage = '';
  state.groupSelectOpen = false;
  return true;
}

export function selectTokenGroupState(state, group) {
  state.selectedTokenGroup = group || '';
  state.modelFilter = '';
  state.modelTestMessage = '';
  state.groupSelectOpen = false;
}
