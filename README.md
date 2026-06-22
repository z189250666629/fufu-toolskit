# fufu-toolskit

`fufu-toolskit` 是 FuFu 工具集合 monorepo。当前生产入口已合并为一个 Go 服务：`fufu-tool-site`。

| 入口 | 目录 | 本地默认端口 | 生产对外端口 | 说明 |
| --- | --- | ---: | ---: | --- |
| fufu 工具站 | `apps/fufu-tool-site` | `8080` | `38473` | 首页导航 + API/模型状态 + 合卡 + 活动前台 + 统一管理后台 |

旧的独立生产入口 `y2k-nav:33148`、`fufu-act:18820` 已下线；`apps/y2k-nav` 与 `apps/fufu-act` 仍保留为嵌入式资源模块。`fufu-combine` 已并入合卡模块，入口为 `/combine`。

## 路由

| 路径 | 功能 |
| --- | --- |
| `/` | fufu 工具站导航页 |
| `/status` | API / NewAPI 模型状态面板 |
| `/combine` | 合卡工具 |
| `/activity` | 活动前台 |
| `/admin` / `/admin.html` | 统一管理后台 |
| `/activity-admin` / `/activity-admin.html` | 旧 activity 后台兼容地址，重定向到 `/admin` |
| `/api/health` | 健康检查 |

## 需求整理

项目整理先从需求和验收标准开始，详见 `docs/requirements.md`。

## 目录结构

```text
apps/
  fufu-tool-site/     # 统一生产服务：导航 + status + combine + activity
    frontend/         # API/模型状态静态前端
    combine/          # 合卡静态前端
  y2k-nav/            # 导航页静态资源模块，被 fufu-tool-site 嵌入
  fufu-act/           # activity 后端与 public 静态资源模块，被 fufu-tool-site 嵌入
  network-detect/     # 历史模块源码保留，生产不再单独部署
packages/go/fufu/     # 共享 Go 包：config/newapi/tokens/combine/activity/auth
scripts/
  start-config.mjs    # 唯一启动目标配置
  start-all.mjs       # 读取集中配置，只启动 fufu-tool-site
  test-config.mjs     # 唯一测试套件配置
  test-suite.mjs      # 唯一测试入口文件
```

## 安装/检查

要求：

- Go 1.25+
- Node.js 20+

```powershell
npm run deps
npm test
```

整个项目只保留根目录 `npm test` 一个测试入口；它只启动 `node --test scripts/test-suite.mjs` 这一条根级测试进程。子 package 不再提供 `test` / `test:*` 脚本，root 也不再分开调用各 app 测试或 Go 测试二进制，避免 Windows 反复弹出临时 `*.test.exe` 确认。

## 本地启动

```powershell
npm start
```

默认访问：`http://127.0.0.1:8080/`。

## 配置

统一服务复用已有变量，不新增重复配置。

> **配置持久化**：管理后台保存的 NewAPI 站点与活动配置写入 SQLite 数据库
> `apps/fufu-tool-site/data/tool-config.db`（随 `./data` 卷持久化）。下面列出的
> `NEWAPI_*` / `FUFU_*` / `MCY_*` 变量**仅作为首次启动的初始种子**——数据库一旦写入即成为
> 唯一数据源，之后重新部署代码无需再次修改这些环境变量，直接在后台修改并保存即可。
> 例外：`ADMIN_TOKEN` 始终用于后台登录鉴权，必须保留在环境变量中。旧版
> `data/tool-config.json` 会在首次启动时自动迁移进数据库，并备份为 `tool-config.json.migrated`。
> 首页工具入口（Web Terminal、Build、状态、合卡、活动等）同样由后台保存到
> `tool-config.db`，首页通过 `/api/nav/tools` 读取；接口不可用时前端保留静态 fallback。

### NewAPI / 状态面板 / 合卡

```powershell
$env:NEWAPI_API_SITE_URL = 'https://api.fufuflower.top'
$env:NEWAPI_API_SITE_TOKEN = '<api-site-admin-token>'
$env:NEWAPI_TOKEN_SITE_URL = 'https://token.fufuflower.top'
$env:NEWAPI_TOKEN_SITE_TOKEN = '<token-site-admin-token>'
$env:PORT = '8080'
```

也支持 `NEWAPI_MANAGED_API_SITES` / `NEWAPI_MANAGED_API_CONFIG`。合卡和状态面板复用同一套 NewAPI 配置。
`CONNECTIVITY_API_URLS`、`CONNECTIVITY_TOKEN_URLS` 仅是可选多地址检测覆盖；留空时分别复用非内网的 `NEWAPI_API_SITE_URL`、`NEWAPI_TOKEN_SITE_URL`，不要为了单站点重复新增变量。

### 活动模块 / 后台

```powershell
$env:FUFU_API_BASE_URL = 'https://api.fufuflower.top'
$env:FUFU_API_TOKEN = '<newapi-admin-token>'
$env:FUFU_API_USER_ID = '1'
$env:FUFU_QUOTA_UNIT = '500000'
$env:ADMIN_TOKEN = '<admin-token>'
```

可选 MCY 商城变量：

```powershell
$env:MCY_BASE_URL = 'https://shop.example.com'
$env:MCY_COOKIE = '<session-cookie>'
$env:MCY_USERNAME = '<username>'
$env:MCY_PASSWORD = '<password>'
$env:MCY_LOGIN_ENDPOINT = '/admin/login'
$env:MCY_UPLOAD_ENDPOINT = '/plugin/virtual-card-ship/card/add'
```

`MCY_COOKIE` 是可选会话覆盖；已配置 `MCY_BASE_URL`、`MCY_USERNAME`、`MCY_PASSWORD` 时，服务会登录商城并复用返回的 cookie。`ADMIN_TOKEN` 未设置时后台接口拒绝访问。

## 不应提交的内容

- `.env*`
- `config.json`
- `apps/**/data/*.db*`
- `node_modules/`
- 构建产物和日志

## CI/CD

生产部署按 tag + directive 触发，详见 `docs/CI_CD.md`。当前只部署 `fufu-tool-site`，外部端口继续使用 `38473`。
