# fufu-toolskit

`fufu-toolskit` 是 FuFu 工具集合 monorepo。本阶段后端已 Go 化，前端暂时复用现有静态 HTML/CSS/JS。

| 子项目 | 目录 | 对外端口 | 说明 |
| --- | --- | ---: | --- |
| y2k-nav | `apps/y2k-nav` | `33148` | Go 静态服务，导航页 |
| network-detect | `apps/network-detect` | `38473` | Go 后端，网络检测 + NewAPI 模型状态 + 合卡工具 |
| fufu-act / activity | `apps/fufu-act` | `18820` | Go 后端，活动抽奖/刮刮卡服务 |

`fufu-combine` 已并入 `network-detect`，合卡入口为 `http://127.0.0.1:8080/combine`（部署对外端口 `38473`）。旧独立 combine 部署链路已移除。

## 目录结构

```text
apps/
  y2k-nav/            # 导航页 Go 静态服务
  network-detect/     # 网络检测 + 合卡统一后台
    frontend/         # 原 network 静态前端
    combine/          # 复用原 combine 静态前端
  fufu-act/           # 活动服务 Go 后端 + 原静态前端
packages/go/fufu/     # 共享 Go 包：config/newapi/tokens/combine/activity/auth
scripts/
  start-all.mjs
```

## 安装/检查

要求：

- Go 1.25+
- Node.js 20+（仅用于根目录 npm 脚本和 `start-all.mjs`）

```powershell
npm run deps
npm test
go test -count=1 ./...
```

## 本地启动

```powershell
npm run start:network
npm run start:act
npm run start:y2k
npm run start:all
```

本地默认访问：

- network-detect: `http://127.0.0.1:8080/`
- 合卡工具: `http://127.0.0.1:8080/combine`
- fufu-act: `http://127.0.0.1:18820/`
- y2k-nav: `http://127.0.0.1:33148/`

## 配置

### network-detect + 合卡

常用变量：

```powershell
$env:NEWAPI_API_SITE_URL = 'https://api.fufuflower.top'
$env:NEWAPI_API_SITE_TOKEN = '<api-site-admin-token>'
$env:NEWAPI_TOKEN_SITE_URL = 'https://token.fufuflower.top'
$env:NEWAPI_TOKEN_SITE_TOKEN = '<token-site-admin-token>'
$env:PORT = '8080'
```

也支持 `NEWAPI_MANAGED_API_SITES` / `NEWAPI_MANAGED_API_CONFIG`。合卡功能复用同一套 NewAPI 配置。
`CONNECTIVITY_API_URLS`、`CONNECTIVITY_TOKEN_URLS` 仅是可选多地址检测覆盖；留空时分别复用 `NEWAPI_API_SITE_URL`、`NEWAPI_TOKEN_SITE_URL`，不要为了单站点重复新增变量。

### fufu-act / activity

```powershell
$env:FUFU_API_BASE_URL = 'https://api.fufuflower.top'
$env:FUFU_API_TOKEN = '<newapi-admin-token>'
$env:FUFU_API_USER_ID = '1'
$env:SLOT_PORT = '18820'
$env:ADMIN_TOKEN = '<activity-admin-token>'
```

可选 MCY 商城变量：

```powershell
$env:MCY_BASE_URL = 'https://shop.example.com'
$env:MCY_COOKIE = '<session-cookie>'
$env:MCY_USERNAME = '<username>'
$env:MCY_PASSWORD = '<password>'
```

`MCY_COOKIE` 是可选会话覆盖；已配置 `MCY_BASE_URL`、`MCY_USERNAME`、`MCY_PASSWORD` 时，服务会登录商城并复用返回的 cookie。

## 不应提交的内容

- `.env*`
- `config.json`
- `apps/**/data/*.db*`
- `node_modules/`
- 构建产物和日志

## CI/CD

生产部署按 tag + directive 触发，详见 `docs/CI_CD.md`。
