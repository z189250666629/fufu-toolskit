const TOKEN_GROUP_TRIGGER_OPEN_KEYS = ['Enter', ' ', 'ArrowDown'];
const TOKEN_GROUP_OPTION_KEYS = ['ArrowDown', 'ArrowUp', 'Home', 'End', 'Escape'];
const TOKEN_GROUP_OPTION_NAV_KEYS = ['ArrowDown', 'ArrowUp', 'Home', 'End'];
const TOKEN_GROUP_OPTION_NAVIGATION = {
  forwardKeys: ['ArrowDown'],
  backwardKeys: ['ArrowUp'],
  wrap: false
};

export function isTokenGroupTriggerOpenKey(key) {
  return TOKEN_GROUP_TRIGGER_OPEN_KEYS.includes(key);
}

export function isTokenGroupOptionKey(key) {
  return TOKEN_GROUP_OPTION_KEYS.includes(key);
}

export function nextKeyboardNavigationIndex(key, currentIndex, total, options = {}) {
  if (total <= 0) return null;
  const lastIndex = total - 1;
  const boundedCurrentIndex = Math.max(0, Math.min(lastIndex, Number(currentIndex) || 0));

  if (key === 'Home') return 0;
  if (key === 'End') return lastIndex;

  const {
    forwardKeys = [],
    backwardKeys = [],
    wrap = false
  } = options;
  const delta = forwardKeys.includes(key)
    ? 1
    : backwardKeys.includes(key)
      ? -1
      : null;

  if (delta === null) return null;
  if (wrap) return (boundedCurrentIndex + delta + total) % total;
  return Math.max(0, Math.min(lastIndex, boundedCurrentIndex + delta));
}

export function nextTokenGroupOptionIndex(key, currentIndex, total) {
  if (total <= 0 || !TOKEN_GROUP_OPTION_NAV_KEYS.includes(key)) return null;
  return nextKeyboardNavigationIndex(key, currentIndex, total, TOKEN_GROUP_OPTION_NAVIGATION);
}
