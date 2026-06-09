import test from 'node:test';
import assert from 'node:assert/strict';

import {
  animateTabIndicators,
  captureRenderScroll,
  captureTabIndicatorRects,
  motionClass,
  restoreRenderScroll
} from './ui_motion.js';

test('motionClass returns motion marker only for active type', () => {
  assert.equal(motionClass('panel', 'panel'), ' motion-enter');
  assert.equal(motionClass('panel', 'scope', 'select'), '');
});

test('captureTabIndicatorRects records indicators by motion key', () => {
  const indicator = {
    closest: () => ({
      dataset: { tabMotionKey: 'panel' },
      getAttribute: () => '状态视图'
    }),
    getBoundingClientRect: () => ({ left: 10, top: 20 })
  };
  const rects = captureTabIndicatorRects({
    querySelectorAll: () => [indicator]
  });

  assert.equal(rects.size, 1);
  assert.deepEqual(rects.get('panel'), { left: 10, top: 20 });
});

test('animateTabIndicators moves indicator from previous rect then clears inline styles', () => {
  const styles = [];
  const tabList = {
    dataset: { tabMotionKey: 'panel', orientation: 'horizontal' },
    getAttribute: () => '状态视图'
  };
  const style = {};
  Object.defineProperty(style, 'transition', {
    set: (value) => styles.push(['transition', value]),
    get: () => ''
  });
  Object.defineProperty(style, 'transform', {
    set: (value) => styles.push(['transform', value]),
    get: () => ''
  });
  const indicator = {
    style,
    closest: () => tabList,
    getBoundingClientRect: () => ({ left: 30, top: 20 })
  };

  animateTabIndicators(new Map([['panel', { left: 10, top: 20 }]]), {
    document: { querySelectorAll: () => [indicator] },
    window: {
      matchMedia: () => ({ matches: false }),
      requestAnimationFrame: (fn) => fn(),
      setTimeout: (fn) => fn()
    }
  });

  assert.deepEqual(styles, [
    ['transition', 'none'],
    ['transform', 'translate(calc(var(--indicator-x) + -20px), calc(var(--indicator-y) + 0px))'],
    ['transition', 'transform 260ms var(--ease-out-fluid), width 260ms var(--ease-out-fluid), height 260ms var(--ease-out-fluid)'],
    ['transform', ''],
    ['transition', '']
  ]);
});

test('capture and restore render scroll preserves active panel', () => {
  const tableWrap = { scrollTop: 7, scrollLeft: 9 };
  const snapshot = captureRenderScroll('models', {
    document: { querySelector: () => tableWrap },
    window: { scrollX: 11, scrollY: 13 }
  });

  assert.deepEqual(snapshot, {
    activePanel: 'models',
    windowX: 11,
    windowY: 13,
    tableTop: 7,
    tableLeft: 9
  });

  const scrolls = [];
  tableWrap.scrollTop = 0;
  tableWrap.scrollLeft = 0;
  restoreRenderScroll(snapshot, 'models', {
    document: { querySelector: () => tableWrap },
    window: { scrollTo: (x, y) => scrolls.push([x, y]) }
  });

  assert.equal(tableWrap.scrollTop, 7);
  assert.equal(tableWrap.scrollLeft, 9);
  assert.deepEqual(scrolls, [[11, 13]]);
});
