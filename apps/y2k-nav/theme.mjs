function readStoredTheme(storage) {
  try {
    const value = storage?.getItem?.('theme');
    return value === 'dark' || value === 'light' ? value : null;
  } catch {
    return null;
  }
}

function writeStoredTheme(storage, value) {
  try {
    storage?.setItem?.('theme', value);
  } catch {
    // Storage can be disabled in privacy modes. Theme should still update.
  }
}

function getSystemThemeMedia(matchMedia) {
  try {
    return matchMedia?.('(prefers-color-scheme: dark)') ?? { matches: false };
  } catch {
    return { matches: false };
  }
}

export function applyTheme({ html, icon, storage }, dark, { persist = false } = {}) {
  html?.setAttribute?.('data-theme', dark ? 'dark' : 'light');
  if (icon) {
    icon.textContent = dark ? '☀' : '☽';
  }
  if (persist) {
    writeStoredTheme(storage, dark ? 'dark' : 'light');
  }
}

export function initTheme({
  html = document.documentElement,
  button = document.getElementById('themeToggle'),
  icon = document.getElementById('themeIcon'),
  storage = localStorage,
  matchMedia = window.matchMedia.bind(window)
} = {}) {
  if (!html || !icon) return;
  const media = getSystemThemeMedia(matchMedia);
  const stored = readStoredTheme(storage);
  const context = { html, icon, storage };

  if (stored) {
    applyTheme(context, stored === 'dark');
  } else {
    applyTheme(context, media.matches);
  }

  button?.addEventListener?.('click', function() {
    const isDark = html.getAttribute('data-theme') === 'dark';
    applyTheme(context, !isDark, { persist: true });
  });

  media.addEventListener?.('change', function(e) {
    if (!readStoredTheme(storage)) {
      applyTheme(context, e.matches);
    }
  });
}
