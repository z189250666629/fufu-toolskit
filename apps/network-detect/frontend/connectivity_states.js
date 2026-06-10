import {
  formatRate
} from './utils.js';

export function flattenConnectivityTargets(groups = []) {
  return groups.flatMap((group) => {
    const urls = Array.isArray(group.urls) ? group.urls : [];
    return urls
      .map((url) => String(url ?? '').trim())
      .filter(Boolean)
      .map((url) => ({ group, url }));
  });
}

export function buildNoTargetsState(testedAt) {
  return {
    mode: 'complete',
    tone: 'bad',
    icon: 'x',
    title: '没有测试目标',
    text: '后端没有返回固定 Base URL 目标。',
    progress: 100,
    progressText: '没有目标',
    currentUrl: '-',
    success: '-',
    testedAt,
    results: []
  };
}

export function buildRunningState() {
  return {
    running: true,
    mode: 'running',
    tone: 'warn',
    icon: '...',
    title: '测试中',
    text: '正在从当前浏览器逐个访问固定 Base URL。',
    progress: 0,
    progressText: '准备测试',
    currentUrl: '正在准备测试目标',
    success: '-',
    testedAt: '-',
    results: []
  };
}

export function buildConnectivityCompletionState(results, testedAt) {
  const totalReachable = results.filter((item) => item.reachable).length;
  const total = results.length;
  const baseState = {
    running: false,
    mode: 'complete',
    progress: 100,
    progressText: '测试完成',
    currentUrl: '全部目标测试完成',
    success: total ? formatRate(totalReachable / total) : '-',
    testedAt,
    results
  };

  if (totalReachable === total) {
    return {
      ...baseState,
      tone: 'ok',
      icon: 'OK',
      title: '全部可达',
      text: '当前用户浏览器可以访问全部 API 次数站和 Token 站 Base URL。'
    };
  }

  if (totalReachable > 0) {
    return {
      ...baseState,
      tone: 'warn',
      icon: '!',
      title: '部分可达',
      text: '当前用户网络只能访问部分 fufu Base URL，请优先使用可达且延迟较低的站点。'
    };
  }

  return {
    ...baseState,
    tone: 'bad',
    icon: 'x',
    title: '全部不可达',
    text: '当前用户浏览器无法访问这些 fufu Base URL。可能是 DNS、证书、网络阻断、代理或目标服务异常。',
    success: '0%'
  };
}

export function buildConnectivityErrorState(error, results, testedAt) {
  return {
    running: false,
    mode: 'complete',
    tone: 'bad',
    icon: 'x',
    title: '测试异常',
    text: error.message || '浏览器执行检测时发生异常。',
    progress: 100,
    progressText: '测试异常',
    currentUrl: '-',
    testedAt,
    results
  };
}
