export function applyTheme({ html, icon, storage }, dark, { persist = false } = {}) {
  html.setAttribute('data-theme', dark ? 'dark' : 'light');
  icon.textContent = dark ? '☀' : '☽';
  if (persist) {
    storage.setItem('theme', dark ? 'dark' : 'light');
  }
}

export function initTheme({
  html = document.documentElement,
  button = document.getElementById('themeToggle'),
  icon = document.getElementById('themeIcon'),
  storage = localStorage,
  matchMedia = window.matchMedia.bind(window)
} = {}) {
  const media = matchMedia('(prefers-color-scheme: dark)');
  const stored = storage.getItem('theme');
  const context = { html, icon, storage };

  if (stored === 'dark' || stored === 'light') {
    applyTheme(context, stored === 'dark');
  } else {
    applyTheme(context, media.matches);
  }

  button.addEventListener('click', function() {
    const isDark = html.getAttribute('data-theme') === 'dark';
    applyTheme(context, !isDark, { persist: true });
  });

  media.addEventListener('change', function(e) {
    if (!storage.getItem('theme')) {
      applyTheme(context, e.matches);
    }
  });
}
