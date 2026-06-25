import test from 'node:test';
import assert from 'node:assert/strict';
import { readdir, readFile } from 'node:fs/promises';

const repoRoot = new URL('../', import.meta.url);

async function readRepoFile(path) {
  return readFile(new URL(path, repoRoot), 'utf8');
}

function nonCommentLines(source) {
  return source
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter((line) => line && !line.startsWith('#'));
}

async function listTopLevelDirs(path) {
  const entries = await readdir(new URL(path, repoRoot), { withFileTypes: true });
  return entries
    .filter((entry) => entry.isDirectory())
    .map((entry) => entry.name)
    .sort();
}

test('top-level directories separate active apps, legacy code and local tools', async () => {
  assert.deepEqual(await listTopLevelDirs('apps/'), [
    'fufu-act',
    'fufu-tool-site',
    'y2k-nav'
  ]);
  assert.deepEqual(await listTopLevelDirs('legacy/'), ['network-detect']);
  assert.deepEqual(await listTopLevelDirs('tools/'), ['mcy-card-upload']);
  assert.deepEqual(await listTopLevelDirs('tests/'), ['workspace']);
});

test('tool-site keeps page static assets under web buckets', async () => {
  assert.deepEqual(await listTopLevelDirs('apps/fufu-tool-site/web/'), [
    'combine',
    'status'
  ]);
  const appRootDirs = await listTopLevelDirs('apps/fufu-tool-site/');
  for (const staleRoot of ['admin', 'combine', 'frontend']) {
    assert.equal(appRootDirs.includes(staleRoot), false, `${staleRoot} should live under web/ or be retired`);
  }
  const configFiles = await readRepoFile('apps/fufu-tool-site/config/newapi-managed-api-sites.example.json');
  assert.match(configFiles, /managedApiSites/);
});

test('tool-site keeps entrypoint, runtime state and route adapters separated', async () => {
  const main = await readRepoFile('apps/fufu-tool-site/main.go');
  const runtime = await readRepoFile('apps/fufu-tool-site/runtime.go');
  const runtimeState = await readRepoFile('apps/fufu-tool-site/runtime_state.go');
  const apiRoutes = await readRepoFile('apps/fufu-tool-site/api_routes.go');
  const staticRoutes = await readRepoFile('apps/fufu-tool-site/static.go');
  const appReadme = await readRepoFile('apps/fufu-tool-site/README.md');

  assert.match(main, /func main\(/);
  assert.match(main, /func run\(/);
  assert.doesNotMatch(main, /func initRuntime\(/);
  assert.doesNotMatch(main, /\brootDir\b/);
  assert.doesNotMatch(main, /\bmodelCache\b/);
  assert.doesNotMatch(main, /type httpError struct/);

  assert.match(runtime, /func initRuntime\(/);
  assert.match(runtime, /func shutdownRuntime\(/);
  assert.match(runtime, /func newHTTPServer\(/);
  assert.match(runtimeState, /var modelCache = struct/);
  assert.match(runtimeState, /type apiResult struct/);
  assert.match(runtimeState, /type testRecord struct/);

  assert.match(apiRoutes, /type toolAPIRouteSpec struct/);
  assert.match(apiRoutes, /func findToolAPIPath/);
  assert.doesNotMatch(apiRoutes, /networkAPIRoute|NetworkAPI/);
  assert.match(staticRoutes, /func serveStatusStatic/);
  assert.match(staticRoutes, /func isReferencedToolWebAsset/);
  assert.doesNotMatch(staticRoutes, /serveFrontendStatic|ReferencedNetworkBrowserAsset/);

  assert.match(appReadme, /`main\.go`：进程入口/);
  assert.match(appReadme, /`runtime\.go` \/ `runtime_state\.go`：运行时初始化/);
  assert.match(appReadme, /`model_status_\*`：模型状态/);
});

test('repo ignores generated binaries and excludes non-production buckets from Docker context', async () => {
  const ignored = new Set(nonCommentLines(await readRepoFile('.gitignore')));
  const dockerIgnored = new Set(nonCommentLines(await readRepoFile('.dockerignore')));

  for (const pattern of [
    '.playwright-cli/',
    '/fufu-tool-site',
    '/fufu-tool-site.exe',
    '/fufu-act',
    '/fufu-act.exe',
    '/network-detect',
    '/network-detect.exe',
    'apps/fufu-tool-site/fufu-tool-site',
    'apps/fufu-tool-site/fufu-tool-site.exe',
    'apps/fufu-act/fufu-act',
    'apps/fufu-act/fufu-act.exe',
    'legacy/network-detect/network-detect',
    'legacy/network-detect/network-detect.exe',
    'apps/y2k-nav/y2k-nav',
    'apps/y2k-nav/y2k-nav.exe'
  ]) {
    assert.equal(ignored.has(pattern), true, `.gitignore should ignore ${pattern}`);
  }
  for (const pattern of ['legacy/', 'tools/']) {
    assert.equal(dockerIgnored.has(pattern), true, `.dockerignore should exclude ${pattern}`);
  }
  assert.equal(dockerIgnored.has('tests/'), true, '.dockerignore should exclude tests/');
});

test('root directory stays focused on entry files and shared config only', async () => {
  const entries = await readdir(repoRoot, { withFileTypes: true });
  const rootFiles = entries
    .filter((entry) => entry.isFile())
    .map((entry) => entry.name)
    .sort();
  const rootDirs = entries
    .filter((entry) => entry.isDirectory())
    .map((entry) => entry.name)
    .sort();

  assert.equal(rootFiles.includes('workspace_test.go'), false, 'workspace tests should live under tests/workspace');
  for (const generatedDir of ['.tmp', '.playwright-cli']) {
    assert.equal(rootDirs.includes(generatedDir), false, `root should not contain generated directory ${generatedDir}`);
  }
  for (const generated of [
    '.codex-fufu-tool-site-8081.err.log',
    '.codex-fufu-tool-site-8081.out.log',
    'tmp-fufu-tool-site-8081.err.log',
    'tmp-fufu-tool-site-8081.out.log',
    'cover.out',
    'coverage.out',
    'fufu-tool-site.exe'
  ]) {
    assert.equal(rootFiles.includes(generated), false, `root should not contain generated artifact ${generated}`);
  }

  const workspaceTest = await readRepoFile('tests/workspace/workspace_test.go');
  assert.match(workspaceTest, /package workspace/);
  assert.match(workspaceTest, /func TestWorkspaceModules/);
});

test('docs describe the unified admin and legacy activity-admin redirect', async () => {
  const readme = await readRepoFile('README.md');
  const mergeNotes = await readRepoFile('docs/merge-notes.md');

  assert.match(readme, /\| `\/admin` \/ `\/admin\.html` \| 统一管理后台 \|/);
  assert.match(readme, /\| `\/activity-admin` \/ `\/activity-admin\.html` \| 旧 activity 后台兼容地址，重定向到 `\/admin` \|/);
  assert.doesNotMatch(readme, /\| `\/admin` \/ `\/admin\.html` \| 活动后台 \|/);

  assert.match(mergeNotes, /`apps\/fufu-act`：保留为 activity 后端与 `public\/` 前台静态资源模块，被 `fufu-tool-site` 的 `\/activity` 和 `\/api\/admin\/\*` 等路由嵌入。/);
  assert.match(mergeNotes, /`\/admin`：统一管理后台/);
  assert.match(mergeNotes, /`\/activity-admin`：旧 activity 后台兼容地址，重定向到 `\/admin`。/);
});

test('workspace keeps only active apps and shared packages as modules', async () => {
  const goWork = await readRepoFile('go.work');
  for (const modulePath of [
    './apps/fufu-tool-site',
    './apps/fufu-act',
    './apps/y2k-nav',
    './packages/go/fufu'
  ]) {
    assert.match(goWork, new RegExp(modulePath.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')));
  }
  for (const inactivePath of ['./legacy/network-detect', './tools/mcy-card-upload', './apps/network-detect', './apps/mcy-card-upload']) {
    assert.doesNotMatch(goWork, new RegExp(inactivePath.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')));
  }
});

test('README points project cleanup to the requirements baseline first', async () => {
  const readme = await readRepoFile('README.md');
  const requirements = await readRepoFile('docs/requirements.md');
  const structure = await readRepoFile('docs/project-structure.md');

  assert.match(readme, /docs\/requirements\.md/);
  assert.match(readme, /docs\/project-structure\.md/);
  assert.match(readme, /legacy\/network-detect/);
  assert.match(readme, /tools\/mcy-card-upload/);
  assert.match(requirements, /谁用，怎么用，达成什么效果/);
  assert.match(requirements, /统一的是入口和运营体验，不是抹掉模块边界/);
  assert.match(requirements, /整理顺序/);
  assert.match(structure, /`apps\/`\s*\|\s*当前仍参与统一工具站运行/);
  assert.match(structure, /`legacy\/`\s*\|\s*已退出生产入口/);
  assert.match(structure, /`tools\/`\s*\|\s*本地运营\/维护脚本/);
});

test('navigation configuration is isolated from generic admin config plumbing', async () => {
  const navigationConfig = await readRepoFile('apps/fufu-tool-site/navigation_config.go');
  const navigationRoutes = await readRepoFile('apps/fufu-tool-site/navigation_routes.go');
  const navigationCore = await readRepoFile('packages/go/fufu/navcore/navcore.go');
  const adminConfig = await readRepoFile('apps/fufu-tool-site/admin_config.go');

  assert.match(navigationConfig, /fufu\/navcore/);
  assert.match(navigationConfig, /func normalizeNavigationConfig/);
  assert.match(navigationCore, /type NavigationCardConfig struct/);
  assert.match(navigationCore, /func NormalizeNavigationConfig/);
  assert.match(navigationRoutes, /func navLineCategories/);
  assert.match(navigationRoutes, /func navigationToolsForRuntime/);
  assert.doesNotMatch(adminConfig, /type NavigationCardConfig struct/);
  assert.doesNotMatch(adminConfig, /func normalizeNavigationConfig/);
});

test('core packages avoid adapter-only dependencies', async () => {
  for (const path of [
    'packages/go/fufu/admincore/admincore.go',
    'packages/go/fufu/navcore/navcore.go',
    'packages/go/fufu/modelcore/modelcore.go',
    'packages/go/fufu/salecore/salecore.go',
    'packages/go/fufu/mcycore/mcycore.go',
    'packages/go/fufu/scratchcore/scratchcore.go',
    'packages/go/fufu/probabilitycore/probabilitycore.go',
    'packages/go/fufu/prizepoolcore/prizepoolcore.go',
    'packages/go/fufu/lotterycore/lotterycore.go',
    'packages/go/fufu/spincore/spincore.go'
  ]) {
    const source = await readRepoFile(path);
    assert.doesNotMatch(source, /"fufu\/newapi"/, `${path} should not import NewAPI adapters or DTOs`);
    assert.doesNotMatch(source, /"fufu\/tokens"/, `${path} should not import token-service adapters or DTOs`);
    assert.doesNotMatch(source, /\bnewapi\./, `${path} should not use NewAPI adapter DTOs`);
    assert.doesNotMatch(source, /"net\/http"/, `${path} should not import HTTP adapters`);
    assert.doesNotMatch(source, /"database\/sql"/, `${path} should not import DB adapters`);
    assert.doesNotMatch(source, /"os"/, `${path} should not import filesystem/env adapters`);
    assert.doesNotMatch(source, /config\.Env|os\.|ReadFile|WriteFile|http\.|sql\./, `${path} should stay object-only`);
  }
});

test('unified admin adapters are split by responsibility', async () => {
  const adminConfig = await readRepoFile('apps/fufu-tool-site/admin_config.go');
  const adminStore = await readRepoFile('apps/fufu-tool-site/admin_config_store.go');
  const adminStoreDB = await readRepoFile('apps/fufu-tool-site/admin_config_store_db.go');
  const adminStoreLoad = await readRepoFile('apps/fufu-tool-site/admin_config_store_load.go');
  const adminStoreLegacy = await readRepoFile('apps/fufu-tool-site/admin_config_store_legacy.go');
  const adminStoreNormalize = await readRepoFile('apps/fufu-tool-site/admin_config_store_normalize.go');
  const adminStoreSave = await readRepoFile('apps/fufu-tool-site/admin_config_store_save.go');
  const adminStoreSeed = await readRepoFile('apps/fufu-tool-site/admin_config_store_seed.go');
  const adminRuntime = await readRepoFile('apps/fufu-tool-site/admin_runtime.go');
  const adminRoutes = await readRepoFile('apps/fufu-tool-site/admin_config_routes.go');
  const adminSession = await readRepoFile('apps/fufu-tool-site/admin_session.go');

  assert.match(adminConfig, /type ToolConfig struct/);
  assert.match(adminConfig, /type adminConfigPatch struct/);
  assert.doesNotMatch(adminConfig, /func \(s \*toolConfigStore\) Load/);
  assert.doesNotMatch(adminConfig, /func handleUnifiedAdminSessionAPI/);
  assert.doesNotMatch(adminConfig, /func handleUnifiedAdminConfigAPI/);
  assert.doesNotMatch(adminConfig, /func applyToolConfigSnapshot/);

  assert.match(adminStore, /type toolConfigStore struct/);
  assert.match(adminStore, /func \(s \*toolConfigStore\) Load/);
  assert.doesNotMatch(adminStore, /func normalizeToolConfig/);
  assert.doesNotMatch(adminStore, /func openToolConfigDBStore/);
  assert.doesNotMatch(adminStore, /func defaultToolConfig/);
  assert.doesNotMatch(adminStore, /func handleUnifiedAdmin/);

  assert.match(adminStoreDB, /type toolConfigDBStore struct/);
  assert.match(adminStoreDB, /func openToolConfigDBStore/);
  assert.match(adminStoreDB, /func decodeStoredToolConfig/);
  assert.match(adminStoreLoad, /func loadInitialToolConfig/);
  assert.match(adminStoreLegacy, /type legacyToolConfigFile struct/);
  assert.match(adminStoreNormalize, /func normalizeToolConfig/);
  assert.match(adminStoreSave, /func \(s \*toolConfigStore\) Save/);
  assert.match(adminStoreSeed, /func defaultToolConfig/);

  assert.match(adminRuntime, /func applyToolConfigSnapshot/);
  assert.match(adminRuntime, /func managedSitesForRuntime/);
  assert.match(adminRoutes, /func handleUnifiedAdminConfigAPI/);
  assert.match(adminRoutes, /func adminConfigResponse/);
  assert.match(adminSession, /func handleUnifiedAdminSessionAPI/);
  assert.match(adminSession, /func validUnifiedAdminSession/);
});

test('sale-card admin separates handlers from plan and schedule adapters', async () => {
  const saleCardAdmin = await readRepoFile('apps/fufu-act/card_listing_admin.go');
  const saleCardPlan = await readRepoFile('apps/fufu-act/sale_card_admin_plan.go');
  const saleCardSchedule = await readRepoFile('apps/fufu-act/sale_card_schedule_store.go');

  assert.match(saleCardAdmin, /func handleAdminSaleCardsRun/);
  assert.match(saleCardAdmin, /func handleAdminSaleCardsConfig/);
  assert.match(saleCardAdmin, /func handleAdminSaleCardsStock/);
  assert.doesNotMatch(saleCardAdmin, /func saleCardPlanFromRunRequest/);
  assert.doesNotMatch(saleCardAdmin, /func loadSaleCardSchedule/);
  assert.doesNotMatch(saleCardAdmin, /func saveSaleCardSchedule/);

  assert.match(saleCardPlan, /func saleCardPlanFromRunRequest/);
  assert.match(saleCardPlan, /func saleCardPlanList/);
  assert.doesNotMatch(saleCardPlan, /func handleAdminSaleCards/);

  assert.match(saleCardSchedule, /func loadSaleCardSchedule/);
  assert.match(saleCardSchedule, /func saveSaleCardSchedule/);
  assert.match(saleCardSchedule, /func normalizeSaleCardSchedule/);
  assert.doesNotMatch(saleCardSchedule, /func handleAdminSaleCards/);
});

test('model manual testing separates http, runner, cache projection and overview', async () => {
  const manualHandler = await readRepoFile('apps/fufu-tool-site/model_manual.go');
  const manualRunner = await readRepoFile('apps/fufu-tool-site/model_manual_runner.go');
  const manualCache = await readRepoFile('apps/fufu-tool-site/model_manual_cache.go');
  const overview = await readRepoFile('apps/fufu-tool-site/model_overview.go');

  assert.match(manualHandler, /func handleModelTest/);
  assert.doesNotMatch(manualHandler, /func testModel/);
  assert.doesNotMatch(manualHandler, /func applyManualToCachedStatus/);
  assert.doesNotMatch(manualHandler, /func buildOverview/);

  assert.match(manualRunner, /func testModel/);
  assert.match(manualRunner, /func reserveModelTestCooldown/);
  assert.doesNotMatch(manualRunner, /func handleModelTest/);
  assert.doesNotMatch(manualRunner, /func applyManualToCell/);

  assert.match(manualCache, /func applyManualToCachedStatus/);
  assert.match(manualCache, /func cloneModelStatus/);
  assert.doesNotMatch(manualCache, /func testModel/);
  assert.match(overview, /func buildOverview/);
});

test('model status build separates cache, fetch, site build and projection', async () => {
  const build = await readRepoFile('apps/fufu-tool-site/model_status_build.go');
  const cacheFlow = await readRepoFile('apps/fufu-tool-site/model_status_cache_flow.go');
  const fetch = await readRepoFile('apps/fufu-tool-site/model_status_fetch_orchestration.go');
  const siteBuild = await readRepoFile('apps/fufu-tool-site/model_status_site_build.go');
  const projection = await readRepoFile('apps/fufu-tool-site/model_status_projection.go');

  assert.match(build, /func buildModelStatus/);
  assert.match(build, /func newModelStatusBuildPlan/);
  assert.doesNotMatch(build, /func getModelStatus/);
  assert.doesNotMatch(build, /func fetchModelStatusSiteData/);
  assert.doesNotMatch(build, /func buildPerSiteModelStatus/);
  assert.doesNotMatch(build, /func projectModelStatus/);

  assert.match(cacheFlow, /func getModelStatus/);
  assert.match(fetch, /func fetchModelStatusSiteData/);
  assert.match(fetch, /loadSiteLogs/);
  assert.match(fetch, /loadSiteChannels/);
  assert.match(fetch, /loadPricing/);
  assert.match(siteBuild, /func buildPerSiteModelStatus/);
  assert.match(siteBuild, /func buildCell/);
  assert.match(projection, /func projectModelStatus/);
  assert.match(projection, /func runtimeModelManualProjection/);
  assert.doesNotMatch(projection, /func loadSiteLogs/);
});

test('legacy network-detect keeps historical model status shell split by responsibility', async () => {
  const build = await readRepoFile('legacy/network-detect/model_status_build.go');
  const cacheFlow = await readRepoFile('legacy/network-detect/model_status_cache_flow.go');
  const fetch = await readRepoFile('legacy/network-detect/model_status_fetch_orchestration.go');
  const siteBuild = await readRepoFile('legacy/network-detect/model_status_site_build.go');
  const projection = await readRepoFile('legacy/network-detect/model_status_projection.go');

  assert.match(build, /func buildModelStatus/);
  assert.match(build, /func newModelStatusBuildPlan/);
  assert.doesNotMatch(build, /func getModelStatus/);
  assert.doesNotMatch(build, /func fetchModelStatusSiteData/);
  assert.doesNotMatch(build, /func buildPerSiteModelStatus/);
  assert.doesNotMatch(build, /func projectModelStatus/);

  assert.match(cacheFlow, /func getModelStatus/);
  assert.match(fetch, /func fetchModelStatusSiteData/);
  assert.match(fetch, /loadSiteLogs/);
  assert.match(fetch, /loadSiteChannels/);
  assert.match(fetch, /loadPricing/);
  assert.match(siteBuild, /func buildPerSiteModelStatus/);
  assert.match(siteBuild, /func buildCell/);
  assert.match(projection, /func projectModelStatus/);
  assert.match(projection, /func runtimeModelManualProjection/);
  assert.doesNotMatch(projection, /func loadSiteLogs/);
});

test('login module separates handler, request, token, purchase, store and response', async () => {
  const handler = await readRepoFile('apps/fufu-act/login.go');
  const request = await readRepoFile('apps/fufu-act/login_request.go');
  const token = await readRepoFile('apps/fufu-act/login_token.go');
  const purchase = await readRepoFile('apps/fufu-act/login_purchase.go');
  const plan = await readRepoFile('apps/fufu-act/login_plan.go');
  const store = await readRepoFile('apps/fufu-act/login_store.go');
  const response = await readRepoFile('apps/fufu-act/login_response.go');

  assert.match(handler, /func handleLogin/);
  assert.doesNotMatch(handler, /func lookupLoginToken/);
  assert.doesNotMatch(handler, /func planLoginCardForToken/);
  assert.doesNotMatch(handler, /func insertLoginCard/);
  assert.doesNotMatch(handler, /func buildLoginCardResponse/);

  assert.match(request, /func readLoginIdentity/);
  assert.match(request, /func parseLoginUserID/);
  assert.match(token, /func lookupLoginToken/);
  assert.match(token, /func dollarsTier/);
  assert.match(purchase, /func createLoginCard/);
  assert.match(purchase, /func prepareLoginCardPlan/);
  assert.match(plan, /func planLoginCardForToken/);
  assert.match(store, /func insertLoginCard/);
  assert.match(store, /func lookupCard/);
  assert.match(response, /func respondCard/);
  assert.match(response, /func buildLoginCardResponse/);
});

test('credit module separates enqueue, worker, store, quota adapter and rules', async () => {
  const enqueue = await readRepoFile('apps/fufu-act/credit.go');
  const worker = await readRepoFile('apps/fufu-act/credit_worker.go');
  const store = await readRepoFile('apps/fufu-act/credit_store.go');
  const quota = await readRepoFile('apps/fufu-act/credit_quota.go');
  const rules = await readRepoFile('apps/fufu-act/credit_rules.go');

  assert.match(enqueue, /func enqueueCredit/);
  assert.doesNotMatch(enqueue, /func processCredits/);
  assert.doesNotMatch(enqueue, /func buildCreditFailureUpdate/);
  assert.doesNotMatch(enqueue, /func \(s sqliteCreditProcessorStore\) Pending/);

  assert.match(worker, /func processCredits/);
  assert.match(worker, /func processCreditsWith/);
  assert.match(store, /type sqliteCreditProcessorStore struct/);
  assert.match(store, /func \(s sqliteCreditProcessorStore\) Pending/);
  assert.match(quota, /type newAPICreditQuotaAdapter struct/);
  assert.match(quota, /func \(a newAPICreditQuotaAdapter\) AddQuota/);
  assert.match(rules, /func buildCreditFailureUpdate/);
  assert.match(rules, /func sanitizeCreditFailureError/);
});

test('scratch module separates handlers, db store and pure rules', async () => {
  const scratchHandlers = await readRepoFile('apps/fufu-act/scratch.go');
  const scratchStore = await readRepoFile('apps/fufu-act/scratch_store.go');
  const scratchRules = await readRepoFile('apps/fufu-act/scratch_rules.go');
  const scratchCore = await readRepoFile('packages/go/fufu/scratchcore/scratchcore.go');

  assert.match(scratchHandlers, /func handleScratchStart/);
  assert.match(scratchHandlers, /func handleScratchReveal/);
  assert.match(scratchHandlers, /func handleScratchCashout/);
  assert.match(scratchHandlers, /func handleScratchReset/);
  assert.doesNotMatch(scratchHandlers, /func lookupScratch/);
  assert.doesNotMatch(scratchHandlers, /func parseScratchIntArray/);
  assert.doesNotMatch(scratchHandlers, /func validScratchCells/);

  assert.match(scratchStore, /func lookupScratch/);
  assert.match(scratchStore, /func insertScratchGame/);
  assert.match(scratchStore, /func updateScratchLost/);
  assert.doesNotMatch(scratchStore, /func handleScratch/);

  assert.match(scratchRules, /fufu\/scratchcore/);
  assert.match(scratchRules, /func parseScratchIntArray/);
  assert.match(scratchRules, /func scratchPrizeForSafeCount/);
  assert.doesNotMatch(scratchRules, /db\./);

  assert.match(scratchCore, /package scratchcore/);
  assert.match(scratchCore, /func ParseIntArray/);
  assert.match(scratchCore, /func ValidCells/);
  assert.match(scratchCore, /func GameResponse/);
});

test('mcy shop adapter separates high-level use cases, inventory, session, login and http', async () => {
  const shop = await readRepoFile('apps/fufu-act/shop.go');
  const inventory = await readRepoFile('apps/fufu-act/shop_inventory.go');
  const session = await readRepoFile('apps/fufu-act/shop_session.go');
  const login = await readRepoFile('apps/fufu-act/shop_login.go');
  const http = await readRepoFile('apps/fufu-act/shop_http.go');

  assert.match(shop, /func findShopPurchase/);
  assert.match(shop, /type ShopPurchaseLookup struct/);
  assert.doesNotMatch(shop, /func queryMCYUsableStock/);
  assert.doesNotMatch(shop, /func mcyLogin/);
  assert.doesNotMatch(shop, /func mcyPost/);

  assert.match(inventory, /func queryMCYUsableStock/);
  assert.match(inventory, /func mcyCardGet/);
  assert.match(inventory, /func mcyCardGetTotal/);
  assert.doesNotMatch(inventory, /func mcyLogin/);

  assert.match(session, /func mcyConfig/);
  assert.match(session, /func ensureMCYCookie/);
  assert.match(session, /func refreshMCYCookie/);
  assert.doesNotMatch(session, /func mcyLoginJSON/);

  assert.match(login, /func mcyLogin/);
  assert.match(login, /func mcyLoginJSON/);
  assert.match(login, /func encryptedMCYLoginEndpoints/);
  assert.match(http, /func mcyPost/);
  assert.match(http, /func decodeMCYResponse/);
  assert.match(http, /func isMCYAuthError/);
});

test('spin module keeps draw rules in spincore and app handler thin', async () => {
  const spinHandler = await readRepoFile('apps/fufu-act/spin.go');
  const spinRunner = await readRepoFile('apps/fufu-act/spin_runner.go');
  const spinStore = await readRepoFile('apps/fufu-act/spin_store.go');
  const spinRules = await readRepoFile('apps/fufu-act/spin_rules.go');
  const spinCore = await readRepoFile('packages/go/fufu/spincore/spincore.go');
  const activityConfig = await readRepoFile('packages/go/fufu/activity/activity.go');

  assert.match(spinHandler, /func handleSpin/);
  assert.doesNotMatch(spinHandler, /func spin\(/);
  assert.doesNotMatch(spinHandler, /db\./);
  assert.doesNotMatch(spinHandler, /func secureRandomInt/);

  assert.match(spinRunner, /func runSpinForCard/);
  assert.match(spinRunner, /func spinForceForCard/);
  assert.doesNotMatch(spinRunner, /func handleSpin/);

  assert.match(spinStore, /func maxSpinPrize/);
  assert.match(spinStore, /func recordSpinRetry/);
  assert.match(spinStore, /func recordSpinResult/);

  assert.match(spinRules, /fufu\/activity/);
  assert.match(spinRules, /func spin\(/);
  assert.match(spinRules, /secureRandomInt = func/);
  assert.match(spinCore, /package spincore/);
  assert.match(spinCore, /func Spin/);
  assert.match(spinCore, /func Roll/);
  assert.match(activityConfig, /fufu\/spincore/);
});

test('spin core uses one unified prize pool across dollar tiers', async () => {
  const spinCore = await readRepoFile('packages/go/fufu/spincore/spincore.go');
  const lotteryCore = await readRepoFile('packages/go/fufu/lotterycore/lotterycore.go');
  const probabilityCore = await readRepoFile('packages/go/fufu/probabilitycore/probabilitycore.go');
  const prizePoolCore = await readRepoFile('packages/go/fufu/prizepoolcore/prizepoolcore.go');
  const spinRules = await readRepoFile('apps/fufu-act/spin_rules.go');
  const activityPanels = await readRepoFile('apps/fufu-tool-site/ui/src/admin/activityPanels.tsx');

  assert.match(lotteryCore, /func BalancePool/);
  assert.match(lotteryCore, /func BalancePoolForPlan/);
  assert.match(lotteryCore, /func ExpectedValue/);
  assert.match(probabilityCore, /func BalanceWeights/);
  assert.match(probabilityCore, /func TargetPerDrawExpectedValue/);
  assert.doesNotMatch(probabilityCore, /type Prize|Rank|Label|Advertised/, 'probability core must not know prize-pool display concepts');
  assert.match(prizePoolCore, /fufu\/probabilitycore/);
  assert.match(prizePoolCore, /type Prize struct/);
  assert.match(spinCore, /fufu\/prizepoolcore/);
  assert.doesNotMatch(spinCore, /fufu\/lotterycore/);
  assert.match(spinCore, /BalancePoolForPlan/);
  assert.doesNotMatch(spinCore, /TierPools/, 'spincore must not branch prize pools by dollar tier');
  assert.doesNotMatch(spinCore, /tierPools/i, 'spincore must keep only one active prize pool');
  assert.doesNotMatch(spinCore, /PostJackpot/, 'spincore must not switch into a separate post-jackpot probability pool');
  assert.doesNotMatch(spinCore, /postJackpot/i, 'spincore must not keep a second jackpot probability pool');
  assert.doesNotMatch(spinRules, /TierPools:/, 'activity app must not pass legacy tier pools into spin core');
  assert.doesNotMatch(spinRules, /PostJackpot/, 'activity app must not pass legacy post-jackpot pools into spin core');
  assert.doesNotMatch(activityPanels, /分额度奖池/, 'admin UI must not expose per-tier prize pools');
  assert.doesNotMatch(activityPanels, /tierEntries/, 'admin UI should edit one prize pool only');
  assert.doesNotMatch(activityPanels, /<PrizePoolEditor rows=\{prizePool\}/);
  assert.doesNotMatch(activityPanels, /postJackpot/i);
});
