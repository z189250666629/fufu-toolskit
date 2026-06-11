export function createInitialAppState() {
  return {
    loading: false,
    initialized: false,
    modelFilter: '',
    selectedModelSite: '次数fufu',
    selectedTokenGroup: '',
    groupSelectOpen: false,
    modelTestMessage: '',
    activePanel: 'url',
    modelStatus: null,
    testingCells: new Set(),
    client: null,
    clientLoadError: '',
    targets: [],
    connectivityTargetError: '',
    connectivity: {
      running: false,
      mode: 'pending',
      tone: '',
      icon: '?',
      title: '等待测试',
      text: '页面会从当前用户浏览器发起请求，因此结果代表当前用户网络环境。测试会自动访问全部固定 Base URL。',
      progress: 0,
      progressText: '尚未开始测试',
      currentUrl: '尚未开始测试',
      success: '-',
      testedAt: '-',
      results: []
    },
    error: ''
  };
}

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
