import {
  nextKeyboardNavigationIndex
} from './app_event_keys.js';

const TAB_NAVIGATION = {
  forwardKeys: ['ArrowRight', 'ArrowDown'],
  backwardKeys: ['ArrowLeft', 'ArrowUp'],
  wrap: true
};

export function getCopyUrl(button, allowedUrls) {
  const candidates = [
    button.value,
    button.getAttribute('data-copy-value'),
    button.querySelector('.url-text')?.textContent?.trim()
  ];
  return candidates.find((value) => allowedUrls.has(value)) || '';
}

export async function copyText(value, deps = {}) {
  if (!value) throw new Error('没有可复制的 URL');

  const {
    navigatorLike = globalThis.navigator,
    windowLike = globalThis.window,
    documentLike = globalThis.document
  } = deps;

  if (navigatorLike.clipboard && windowLike.isSecureContext) {
    await navigatorLike.clipboard.writeText(value);
    return;
  }

  const selection = windowLike.getSelection?.();
  if (selection) selection.removeAllRanges();

  const textarea = documentLike.createElement('textarea');
  textarea.value = value;
  textarea.setAttribute('readonly', '');
  textarea.style.position = 'fixed';
  textarea.style.left = '-9999px';
  documentLike.body.appendChild(textarea);
  textarea.focus();
  textarea.select();
  textarea.setSelectionRange(0, value.length);
  const copied = documentLike.execCommand('copy');
  textarea.remove();
  if (!copied) throw new Error('复制失败');
}

export function showCopiedFeedback(button, text = '已复制', deps = {}) {
  const {
    documentLike = globalThis.document,
    windowLike = globalThis.window
  } = deps;

  const tip = button.querySelector('.copy-tip');
  if (tip) tip.textContent = text;
  documentLike.querySelectorAll('.url-copy.copied').forEach((item) => item.classList.remove('copied'));
  button.classList.add('copied');
  windowLike.clearTimeout(button._copiedTimer);
  button._copiedTimer = windowLike.setTimeout(() => button.classList.remove('copied'), 1200);
}

export function formatNetworkType(navigatorLike = globalThis.navigator) {
  const connection = navigatorLike.connection || navigatorLike.mozConnection || navigatorLike.webkitConnection;
  if (!connection) return '浏览器未提供';

  const parts = [
    connection.type,
    connection.effectiveType,
    Number.isFinite(connection.downlink) ? `${connection.downlink} Mbps` : '',
    Number.isFinite(connection.rtt) ? `${connection.rtt} ms RTT` : ''
  ].filter(Boolean);

  return parts.length ? [...new Set(parts)].join(' / ') : '未知';
}

export function bindTabKeyboard(button, selector, activate, documentLike = globalThis.document) {
  button.addEventListener('keydown', (event) => {
    const tabList = button.closest('[role="tablist"]') || documentLike;
    const tabs = [...tabList.querySelectorAll(selector)];
    const currentIndex = Math.max(0, tabs.indexOf(button));
    const nextIndex = nextKeyboardNavigationIndex(event.key, currentIndex, tabs.length, TAB_NAVIGATION);
    if (nextIndex === null) return;
    event.preventDefault();
    const nextTab = tabs[nextIndex];
    nextTab?.focus();
    activate(nextTab);
  });
}
