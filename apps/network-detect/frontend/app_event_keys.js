const TOKEN_GROUP_TRIGGER_OPEN_KEYS = ['Enter', ' ', 'ArrowDown'];
const TOKEN_GROUP_OPTION_KEYS = ['ArrowDown', 'ArrowUp', 'Home', 'End', 'Escape'];
const TOKEN_GROUP_OPTION_NAV_KEYS = ['ArrowDown', 'ArrowUp', 'Home', 'End'];

export function isTokenGroupTriggerOpenKey(key) {
  return TOKEN_GROUP_TRIGGER_OPEN_KEYS.includes(key);
}

export function isTokenGroupOptionKey(key) {
  return TOKEN_GROUP_OPTION_KEYS.includes(key);
}

export function nextTokenGroupOptionIndex(key, currentIndex, total) {
  if (total <= 0 || !TOKEN_GROUP_OPTION_NAV_KEYS.includes(key)) return null;
  if (key === 'Home') return 0;
  if (key === 'End') return total - 1;
  const delta = key === 'ArrowDown' ? 1 : -1;
  return Math.max(0, Math.min(total - 1, currentIndex + delta));
}
