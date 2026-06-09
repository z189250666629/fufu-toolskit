export function motionClass(renderMotion, ...types) {
  return types.includes(renderMotion) ? ' motion-enter' : '';
}

export function captureTabIndicatorRects(documentLike = document) {
  const rects = new Map();
  documentLike.querySelectorAll('.tabs__list > .tab-indicator').forEach((indicator) => {
    const tabList = indicator.closest('[role="tablist"]');
    const key = tabList?.dataset.tabMotionKey || tabList?.getAttribute('aria-label') || '';
    if (key) rects.set(key, indicator.getBoundingClientRect());
  });
  return rects;
}

export function animateTabIndicators(previousRects, deps = {}) {
  const documentLike = deps.document || document;
  const windowLike = deps.window || window;
  if (!previousRects?.size) return;
  if (windowLike.matchMedia?.('(prefers-reduced-motion: reduce)').matches) return;

  documentLike.querySelectorAll('.tabs__list > .tab-indicator').forEach((indicator) => {
    const tabList = indicator.closest('[role="tablist"]');
    const key = tabList?.dataset.tabMotionKey || tabList?.getAttribute('aria-label') || '';
    const previous = previousRects.get(key);
    if (!previous) return;

    const next = indicator.getBoundingClientRect();
    const horizontal = tabList?.dataset.orientation !== 'vertical';
    const dx = previous.left - next.left;
    const dy = horizontal ? 0 : previous.top - next.top;
    if (Math.abs(dx) < 0.5 && Math.abs(dy) < 0.5) return;

    indicator.style.transition = 'none';
    indicator.style.transform = `translate(calc(var(--indicator-x) + ${dx}px), calc(var(--indicator-y) + ${dy}px))`;
    indicator.getBoundingClientRect();

    windowLike.requestAnimationFrame(() => {
      indicator.style.transition = 'transform 260ms var(--ease-out-fluid), width 260ms var(--ease-out-fluid), height 260ms var(--ease-out-fluid)';
      indicator.style.transform = '';
      windowLike.setTimeout(() => {
        indicator.style.transition = '';
      }, 280);
    });
  });
}

export function captureRenderScroll(activePanel, deps = {}) {
  const documentLike = deps.document || document;
  const windowLike = deps.window || window;
  const tableWrap = documentLike.querySelector('.availability-wrap');
  return {
    activePanel,
    windowX: windowLike.scrollX,
    windowY: windowLike.scrollY,
    tableTop: tableWrap?.scrollTop || 0,
    tableLeft: tableWrap?.scrollLeft || 0
  };
}

export function restoreRenderScroll(snapshot, activePanel, deps = {}) {
  const documentLike = deps.document || document;
  const windowLike = deps.window || window;
  if (!snapshot || snapshot.activePanel !== activePanel) return;
  const tableWrap = documentLike.querySelector('.availability-wrap');
  if (tableWrap) {
    tableWrap.scrollTop = snapshot.tableTop;
    tableWrap.scrollLeft = snapshot.tableLeft;
  }
  windowLike.scrollTo(snapshot.windowX, snapshot.windowY);
}
