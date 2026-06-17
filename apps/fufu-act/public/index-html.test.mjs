import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';

test('scratch test-card flag uses shared standalone-marker helper', async () => {
  const source = await readFile(new URL('./index.html', import.meta.url), 'utf8');

  assert.match(source, /<script src="scratch-card\.js"><\/script>/);
  assert.match(source, /scratchIsTest = scratchCard\.isTestCardName\(data\.cardName\)/);
  assert.doesNotMatch(source, /scratchIsTest = .*includes\('test'\)/);
});

test('spin flow uses safe API parsing instead of exposing raw response errors', async () => {
  const source = await readFile(new URL('./index.html', import.meta.url), 'utf8');

  assert.match(source, /<script src="activity-api\.js"><\/script>/);
  assert.match(source, /activityApi\.readApiJson\(res,\s*\{/);
  assert.match(source, /activityApi\.safeErrorMessage\(err,\s*'抽奖失败，请稍后重试'\)/);
  assert.doesNotMatch(source, /data = await res\.json\(\);\s*if \(!res\.ok\) throw new Error\(data\.error/);
  assert.doesNotMatch(source, /\$\('result'\)\.textContent = err\.message \|\| '网络错误'/);
});

test('login flow masks server errors instead of rendering raw data.error', async () => {
  const source = await readFile(new URL('./index.html', import.meta.url), 'utf8');
  const loginStart = source.indexOf("$('btn-login').onclick = async () =>");
  const loginEnd = source.indexOf("$('card-input').onkeydown");
  const loginSource = source.slice(loginStart, loginEnd);

  assert.match(loginSource, /activityApi\.readApiJson\(res,\s*\{/);
  assert.match(loginSource, /serverErrorMessage: '登录失败，请稍后重试'/);
  assert.match(loginSource, /clientErrorMessage: 'INVALID KEY'/);
  assert.match(loginSource, /activityApi\.safeErrorMessage\(e,\s*'登录失败，请稍后重试'\)/);
  assert.doesNotMatch(loginSource, /const data = await res\.json\(\)/);
  assert.doesNotMatch(loginSource, /\$\('login-error'\)\.textContent = data\.error/);
  assert.doesNotMatch(loginSource, /\$\('login-error'\)\.textContent = 'NETWORK ERROR'/);
});

test('login flow routes dragon boat cards by backend game mode', async () => {
  const source = await readFile(new URL('./index.html', import.meta.url), 'utf8');
  const loginStart = source.indexOf("$('btn-login').onclick = async () =>");
  const loginEnd = source.indexOf("$('card-input').onkeydown");
  const loginSource = source.slice(loginStart, loginEnd);

  assert.match(source, /id="dragonboat-panel"/);
  assert.match(source, /id="btn-dragon-fish"/);
  assert.match(source, /id="btn-dragon-peel"/);
  assert.match(loginSource, /data\.game === 'dragonboat'/);
  assert.match(loginSource, /showDragonBoat\(data\)/);
  assert.doesNotMatch(loginSource, /data\.dollars\s*={2,3}\s*55/);
  assert.match(source, /\$\('dragonboat-panel'\)\.style\.display = 'none'/);
  assert.match(source, /document\.body\.classList\.add\('dragonboat-fullscreen'\)/);
});

test('dragon boat stage renders zongzi as page-native art instead of generic emoji', async () => {
  const source = await readFile(new URL('./index.html', import.meta.url), 'utf8');
  const stageSource = source.slice(
    source.indexOf('<div class="dragonboat-stage"'),
    source.indexOf('<div class="dragonboat-result"'),
  );

  assert.doesNotMatch(stageSource, /\u{1F359}/u);
  assert.match(stageSource, /id="dragonboat-stage" role="button" tabindex="0"/);
  assert.match(stageSource, /id="dragonboat-obstacles"/);
  assert.match(stageSource, /id="dragonboat-zongzi-field"/);
  assert.match(stageSource, /id="dragonboat-peel-screen"/);
  assert.match(stageSource, /id="dragon-peel-count"/);
  assert.match(source, /function renderDragonBoatScene\(scene\)/);
  assert.match(source, /function dragonBoatSceneObjectStyle\(item,\s*index\)/);
  assert.match(source, /<span class="zongzi-rice"><\/span><span class="zongzi-rope"><\/span>/);
  assert.match(source, /\.dragonboat-zongzi::before/);
  assert.match(source, /\.dragonboat-zongzi::after/);
  assert.match(source, /\.zongzi-rope::before/);
  assert.match(source, /\$\('dragon-result'\)\.textContent = '捞到粽子'/);
  assert.doesNotMatch(source, /夹到粽子/);
});

test('dragon boat stage keeps boat on top and targets below', async () => {
  const source = await readFile(new URL('./index.html', import.meta.url), 'utf8');
  const stageSource = source.slice(
    source.indexOf('<div class="dragonboat-stage"'),
    source.indexOf('<div class="dragonboat-result"'),
  );

  assert.match(source, /\.dragonboat-boat \{\s*position: absolute;\s*left: calc\(50% - 78px\);\s*top: 46px;/);
  assert.match(source, /\.dragonboat-hook \{\s*position: absolute;\s*top: 94px;\s*left: 50%;/);
  assert.match(source, /\.dragonboat-mine-zone \{/);
  assert.match(source, /\.dragonboat-obstacles \{/);
  assert.match(source, /\.dragonboat-obstacle::before/);
  assert.match(source, /--dragon-x:\$\{x\}%/);
  assert.match(source, /--dragon-y:\$\{y\}%/);
  assert.match(source, /sceneSeed/);
  assert.doesNotMatch(source, /\.dragonboat-obstacle:nth-child/);
  assert.doesNotMatch(source, /\.dragonboat-zongzi:nth-child/);
  assert.ok(stageSource.indexOf('dragonboat-boat') < stageSource.indexOf('dragonboat-hook'));
  assert.ok(stageSource.indexOf('dragonboat-hook') < stageSource.indexOf('dragonboat-mine-zone'));
  assert.ok(stageSource.indexOf('dragonboat-mine-zone') < stageSource.indexOf('dragonboat-obstacles'));
  assert.ok(stageSource.indexOf('dragonboat-obstacles') < stageSource.indexOf('dragonboat-zongzi-field'));
});

test('dragon boat fullscreen uses screen click then switches to peel phase after fishing', async () => {
  const source = await readFile(new URL('./index.html', import.meta.url), 'utf8');
  const logoutStart = source.indexOf("$('btn-logout').onclick = () =>");
  const logoutEnd = source.indexOf("$('btn-scratch-logout').onclick");
  const logoutSource = source.slice(logoutStart, logoutEnd);

  assert.match(source, /body\.dragonboat-fullscreen #dragonboat-panel/);
  assert.match(source, /body\.dragonboat-fullscreen \.dragonboat-stage/);
  assert.match(source, /body\.dragonboat-fullscreen #dragonboat-panel \{[\s\S]*cursor: pointer;/);
  assert.match(source, /body\.dragonboat-fullscreen \.dragonboat-stage \{[\s\S]*height: 100%;[\s\S]*min-height: 0;/);
  assert.match(source, /body\.dragonboat-fullscreen \.dragonboat-hook \{[\s\S]*height: 86px;[\s\S]*--dragon-hook-idle: 86px;/);
  assert.doesNotMatch(source, /--dragon-hook-idle: 42%/);
  assert.match(source, /body\.dragonboat-fullscreen \.dragonboat-actions,[\s\S]*display: none;/);
  assert.match(logoutSource, /document\.body\.classList\.remove\('dragonboat-fullscreen'\)/);
  assert.match(source, /function updateDragonBoatPhase\(\)/);
  assert.match(source, /phase = 'fish'/);
  assert.match(source, /phase = 'peel'/);
  assert.match(source, /\$\('btn-dragon-peel'\)\.disabled = dragonBusy \|\| remaining > 0 \|\| ready <= 0/);
  assert.match(source, /function handleDragonBoatScreenAction\(\)/);
  assert.match(source, /function prepareDragonBoatReelTarget\(el\)/);
  assert.match(source, /function animateDragonBoatPayload\(el\)/);
  assert.match(source, /\.dragonboat-reel-source \{[\s\S]*opacity: 0 !important;[\s\S]*animation: none !important;/);
  assert.match(source, /\.dragonboat-reel-payload \{[\s\S]*animation: dragon-payload-reel/);
  assert.match(source, /clone\.classList\.add\('dragonboat-reel-payload'\)/);
  assert.match(source, /el\.classList\.add\('dragonboat-reel-source'\)/);
  assert.match(source, /zongzi\?\.classList\.add\('dragonboat-reel-source'\);\s*return;/);
  assert.match(source, /obstacle\?\.classList\.add\('dragonboat-reel-source'\);/);
  assert.match(source, /document\.querySelectorAll\('\.dragonboat-reel-payload'\)\.forEach\(el => el\.remove\(\)\)/);
  assert.match(source, /setTimeout\(resolve,\s*960\)/);
  assert.match(source, /if \(remaining > 0\) \{\s*launchDragonHook\(\);/);
  assert.match(source, /if \(ready > 0\) \{\s*peelDragonBoat\(\);/);
  assert.match(source, /function isDragonBoatControlClick\(target\)/);
  assert.match(source, /\$\('dragonboat-panel'\)\.onclick = \(e\) => \{/);
  assert.match(source, /if \(isDragonBoatControlClick\(e\.target\)\) return;/);
  assert.match(source, /handleDragonBoatScreenAction\(\);/);
  assert.match(source, /\$\('dragon-result'\)\.textContent = '勾到障碍物'/);
  assert.match(source, /\$\('dragonboat-stage'\)\.onkeydown = \(e\) =>/);
});

test('dragon boat hook swings and launches along its angle (gold-miner), not at the mouse', async () => {
  const source = await readFile(new URL('./index.html', import.meta.url), 'utf8');

  // shared, unit-tested geometry module is wired in
  assert.match(source, /<script src="dragonboat-hook\.js"><\/script>/);
  // the hook element is addressable by id (driven by JS swing + cast)
  assert.match(source, /<div class="dragonboat-hook" id="dragonboat-hook"/);
  // swing driven in JS so the exact angle is known at click time
  assert.match(source, /function startDragonSwing\(fromAngle, descending\)/);
  assert.match(source, /function stopDragonSwing\(\)/);
  assert.match(source, /dragonboatHook\.swingAngle\(elapsed, DRAGON_SWING\)/);
  assert.match(source, /dragonboatHook\.swingPhaseForAngle\(fromAngle, DRAGON_SWING, descending\)/);
  // click launches at the CURRENT swing angle, target derived from that angle (not the cursor)
  assert.match(source, /function launchDragonHook\(\)/);
  assert.match(source, /const angle = dragonHookAngle;/);
  assert.match(source, /dragonboatHook\.castTarget\(angle, dragonHookOrigin\(\), dragonHookDims\(\)\)/);
  // the hook stops at the server-reported hit item's ray depth (no pass-through)
  assert.match(source, /function dragonHookCastLength\(cast, angle, origin, dims, idle, maxLen\)/);
  assert.match(source, /dragonboatHook\.rayDepthPx\(angle, origin, item, dims\)/);
  assert.match(source, /stage\.style\.setProperty\('--dragon-hook-length', length\.toFixed\(0\) \+ 'px'\)/);
  // hook pivot is anchored at the percentage origin so visual == server collision at any size
  assert.match(source, /stage\.style\.setProperty\('--dragon-hook-left', origin\.x \+ '%'\)/);
  assert.match(source, /stage\.style\.setProperty\('--dragon-hook-top', origin\.y \+ '%'\)/);
  assert.match(source, /body\.dragonboat-fullscreen \.dragonboat-hook \{[\s\S]*top: var\(--dragon-hook-top, 18%\);/);
  // swing only runs during the fishing phase and is stopped on logout
  assert.match(source, /if \(phase === 'fish' && !dragonBusy\) startDragonSwing\(\);\s*else stopDragonSwing\(\);/);
  assert.match(source, /\$\('btn-logout'\)\.onclick = \(\) => \{\s*currentCardKey = '';\s*stopDragonSwing\(\);/);
  // swing resumes with continuous direction (no pendulum reversal after a cast)
  assert.match(source, /dragonHookDescending = Math\.cos\(/);
  assert.match(source, /dragonboatHook\.swingPhaseForAngle\(fromAngle, DRAGON_SWING, descending\)/);
  assert.match(source, /const descending = dragonHookDescending;/);
  // reel-payload timing must keep the caught item "riding home" inside the cast's retract window
  assert.match(source, /animation: dragon-payload-reel 0\.42s steps\(7\) 1 forwards/);
  assert.match(source, /animation-delay: 0\.52s/);
  assert.match(source, /@keyframes hook-cast-full \{[\s\S]*54% \{ height: var\(--dragon-hook-length, 76%\);/);
  // boat top is clamped so it can't go negative / overlap the skyline on short viewports
  assert.match(source, /top: max\(40px, calc\(var\(--dragon-hook-top, 18%\) - 60px\)\)/);
  // dead reel CSS removed
  assert.doesNotMatch(source, /dragon-object-reel/);
  assert.doesNotMatch(source, /\.dragonboat-zongzi\.caught \{/);
  assert.doesNotMatch(source, /\.dragonboat-obstacle\.blocked \{/);
  // the old mouse-aimed cast is gone
  assert.doesNotMatch(source, /function dragonBoatCastTargetFromEvent/);
  assert.doesNotMatch(source, /function prepareDragonBoatCast/);
  assert.doesNotMatch(source, /-Math\.atan2\(dx, Math\.max\(1, dy\)\)/);
});

test('prize drawer reports load failures instead of silently swallowing them', async () => {
  const source = await readFile(new URL('./index.html', import.meta.url), 'utf8');
  const fetchPrizesStart = source.indexOf('async function fetchPrizes()');
  const fetchPrizesEnd = source.indexOf('function renderPrizeTable(rows)');
  const fetchPrizesSource = source.slice(fetchPrizesStart, fetchPrizesEnd);

  assert.match(source, /function renderPrizeLoadError\(\)/);
  assert.match(source, /serverErrorMessage: '奖池暂时无法加载'/);
  assert.match(fetchPrizesSource, /catch \(e\) \{\s*renderPrizeLoadError\(\);\s*\}/);
  assert.doesNotMatch(fetchPrizesSource, /if \(!res\.ok\) return;/);
  assert.doesNotMatch(fetchPrizesSource, /catch \(e\) \{ \/\* silent \*\/ \}/);
});

test('prize drawer uses backend weight metadata instead of hardcoded weights', async () => {
  const source = await readFile(new URL('./index.html', import.meta.url), 'utf8');
  const fetchPrizesStart = source.indexOf('async function fetchPrizes()');
  const fetchPrizesEnd = source.indexOf('function renderPrizeTable(rows)');
  const fetchPrizesSource = source.slice(fetchPrizesStart, fetchPrizesEnd);

  assert.match(fetchPrizesSource, /p\.weight/);
  assert.match(fetchPrizesSource, /p\.totalWeight/);
  assert.match(fetchPrizesSource, /rank: p\.rank/);
  assert.match(fetchPrizesSource, /label: p\.label/);
  assert.doesNotMatch(fetchPrizesSource, /const weights = \{/);
  assert.doesNotMatch(fetchPrizesSource, /1:\s*1100/);
});

test('slot jackpot display follows backend rank instead of fixed prize amount', async () => {
  const source = await readFile(new URL('./index.html', import.meta.url), 'utf8');
  const spinStart = source.indexOf('async function doSpin()');
  const spinEnd = source.indexOf('function triggerJackpot');
  const spinSource = source.slice(spinStart, spinEnd);

  assert.match(source, /const RANK_SYMBOLS = \{ jackpot: '🏆', second: '👑', third: '⭐' \}/);
  assert.match(source, /id="jackpot-sub"/);
  assert.match(spinSource, /RANK_SYMBOLS\[data\.rank\]/);
  assert.match(spinSource, /rank === 'jackpot'/);
  assert.match(spinSource, /rank === 'second' \|\| rank === 'third'/);
  assert.doesNotMatch(spinSource, /prize >= 1000/);
  assert.match(source, /\$\('jackpot-sub'\)\.textContent = `\$\$\{prize\} GET`/);
});

test('history refresh marks stale state without clearing previous rows', async () => {
  const source = await readFile(new URL('./index.html', import.meta.url), 'utf8');
  const refreshStart = source.indexOf('async function refreshHistory()');
  const refreshEnd = source.indexOf('/* ============================================================\n       Logout');
  const refreshSource = source.slice(refreshStart, refreshEnd);

  assert.match(source, /id="history-status"/);
  assert.match(source, /function renderHistoryRefreshError\(\)/);
  assert.match(refreshSource, /activityApi\.readApiJson\(res,\s*\{/);
  assert.match(refreshSource, /serverErrorMessage: '历史暂未刷新'/);
  assert.match(refreshSource, /catch \(e\) \{\s*renderHistoryRefreshError\(\);\s*\}/);
  assert.match(source, /\$\('history-status'\)\.textContent = '';/);
  assert.doesNotMatch(refreshSource, /if \(res\.ok\) \{\s*const data = await res\.json\(\);/);
  assert.doesNotMatch(refreshSource, /catch \(e\) \{ \/\* silent \*\/ \}/);
});

test('scratch API calls do not expose server 5xx error fields', async () => {
  const source = await readFile(new URL('./index.html', import.meta.url), 'utf8');
  const scratchSource = source.slice(
    source.indexOf('async function startScratchGame()'),
    source.indexOf('/* ============================================================\n       Login icon mini reels'),
  );
  const slices = [
    scratchSource.slice(scratchSource.indexOf('async function startScratchGame()'), scratchSource.indexOf('function shuffleUnrevealed()')),
    scratchSource.slice(scratchSource.indexOf('async function revealCell'), scratchSource.indexOf("$('btn-scratch-cashout').onclick")),
    scratchSource.slice(scratchSource.indexOf("$('btn-scratch-cashout').onclick"), scratchSource.indexOf("$('btn-scratch-reset').onclick")),
    scratchSource.slice(scratchSource.indexOf("$('btn-scratch-reset').onclick")),
  ];

  assert.match(source, /function readScratchApi\(res,\s*serverErrorMessage,\s*clientErrorMessage\)/);
  for (const slice of slices) {
    assert.match(slice, /readScratchApi\(res,\s*'/);
    assert.doesNotMatch(slice, /await res\.json\(\)/);
    assert.doesNotMatch(slice, /\$\('scratch-progress'\)\.textContent = data\.error \|\|/);
    assert.doesNotMatch(slice, /\$\('scratch-progress'\)\.textContent = '网络错误'/);
  }
});

test('dragon boat API calls use safe parsing and keep card progress in dragon state', async () => {
  const source = await readFile(new URL('./index.html', import.meta.url), 'utf8');
  const dragonStart = source.indexOf('async function startDragonBoatGame()');
  const dragonSource = source.slice(
    dragonStart,
    source.indexOf('/* ============================================================\n       Scratch Card', dragonStart),
  );
  const slices = [
    dragonSource.slice(dragonSource.indexOf('async function startDragonBoatGame()'), dragonSource.indexOf('function readDragonBoatApi')),
    dragonSource.slice(dragonSource.indexOf('async function fishDragonBoat'), dragonSource.indexOf('async function peelDragonBoat')),
    dragonSource.slice(dragonSource.indexOf('async function peelDragonBoat'), dragonSource.indexOf('async function refreshDragonBoatCard')),
    dragonSource.slice(dragonSource.indexOf('async function refreshDragonBoatCard')),
  ];

  assert.match(source, /function readDragonBoatApi\(res,\s*serverErrorMessage,\s*clientErrorMessage\)/);
  assert.match(dragonSource, /\/dragonboat\/start/);
  assert.match(dragonSource, /\/dragonboat\/fish/);
  assert.match(dragonSource, /\/dragonboat\/peel/);
  assert.match(dragonSource, /remainingFish/);
  assert.match(dragonSource, /zongziReady/);
  assert.match(dragonSource, /renderDragonBoatState\(data,\s*true\)/);
  for (const slice of slices) {
    assert.doesNotMatch(slice, /await res\.json\(\)/);
    assert.doesNotMatch(slice, /data\.error \|\|/);
    assert.doesNotMatch(slice, /网络错误/);
  }
});
