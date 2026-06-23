import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile, readdir } from 'node:fs/promises';
import { join } from 'node:path';

const appRoot = new URL('.', import.meta.url);

async function readText(path) {
  return readFile(new URL(path, appRoot), 'utf8');
}

async function readSourceTree(dir) {
  const root = new URL(dir, appRoot);
  const entries = await readdir(root, { withFileTypes: true });
  const parts = [];
  for (const entry of entries) {
    const child = join(dir, entry.name).replaceAll('\\', '/');
    if (entry.isDirectory()) {
      parts.push(await readSourceTree(`${child}/`));
    } else if (/\.(tsx|ts|css|html)$/.test(entry.name)) {
      parts.push(await readText(child));
    }
  }
  return parts.join('\n');
}

test('tool-site frontend declares a real React + HeroUI build chain', async () => {
  const pkg = JSON.parse(await readText('package.json'));
  for (const dependency of [
    '@heroui/react',
    '@heroui/styles',
    '@vitejs/plugin-react',
    'vite',
    'typescript',
    'react',
    'react-dom',
    'tailwindcss'
  ]) {
    assert.ok(
      pkg.dependencies?.[dependency] || pkg.devDependencies?.[dependency],
      `missing ${dependency} dependency`
    );
  }
  for (const script of ['dev:ui', 'build:ui', 'typecheck:ui']) {
    assert.ok(pkg.scripts?.[script], `missing ${script} script`);
  }
});

test('tool-site UI uses HeroUI components and theme hook instead of static slot shims', async () => {
  const source = await readSourceTree('ui/src/');
  assert.match(source, /from ['"]@heroui\/react['"]/);
  assert.match(source, /useTheme/);
  for (const component of ['Button', 'Card', 'Tabs', 'Input', 'Textarea', 'Table', 'Chip']) {
    assert.match(source, new RegExp(`\\b${component}\\b`), `missing HeroUI ${component} usage`);
  }
  assert.doesNotMatch(source, /<[A-Za-z][^>]*\sdata-slot=/);
});

test('tool-site UI imports HeroUI styles and keeps a blueprint-design theme without Sage or wabi tokens', async () => {
  const styles = await readSourceTree('ui/src/');
  assert.match(styles, /@import ['"]@heroui\/styles['"]/);
  assert.match(styles, /--blueprint-canvas/);
  assert.match(styles, /--blueprint-panel/);
  assert.match(styles, /--blueprint-accent/);
  assert.match(styles, /--blueprint-grid-line/);
  assert.doesNotMatch(styles, /--sage-/i);
  assert.doesNotMatch(styles, /sage-design/i);
  assert.doesNotMatch(styles, /--fufu-/i);
  assert.doesNotMatch(styles, /wabi/i);
});

test('tool-site blueprint theme centralizes HeroUI-compatible tokens and radius decisions', async () => {
  const styles = await readText('ui/src/styles.css');
  for (const token of [
    '--background',
    '--foreground',
    '--surface',
    '--surface-secondary',
    '--border',
    '--separator',
    '--focus',
    '--field-background',
    '--radius',
    '--blueprint-canvas',
    '--blueprint-panel',
    '--blueprint-text-primary',
    '--blueprint-text-muted',
    '--blueprint-accent',
    '--blueprint-grid-line',
    '--blueprint-radius-control',
    '--blueprint-radius-panel',
    '--blueprint-radius-stamp',
    '--blueprint-radius-nav'
  ]) {
    assert.match(styles, new RegExp(`${token.replaceAll('-', '\\-')}\\s*:`), `missing token ${token}`);
  }

  const radiusDeclarations = [...styles.matchAll(/border-radius:\s*([^;]+);/g)].map((match) => match[1].trim());
  assert.ok(radiusDeclarations.length > 8, 'expected component radius declarations to be explicit');
  for (const value of radiusDeclarations) {
    assert.match(value, /^var\(--blueprint-radius-[^)]+\)(?:\s*!important)?$/, `raw radius declaration is not tokenized: ${value}`);
  }

  for (const [token, value] of [
    ['--blueprint-radius-control', '2px'],
    ['--blueprint-radius-panel', '2px'],
    ['--blueprint-radius-stamp', '1px'],
    ['--blueprint-radius-nav', '2px']
  ]) {
    assert.match(styles, new RegExp(`${token.replaceAll('-', '\\-')}\\s*:\\s*${value.replace('.', '\\.')};`), `unexpected ${token}`);
  }
});

test('tool-site navigation and admin share blueprint-design primitives', async () => {
  const source = await readSourceTree('ui/src/');
  for (const marker of [
    './blueprint',
    '../blueprint',
    'BlueprintHeader',
    'BlueprintStamp',
    'blueprint-page',
    'blueprint-top-actions',
    'blueprint-primary-button'
  ]) {
    assert.match(source, new RegExp(marker.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')), `missing shared blueprint marker ${marker}`);
  }
  assert.doesNotMatch(source, /Wabi|wabi/);
});

test('tool-site admin layout keeps business tabs primary and utility actions secondary', async () => {
  const adminSource = await readText('ui/src/admin/AdminPage.tsx');
  const styles = await readText('ui/src/styles.css');

  assert.match(adminSource, /className="admin-utility-bar"/, 'admin actions should live in a lightweight utility bar');
  assert.match(adminSource, /className="admin-action-group"/, 'admin action buttons should be visually grouped as utilities');
  assert.doesNotMatch(adminSource, /orientation="vertical"/, 'business tabs should not use the old left vertical rail');
  assert.doesNotMatch(adminSource, /className="command-bar"/, 'old full-width command bar should not remain');
  assert.doesNotMatch(styles, /grid-template-columns:\s*300px\s+minmax\(0,\s*1fr\)/, 'admin tabs should not reserve a left rail');
  assert.doesNotMatch(styles, /\.admin-tab-list\s*\{[^}]*position:\s*sticky/s, 'tab list should not be a sticky sidebar');
});

test('tool-site admin page delegates editors and panels to focused modules', async () => {
  const adminSource = await readText('ui/src/admin/AdminPage.tsx');
  const shared = await readText('ui/src/admin/adminShared.tsx');
  const siteNavigation = await readText('ui/src/admin/siteNavigationPanels.tsx');
  const siteNavigationCore = await readText('ui/src/admin/siteNavigationConfigCore.ts');
  const mcy = await readText('ui/src/admin/mcyConfigPanel.tsx');
  const saleCard = await readText('ui/src/admin/saleCardPanel.tsx');
  const activity = await readText('ui/src/admin/activityPanels.tsx');

  for (const marker of [
    './activityPanels',
    './adminShared',
    './mcyConfigPanel',
    './saleCardPanel',
    './siteNavigationPanels'
  ]) {
    assert.match(adminSource, new RegExp(marker.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')));
  }
  for (const component of ['SiteEditor', 'NavigationToolsEditor', 'SaleCardManager', 'ActivityConfigEditor']) {
    assert.doesNotMatch(adminSource, new RegExp(`function\\s+${component}\\b`), `${component} should not live in AdminPage.tsx`);
  }

  assert.match(shared, /function LoginPanel/);
  assert.match(shared, /function ConfigCard/);
  assert.match(siteNavigation, /function SiteEditor/);
  assert.match(siteNavigation, /function NavigationToolsEditor/);
  assert.match(siteNavigationCore, /function updateManagedSite/);
  assert.match(siteNavigationCore, /function buildAdminNavigationSavePayload/);
  assert.match(mcy, /function MCYConfigEditor/);
  assert.match(saleCard, /function SaleCardManager/);
  assert.match(activity, /function ActivityConfigEditor/);
});

test('tool-site admin navigation links expose one URL field and do not resave ping URLs', async () => {
  const adminSource = await readText('ui/src/admin/AdminPage.tsx');
  const siteNavigation = await readText('ui/src/admin/siteNavigationPanels.tsx');
  const siteNavigationCore = await readText('ui/src/admin/siteNavigationConfigCore.ts');
  const homeSource = await readText('ui/src/HomePage.tsx');

  assert.doesNotMatch(siteNavigation, /可选 ping URL/);
  assert.doesNotMatch(siteNavigation, /value=\{link\.ping/);
  assert.doesNotMatch(siteNavigation, /ping:\s*''/);
  assert.match(adminSource, /buildAdminNavigationSavePayload\(config\)/);
  assert.match(siteNavigationCore, /\.map\(\(link\)\s*=>\s*\(\{\s*label:\s*link\.label,\s*href:\s*link\.href\s*\}\)\)/s);
  assert.match(homeSource, /useLatency\(link\.ping\s*\|\|\s*link\.href\)/);
});

test('tool-site admin navigation editor keeps pure object rules in a core module', async () => {
  const adminSource = await readText('ui/src/admin/AdminPage.tsx');
  const siteNavigation = await readText('ui/src/admin/siteNavigationPanels.tsx');
  const siteNavigationCore = await readText('ui/src/admin/siteNavigationConfigCore.ts');

  for (const marker of [
    'updateManagedSite',
    'addManagedSiteURL',
    'removeManagedSiteURL',
    'patchNavigationCard',
    'addNavigationLink',
    'setNavigationCardLineKind',
    'buildAdminNavigationSavePayload'
  ]) {
    assert.match(siteNavigationCore, new RegExp(`function\\s+${marker}\\b|const\\s+${marker}\\b`), `missing core rule ${marker}`);
  }
  assert.match(siteNavigation, /from '\.\/siteNavigationConfigCore'/);
  assert.match(adminSource, /from '\.\/siteNavigationConfigCore'/);
  assert.doesNotMatch(siteNavigation, /const\s+SITE_GROUPS\b/);
  assert.doesNotMatch(siteNavigation, /const\s+defaultSite\b/);
  assert.doesNotMatch(siteNavigation, /function\s+ensureSite\b/);
  assert.doesNotMatch(siteNavigation, /function\s+patchCard\b/);
  assert.doesNotMatch(adminSource, /const\s+navigationCards\s*=/);
});

test('tool-site activity window uses human date-time controls instead of raw timestamps', async () => {
  const activity = await readText('ui/src/admin/activityPanels.tsx');
  const activityCore = await readText('ui/src/admin/activityConfigCore.ts');

  assert.doesNotMatch(activity, /开始时间戳/);
  assert.doesNotMatch(activity, /结束时间戳/);
  assert.match(activity, /className="field-grid activity-window-grid"/);
  assert.match(activity, /type="datetime-local"/);
  assert.match(activity, /activityDatePatch\('start'/);
  assert.match(activity, /activityDatePatch\('end'/);
  assert.match(activityCore, /startText:\s*formatActivityDateText\(date\),\s*startTS:\s*seconds/s);
  assert.match(activityCore, /endText:\s*formatActivityDateText\(date\),\s*endTS:\s*seconds/s);
  const styles = await readText('ui/src/styles.css');
  assert.match(styles, /\.activity-window-grid\s*\{[^}]*grid-template-columns:\s*repeat\(2,\s*minmax\(270px,\s*280px\)\);[^}]*column-gap:\s*28px;/s);
});

test('tool-site admin URL rows reuse the same control typography and rhythm as normal inputs', async () => {
  const styles = await readText('ui/src/styles.css');
  assert.match(styles, /--blueprint-control-height:\s*38px;/);
  assert.match(styles, /--blueprint-control-font-size:\s*0\.7rem;/);
  assert.match(styles, /--blueprint-control-letter-spacing:\s*0\.06em;/);
  assert.match(styles, /\.blueprint-input,\s*\.blueprint-textarea,\s*\.native-select\s*\{[^}]*font-family:\s*var\(--font-mono\)\s*!important;[^}]*font-size:\s*var\(--blueprint-control-font-size\)\s*!important;[^}]*letter-spacing:\s*var\(--blueprint-control-letter-spacing\);/s);
  assert.match(styles, /\.site-url-row\s+\.site-url-tag,\s*\.site-url-row\s+\.blueprint-input,\s*\.site-url-row\s+\.blueprint-danger-button\s*\{[^}]*height:\s*var\(--blueprint-control-height\)\s*!important;[^}]*min-height:\s*var\(--blueprint-control-height\)\s*!important;/s);
  assert.match(styles, /\.site-url-tag\s*\{[^}]*display:\s*inline-flex;[^}]*align-items:\s*center;[^}]*justify-content:\s*center;[^}]*font-size:\s*var\(--blueprint-control-font-size\);[^}]*letter-spacing:\s*var\(--blueprint-control-letter-spacing\);[^}]*border:\s*1\.5px solid var\(--blueprint-line\);/s);
  assert.match(styles, /\.site-url-row\s+\.site-url-name\s*\{[^}]*flex:\s*0 0 120px;[^}]*min-width:\s*120px;/s);
  assert.doesNotMatch(styles, /\.site-url-tag\s*\{[^}]*padding:\s*4px\s+8px;/s);
  assert.doesNotMatch(styles, /var\(--blueprint-fill-block\)/);
  assert.doesNotMatch(styles, /var\(--blueprint-line-soft\)/);
});

test('tool-site admin form sections keep list actions spaced and compact panels tight', async () => {
  const styles = await readText('ui/src/styles.css');
  const mcy = await readText('ui/src/admin/mcyConfigPanel.tsx');

  assert.match(styles, /\.site-url-list\s*\+\s*\.action-row\s*\{[^}]*margin-top:\s*12px;/s);
  assert.match(styles, /\.business-stack--compact\s*\{[^}]*gap:\s*6px;/s);
  assert.match(mcy, /className="business-stack business-stack--compact"/);
  assert.match(mcy, /\{saveMsg\.text\s*\?\s*<MessageLine tone=\{saveMsg\.tone\}>\{saveMsg\.text\}<\/MessageLine>\s*:\s*null\}/);
  assert.doesNotMatch(mcy, /<MessageLine tone=\{saveMsg\.tone\}>\{saveMsg\.text\}<\/MessageLine>\s*<p className="inline-help"/);
});

test('tool-site UI preserves actual business API wiring in React source', async () => {
  const source = await readSourceTree('ui/src/');
  for (const marker of [
    '/api/admin/session',
    '/api/admin/config',
    '/api/admin/stats',
    '/api/admin/sale-cards/config',
    '/api/admin/sale-cards/test-key',
    '/api/prizes',
    '/api/newapi/sites'
  ]) {
    assert.match(source, new RegExp(marker.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')));
  }
  assert.doesNotMatch(source, /\/api\/admin\/sale-cards\/run/);
  assert.doesNotMatch(source, /Authorization:\s*['"`]Bearer/);
  assert.doesNotMatch(source, /ADMIN_TOKEN/);
  assert.doesNotMatch(source, /登录后加载/);
});

test('tool-site activity panels distinguish loading from empty loaded data', async () => {
  const activity = await readText('ui/src/admin/activityPanels.tsx');
  const types = await readText('ui/src/admin/types.ts');

  assert.match(activity, /empty=\{stats \? '暂无奖项统计。' : '正在加载活动统计。'\}/);
  assert.match(activity, /const pool = prizes\?\.prizePool \?\? prizes\?\.prizes \?\? \[\]/);
  assert.match(activity, /empty=\{prizes \? '暂无普通奖项。' : '正在加载当前奖池。'\}/);
  assert.match(types, /prizes\?: PrizeRow\[\]/);
  assert.doesNotMatch(activity, /等待加载当前奖池。/);
});

test('tool-site activity gameplay routes use MCY sale-card tiers instead of manual tier input', async () => {
  const adminSource = await readText('ui/src/admin/AdminPage.tsx');
  const activity = await readText('ui/src/admin/activityPanels.tsx');
  const activityCore = await readText('ui/src/admin/activityConfigCore.ts');
  const types = await readText('ui/src/admin/types.ts');

  assert.match(types, /export type ActivityGameRoute/);
  assert.match(types, /drawCount\?: number/);
  assert.match(types, /export type ActivityGameConfig = \{\s+game: 'slot' \| 'scratch' \| 'dragonboat' \| string;\s+targetExpectedValue\?: number;\s+\};/);
  assert.doesNotMatch(types, /ActivityGameConfig[\s\S]*actualExpectedValue\?: number;/);
  assert.match(activity, /salePlans/);
  assert.match(activity, /onGenerateTestKey/);
  assert.match(activity, /from '\.\/activityConfigCore'/);
  assert.match(activityCore, /function buildSaleCardTierOptions/);
  assert.match(activityCore, /function materializeActivityGameRoutes/);
  assert.match(activityCore, /function stripComputedExpectedValues/);
  assert.match(activityCore, /function normalizeScratchMaxReveals/);
  assert.match(activityCore, /function materializeGameRoutesForSaleTiers/);
  assert.match(activityCore, /function gameRoutesFromActivity/);
  assert.match(activity, /drawCountForTier/);
  assert.match(activity, /updateDrawCountForTier/);
  assert.match(activityCore, /function spinMapDrawCount/);
  assert.match(activity, /玩法参数/);
  assert.match(activity, /卡档配置（来自 MCY 上架配置）/);
  assert.match(activity, /<span>玩法<\/span><span>目标期望值<\/span>/);
  assert.doesNotMatch(activity, /<span>玩法<\/span><span>目标期望值<\/span><span>实际期望值<\/span>/);
  assert.doesNotMatch(activity, /aria-label=\{`\$\{gameModeLabel\(game\)\}实际期望值`\}/);
  assert.match(adminSource, /stats=\{activityStats\}/);
  assert.match(activity, /<span>MCY 卡档<\/span><span>玩法<\/span><span>抽奖次数<\/span><span>售价<\/span><span>成本<\/span><span>净利润<\/span><span>入池<\/span><span>测试 key<\/span>/);
  assert.doesNotMatch(activity, /<span>额度<\/span>/);
  assert.doesNotMatch(activity, /<span>MCY SKU<\/span>/);
  assert.doesNotMatch(activity, /<span>分组<\/span>/);
  assert.match(activity, /aria-label=\{`\$\{option\.label\}抽奖次数`\}/);
  assert.match(activity, /aria-label=\{`\$\{option\.label\}售价`\}/);
  assert.match(activity, /aria-label=\{`\$\{option\.label\}成本`\}/);
  assert.match(activityCore, /刮刮乐/);
  assert.match(activityCore, /老虎机/);
  assert.match(activityCore, /端午捕粽/);
  assert.match(activityCore, /'dragonboat'/);
  assert.match(activityCore, /primaryPlanId/);
  assert.match(adminSource, /salePlans=\{saleCards\?\.plans \?\? \[\]\}/);
  assert.match(adminSource, /generateSaleCardTestKey/);
  assert.match(adminSource, /\/api\/admin\/sale-cards\/test-key/);
  assert.match(adminSource, /onGenerateTestKey=\{generateSaleCardTestKey\}/);
  assert.match(adminSource, /activity: materializeActivityGameRoutes\(config\.activity, saleCards\?\.plans \?\? \[\]\)/);
  assert.match(adminSource, /from '\.\/activityConfigCore'/);
  assert.match(activity, /一键生成 key/);
  assert.match(activity, /sale-test-key-result/);
  assert.match(activity, /测试 key 只创建 NewAPI token，不上传 MCY/);
  assert.match(activity, /刮刮乐通关安全步数/);
  assert.match(activity, /玩家连续刮开 X 个安全格即通关/);
  assert.match(types, /scratchMaxReveals\?: number/);
  assert.doesNotMatch(activity, /aria-label=\{`\$\{gameModeLabel\(game\)\}抽奖次数`\}/);
  assert.doesNotMatch(activity, /刮刮乐固定步数/);
  assert.doesNotMatch(activity, /SCRATCH_FIXED_STEPS/);
  assert.doesNotMatch(activity, /scratch-chip--static/);
  assert.doesNotMatch(activity, /spinCountForTier/);
  assert.doesNotMatch(activity, /updateSpinCountForTier/);
  assert.doesNotMatch(activity, /emitSpinCount/);
  assert.doesNotMatch(activity, /新增刮刮乐卡档/);
  assert.doesNotMatch(activity, /emitScratchTiers\(\[\.\.\.scratchTiers,\s*0\]\)/);
  assert.doesNotMatch(activity, /<div className="config-subhead">额度 → 抽奖次数<\/div>/);
  assert.doesNotMatch(activity, /spinEntries\.map/);
});

test('tool-site activity prize editor exposes calculated unified prize pool only', async () => {
  const activity = await readText('ui/src/admin/activityPanels.tsx');
  const activityCore = await readText('ui/src/admin/activityConfigCore.ts');
  const styles = await readText('ui/src/styles.css');

  assert.match(activity, /目标期望值/);
  assert.match(activity, /实际期望值/);
  assert.match(activity, /卡密核销时按/);
  assert.match(activity, /奖池开关/);
  assert.match(activity, /比例按百分比填写/);
  assert.match(activityCore, /function rateToPercentInput/);
  assert.match(activityCore, /function percentInputToRate/);
  assert.match(activity, /value=\{rateToPercentInput\(dynamicPool\.contributionRate\)\}/);
  assert.match(activity, /value=\{rateToPercentInput\(dynamicPool\.jackpotRate\)\}/);
  assert.match(activity, /value=\{rateToPercentInput\(dynamicPool\.secondRate\)\}/);
  assert.match(activity, /value=\{rateToPercentInput\(dynamicPool\.thirdRate\)\}/);
  assert.match(activity, /contributionRate:\s*percentInputToRate\(event\.target\.value\)/);
  assert.match(activity, /jackpotRate:\s*percentInputToRate\(event\.target\.value\)/);
  assert.match(activity, /secondRate:\s*percentInputToRate\(event\.target\.value\)/);
  assert.match(activity, /thirdRate:\s*percentInputToRate\(event\.target\.value\)/);
  assert.match(activity, /入池比例 \(%\)/);
  assert.match(activity, /大奖分配 \(%\)/);
  assert.match(activity, /step="0\.01" min=\{0\} max=\{100\}/);
  assert.match(activity, /奖项分配合计 \{rateToPercentInput\(awardRateTotal\)\}%/);
  assert.match(styles, /\.dynamic-pool-panel/);
  assert.match(styles, /\.dynamic-pool-controls/);
  assert.match(styles, /\.dynamic-toggle-control/);
  assert.doesNotMatch(activity, /value=\{String\(dynamicPool\.(?:contributionRate|jackpotRate|secondRate|thirdRate) \?\? 0\)\}/);
  assert.doesNotMatch(activity, /max=\{1\} value=\{String\(dynamicPool\.(?:contributionRate|jackpotRate|secondRate|thirdRate)/);
  assert.doesNotMatch(activity, /动态奖池用于把卡档利润沉淀成大奖预算/);
  assert.doesNotMatch(styles, /\.dynamic-pool-panel\s*\{[^}]*border:/s);
  assert.doesNotMatch(styles, /\.dynamic-toggle-control\s*\{[^}]*border:/s);
  assert.doesNotMatch(activity, /分额度奖池/);
  assert.doesNotMatch(activity, /tierEntries/);
  assert.doesNotMatch(activity, /buildTierPools/);
  assert.doesNotMatch(activity, /emitTiers/);
  assert.doesNotMatch(activity, /emitPrize/);
  assert.doesNotMatch(activity, /<PrizePoolEditor rows=\{prizePool\}/);
  assert.doesNotMatch(activity, /普通奖池（统一中奖率）/);
  assert.doesNotMatch(activity, /中大奖后奖池/);
  assert.doesNotMatch(activity, /postJackpot/);
});

test('tool-site sale card panel shows automatic restock task status', async () => {
  const saleCard = await readText('ui/src/admin/saleCardPanel.tsx');
  const adminPage = await readText('ui/src/admin/AdminPage.tsx');

  assert.match(saleCard, /STATUS_LABELS/);
  assert.match(saleCard, /自动补卡按已保存计划执行/);
  assert.match(saleCard, /任务超时会取消请求、复查 MCY 库存并按 DB 任务单独重试/);
  assert.match(saleCard, /补卡任务状态/);
  assert.match(saleCard, /失败任务/);
  assert.match(saleCard, /原因：\{reason\}/);
  assert.match(saleCard, /restockStatus/);
  assert.match(saleCard, /from '\.\/saleCardConfigCore'/);
  assert.match(adminPage, /<SaleCardManager config=\{saleCards\} \/>/);
  assert.match(adminPage, /自动补卡任务状态/);

  assert.doesNotMatch(saleCard, /STOCK_REFRESH_INTERVAL_MS/);
  assert.doesNotMatch(saleCard, /refreshStock/);
  assert.doesNotMatch(saleCard, /window\.setInterval/);
  assert.doesNotMatch(saleCard, /async function runNow/);
  assert.doesNotMatch(saleCard, /\/api\/admin\/sale-cards\/stock/);
  assert.doesNotMatch(adminPage, /\/api\/admin\/sale-cards\/run/);
  assert.doesNotMatch(adminPage, /onRun=\{runSalePlan\}/);
});

test('tool-site sale card status panel does not keep manual stock or run handlers wired', async () => {
  const saleCard = await readText('ui/src/admin/saleCardPanel.tsx');
  const adminPage = await readText('ui/src/admin/AdminPage.tsx');

  assert.match(saleCard, /<MessageLine tone=\{jobs\.some\(\(job\) => job\.status === 'failed'\) \? 'error' : undefined\}>/);
  assert.match(saleCard, /后台不做全天库存监控/);
  assert.doesNotMatch(saleCard, /setScheduleMessage/);
  assert.doesNotMatch(saleCard, /async function saveSchedule/);
  assert.doesNotMatch(saleCard, /补卡计划已保存/);
  assert.doesNotMatch(saleCard, /runMessage/);
  assert.doesNotMatch(saleCard, /messageFromError\(error, '补卡执行失败'\)/);
  assert.doesNotMatch(adminPage, /async function saveSaleSchedule/);
  assert.doesNotMatch(adminPage, /async function runSalePlan/);
  assert.doesNotMatch(adminPage, /onSave=\{saveSaleSchedule\}/);
  assert.doesNotMatch(adminPage, /onRun=\{runSalePlan\}/);
});

test('home page gets API and token line fallbacks from backend config only', async () => {
  const source = await readText('ui/src/HomePage.tsx');
  assert.match(source, /\/api\/nav\/tools/);
  assert.doesNotMatch(source, /\/api\/nav\/lines/);
  assert.doesNotMatch(source, /API 次数站/);
  assert.doesNotMatch(source, /Token 站/);
  assert.doesNotMatch(source, /linesFor/);
  assert.doesNotMatch(source, /fufuapi\./);
  assert.doesNotMatch(source, /STATIC_(API|TOKEN)_LINES/);
});
