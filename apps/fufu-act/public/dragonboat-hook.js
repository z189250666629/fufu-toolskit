(function(root) {
  // 端午捕粽 钩子几何（黄金矿工式）。纯函数，便于单测。
  //
  // 坐标系：舞台左上为原点，x 向右、y 向下，单位为舞台百分比 (0-100)。
  // 钩子从 origin 垂下，摆角 angle 以「正下方」为 0：正角 = 顺时针 = 偏右
  //（与 CSS rotate(angle) 对一根向下、transform-origin 在顶部的杆一致）。
  // 像素方向单位向量 = (sin θ, cos θ)。

  const DEG = Math.PI / 180;
  const TAU = Math.PI * 2;

  // 钟摆摆角（度）：amplitude·sin(2π·elapsed/period)。由 rAF 每帧驱动 CSS 变量，
  // 点击时即可拿到精确角度，不依赖 getComputedStyle。
  function swingAngle(elapsedMs, opts) {
    const amplitude = Number(opts?.amplitudeDeg) || 72;
    const period = Math.max(1, Number(opts?.periodMs) || 2600);
    return amplitude * Math.sin((TAU * (Number(elapsedMs) || 0)) / period);
  }

  // 给定目标角，求其在摆动周期里的相位（毫秒）。收钩后用它无缝续摆，避免
  // 动画重置导致钩子从某个角度瞬跳回 0。asin 只落在上升的四分之一周期（速度向右）；
  // 若发射时钩子正向左摆（descending），把相位映射到下降段(period/2 - phase)，
  // 这样续摆既位置连续、方向也连续，不会突然反向。
  function swingPhaseForAngle(angleDeg, opts, descending) {
    const amplitude = Number(opts?.amplitudeDeg) || 72;
    const period = Math.max(1, Number(opts?.periodMs) || 2600);
    const ratio = Math.max(-1, Math.min(1, (Number(angleDeg) || 0) / amplitude));
    const phase = (Math.asin(ratio) / TAU) * period; // 上升段，范围 [-period/4, period/4]
    return descending ? period / 2 - phase : phase;
  }

  // 从 getComputedStyle(...).transform 的 matrix(a,b,c,d,e,f) 解析旋转角（度）。
  // rotate(θ) 的矩阵为 [cosθ, sinθ, -sinθ, cosθ]，故 θ = atan2(b, a)。
  function angleFromMatrix(transform) {
    const str = String(transform || '').trim();
    if (!str || str === 'none') return 0;
    const match = str.match(/matrix\(([^)]+)\)/);
    if (!match) return 0;
    const parts = match[1].split(',').map((v) => Number(v.trim()));
    if (parts.length < 4 || parts.some((v) => !Number.isFinite(v))) return 0;
    const [a, b] = parts;
    return Math.atan2(b, a) / DEG;
  }

  // 沿摆角 θ 从 origin 射出，返回射线与舞台边界的交点（百分比）。
  // 这样发给后端的 target 落在可见射线的延长线上，后端 origin->target 的
  // 碰撞检测与玩家看到的方向一致。
  function castTarget(angleDeg, origin, dims) {
    const theta = (Number(angleDeg) || 0) * DEG;
    const ox = clamp(Number(origin?.x), 0, 100, 50);
    const oy = clamp(Number(origin?.y), 0, 100, 18);
    const width = Math.max(1, Number(dims?.width) || 1);
    const height = Math.max(1, Number(dims?.height) || 1);

    // 每像素带来的百分比增量
    const dxPctPerPx = (Math.sin(theta) / width) * 100;
    const dyPctPerPx = (Math.cos(theta) / height) * 100;

    let dist = Infinity;
    if (dxPctPerPx > 1e-9) dist = Math.min(dist, (100 - ox) / dxPctPerPx);
    else if (dxPctPerPx < -1e-9) dist = Math.min(dist, (0 - ox) / dxPctPerPx);
    if (dyPctPerPx > 1e-9) dist = Math.min(dist, (100 - oy) / dyPctPerPx);
    else if (dyPctPerPx < -1e-9) dist = Math.min(dist, (0 - oy) / dyPctPerPx);
    if (!Number.isFinite(dist)) dist = Math.max(width, height); // 退化：水平射线等

    return {
      x: clamp(ox + dist * dxPctPerPx, 0, 100, ox),
      y: clamp(oy + dist * dyPctPerPx, 0, 100, oy),
    };
  }

  // 点 point 在摆角 θ 射线上的投影深度（像素）。用于：钩子伸到第一个命中物
  // 体处停下的长度（命中物体的投影深度）；空钩时用边界 target 的投影 = 满长。
  function rayDepthPx(angleDeg, origin, point, dims) {
    const theta = (Number(angleDeg) || 0) * DEG;
    const ox = clamp(Number(origin?.x), 0, 100, 50);
    const oy = clamp(Number(origin?.y), 0, 100, 18);
    const width = Math.max(1, Number(dims?.width) || 1);
    const height = Math.max(1, Number(dims?.height) || 1);
    const dxPx = ((Number(point?.x) || 0) - ox) / 100 * width;
    const dyPx = ((Number(point?.y) || 0) - oy) / 100 * height;
    const depth = dxPx * Math.sin(theta) + dyPx * Math.cos(theta);
    return Math.max(0, depth);
  }

  function clamp(value, min, max, fallback) {
    const n = Number(value);
    if (!Number.isFinite(n)) return fallback;
    return Math.max(min, Math.min(max, n));
  }

  root.dragonboatHook = {
    angleFromMatrix,
    castTarget,
    rayDepthPx,
    swingAngle,
    swingPhaseForAngle,
  };
})(globalThis);
