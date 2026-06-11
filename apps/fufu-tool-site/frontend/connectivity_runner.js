import {
  SAMPLE_COUNT,
  TIMEOUT_MS,
  addCacheBust,
  fetchErrorText,
  fetchWithTimeout
} from './connectivity.js';
import { summarizeProbeAttempts } from './connectivity_probe.js';
import {
  buildConnectivityCompletionState,
  buildConnectivityErrorState,
  buildNoTargetsState,
  buildRunningState,
  flattenConnectivityTargets
} from './connectivity_states.js';

export {
  buildConnectivityCompletionState,
  buildConnectivityErrorState,
  buildNoTargetsState,
  buildRunningState,
  flattenConnectivityTargets
} from './connectivity_states.js';

export async function probeNoCors(url, onSample, deps = {}) {
  const {
    sampleCount = SAMPLE_COUNT,
    timeoutMs = TIMEOUT_MS,
    now = () => performance.now(),
    addCacheBustImpl = addCacheBust,
    fetchWithTimeoutImpl = fetchWithTimeout,
    fetchErrorTextImpl = fetchErrorText
  } = deps;
  const attempts = [];

  for (let i = 0; i < sampleCount; i++) {
    onSample?.(i + 1, sampleCount);
    const started = now();
    try {
      await fetchWithTimeoutImpl(addCacheBustImpl(url), {
        method: 'GET',
        mode: 'no-cors',
        cache: 'no-store',
        credentials: 'omit',
        redirect: 'follow'
      }, timeoutMs);

      attempts.push({
        ok: true,
        ms: now() - started,
        error: ''
      });
    } catch (error) {
      attempts.push({
        ok: false,
        ms: now() - started,
        error: fetchErrorTextImpl(error)
      });
    }
  }

  return summarizeProbeAttempts(attempts);
}

export async function testConnectivityTarget({
  group,
  url,
  index,
  total,
  setConnectivityState,
  probeImpl = probeNoCors
}) {
  const baseProgress = (index / total) * 90;
  setConnectivityState({
    currentUrl: url,
    progress: Math.round(baseProgress),
    progressText: `测试 ${group.name}: ${url}`
  });

  const reach = await probeImpl(url, (current, samples) => {
    const sampleProgress = (current / samples) * (90 / total);
    setConnectivityState({
      progress: Math.round(baseProgress + sampleProgress),
      progressText: `${group.name} 采样 ${current}/${samples}`,
      currentUrl: url
    });
  });

  return {
    groupId: group.id,
    groupName: group.name,
    url,
    reachable: reach.ok,
    successRate: reach.successRate,
    averageMs: reach.averageMs,
    lastError: reach.lastError
  };
}

export async function runConnectivityTests({
  connectivity,
  targetGroups,
  setConnectivityState,
  nowText = () => new Date().toLocaleString('zh-CN', { hour12: false }),
  testTargetImpl = testConnectivityTarget
}) {
  if (connectivity?.running) return [];

  const allTargets = flattenConnectivityTargets(targetGroups);
  if (!allTargets.length) {
    setConnectivityState(buildNoTargetsState(nowText()));
    return [];
  }

  setConnectivityState(buildRunningState());

  const results = [];
  try {
    for (let i = 0; i < allTargets.length; i++) {
      const { group, url } = allTargets[i];
      const result = await testTargetImpl({
        group,
        url,
        index: i,
        total: allTargets.length,
        setConnectivityState
      });
      results.push(result);
      setConnectivityState({ results: [...results] });
    }

    setConnectivityState(buildConnectivityCompletionState(results, nowText()));
  } catch (error) {
    setConnectivityState(buildConnectivityErrorState(error, results, nowText()));
  }

  return results;
}
