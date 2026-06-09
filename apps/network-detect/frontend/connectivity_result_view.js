import {
  formatMs,
  formatRate
} from './utils.js';

export function buildConnectivityGroupView(group, results = []) {
  const groupResults = results.filter((item) => item.groupId === group.id);
  const okCount = groupResults.filter((item) => item.reachable).length;
  const resultMap = Object.fromEntries(groupResults.map((item) => [item.url, item]));
  const bestInGroup = groupResults
    .filter((item) => item.reachable && item.averageMs != null)
    .sort((a, b) => a.averageMs - b.averageMs)[0] || null;

  return {
    title: group.name,
    description: results.length ? `可达 ${okCount}/${group.urls.length}` : `${group.urls.length} 个站点`,
    rows: group.urls.map((url) => {
      const item = resultMap[url];
      if (!item) {
        return {
          url,
          status: 'idle',
          label: '等待',
          rate: '-',
          latency: '-',
          starred: false
        };
      }
      return {
        url: item.url,
        status: item.reachable ? 'ok' : 'bad',
        label: item.reachable ? '可达' : '失败',
        rate: formatRate(item.successRate),
        latency: formatMs(item.averageMs),
        starred: !!bestInGroup && item.url === bestInGroup.url
      };
    })
  };
}
