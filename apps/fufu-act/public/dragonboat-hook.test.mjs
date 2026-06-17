import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { Script, createContext } from 'node:vm';

async function loadHook() {
  const source = await readFile(new URL('./dragonboat-hook.js', import.meta.url), 'utf8');
  const context = createContext({});
  new Script(source).runInContext(context);
  return context.dragonboatHook;
}

const near = (a, b, eps = 1e-6) => Math.abs(a - b) <= eps;

test('angleFromMatrix reads rotation from a CSS matrix', async () => {
  const hook = await loadHook();
  assert.equal(hook.angleFromMatrix('none'), 0);
  assert.equal(hook.angleFromMatrix(''), 0);
  // rotate(30deg) -> matrix(cos,sin,-sin,cos,0,0)
  const c = Math.cos(30 * Math.PI / 180);
  const s = Math.sin(30 * Math.PI / 180);
  assert.ok(near(hook.angleFromMatrix(`matrix(${c}, ${s}, ${-s}, ${c}, 0, 0)`), 30, 1e-4));
  assert.ok(near(hook.angleFromMatrix(`matrix(${c}, ${-s}, ${s}, ${c}, 0, 0)`), -30, 1e-4));
});

test('castTarget straight down hits the bottom edge directly below origin', async () => {
  const hook = await loadHook();
  const t = hook.castTarget(0, { x: 50, y: 18 }, { width: 1000, height: 1000 });
  assert.ok(near(t.x, 50, 1e-6));
  assert.ok(near(t.y, 100, 1e-6));
});

test('castTarget at +45deg from center reaches the bottom-right corner', async () => {
  const hook = await loadHook();
  const t = hook.castTarget(45, { x: 50, y: 50 }, { width: 1000, height: 1000 });
  assert.ok(near(t.x, 100, 1e-4));
  assert.ok(near(t.y, 100, 1e-4));
});

test('castTarget horizontal (90deg) walks to the right wall at origin height', async () => {
  const hook = await loadHook();
  const t = hook.castTarget(90, { x: 50, y: 18 }, { width: 1000, height: 1000 });
  assert.ok(near(t.x, 100, 1e-4));
  assert.ok(near(t.y, 18, 1e-4));
});

test('castTarget negative angle aims left', async () => {
  const hook = await loadHook();
  const t = hook.castTarget(-90, { x: 50, y: 18 }, { width: 1000, height: 1000 });
  assert.ok(near(t.x, 0, 1e-4));
  assert.ok(near(t.y, 18, 1e-4));
});

test('rayDepthPx projects an on-axis point to its straight-line distance', async () => {
  const hook = await loadHook();
  // straight down: a point 50% below origin in a 1000px-tall stage = 500px
  const depth = hook.rayDepthPx(0, { x: 50, y: 18 }, { x: 50, y: 68 }, { width: 1000, height: 1000 });
  assert.ok(near(depth, 500, 1e-6));
});

test('rayDepthPx ignores the off-axis component (projection only)', async () => {
  const hook = await loadHook();
  // straight-down ray: horizontal offset must not change the depth
  const depth = hook.rayDepthPx(0, { x: 50, y: 18 }, { x: 70, y: 68 }, { width: 1000, height: 1000 });
  assert.ok(near(depth, 500, 1e-6));
});

test('rayDepthPx never returns negative for points behind the origin', async () => {
  const hook = await loadHook();
  const depth = hook.rayDepthPx(0, { x: 50, y: 18 }, { x: 50, y: 5 }, { width: 1000, height: 1000 });
  assert.equal(depth, 0);
});

test('swingAngle oscillates between +/- amplitude', async () => {
  const hook = await loadHook();
  const opts = { amplitudeDeg: 72, periodMs: 2600 };
  assert.ok(near(hook.swingAngle(0, opts), 0, 1e-9));
  assert.ok(near(hook.swingAngle(2600 / 4, opts), 72, 1e-6));
  assert.ok(near(hook.swingAngle(2600 / 2, opts), 0, 1e-6));
  assert.ok(near(hook.swingAngle(2600 * 3 / 4, opts), -72, 1e-6));
});

test('swingPhaseForAngle inverts swingAngle so the swing can resume seamlessly', async () => {
  const hook = await loadHook();
  const opts = { amplitudeDeg: 72, periodMs: 2600 };
  for (const angle of [-72, -40, 0, 25, 60, 72]) {
    const phase = hook.swingPhaseForAngle(angle, opts);
    assert.ok(near(hook.swingAngle(phase, opts), angle, 1e-6), `angle ${angle}`);
  }
  // beyond amplitude clamps (no NaN)
  assert.ok(Number.isFinite(hook.swingPhaseForAngle(200, opts)));
});

test('swingPhaseForAngle(descending) keeps position AND direction continuous on resume', async () => {
  const hook = await loadHook();
  const opts = { amplitudeDeg: 72, periodMs: 2600 };
  const vel = (phase) => Math.cos((2 * Math.PI * phase) / opts.periodMs); // sign of d/dt sin
  for (const angle of [-60, -25, 0, 25, 60]) {
    const up = hook.swingPhaseForAngle(angle, opts, false);
    const down = hook.swingPhaseForAngle(angle, opts, true);
    // both reproduce the angle (position continuous)
    assert.ok(near(hook.swingAngle(up, opts), angle, 1e-6), `up angle ${angle}`);
    assert.ok(near(hook.swingAngle(down, opts), angle, 1e-6), `down angle ${angle}`);
    // rising phase moves right (vel >= 0); descending phase moves left (vel <= 0)
    assert.ok(vel(up) >= -1e-9, `up should not move left at ${angle}`);
    assert.ok(vel(down) <= 1e-9, `down should not move right at ${angle}`);
  }
});

test('a hit item depth is shorter than the boundary (so the hook stops before the wall)', async () => {
  const hook = await loadHook();
  const origin = { x: 50, y: 18 };
  const dims = { width: 900, height: 800 };
  const angle = 20;
  const target = hook.castTarget(angle, origin, dims);
  const boundary = hook.rayDepthPx(angle, origin, target, dims);
  const item = { x: 58, y: 55 };
  const stop = hook.rayDepthPx(angle, origin, item, dims);
  assert.ok(stop > 0);
  assert.ok(stop < boundary, `stop ${stop} should be < boundary ${boundary}`);
});
