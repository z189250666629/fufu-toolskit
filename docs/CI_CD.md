# CI/CD Runbook

生产部署沿用 tag-gated 模式：只有 `v*` tag 且 commit message 包含 deploy directive 时才部署。pre-release tag（包含 `-`）会跳过。

GitHub Actions 部署环境固定使用 `docker`（持有 n5105 主机的 SSH / registry / ADMIN_TOKEN 等 secret）。旧的 `toolskit` environment 已退休，不要再切回。

## 部署应用

| Workflow | 目录 | 镜像 | 远端路径 | 对外端口 | Directive |
| --- | --- | --- | --- | ---: | --- |
| `deploy-fufu-tool-site` | `apps/fufu-tool-site` | `ghcr.io/<owner>/<repo>-fufu-tool-site` | `DEPLOY_PATH` 直接作为完整路径（docker env 为 `/data/docker/toolkit`）；默认 `/data/docker/fufu-tool-site` | `38473` | `[deploy fufu-tool-site]` / `[deploy tool-site]` / `[deploy tools]` / `[deploy all]` |

旧 directive 会作为兼容别名触发统一工具站，而不再部署独立服务：`[deploy network]`、`[deploy network-detect]`、`[deploy network_detect]`、`[deploy combine]`、`[deploy act]`、`[deploy activity]`、`[deploy fufu-act]`、`[deploy y2k]`、`[deploy y2k-nav]`、`[deploy nav]`、`[deploy navigation]`。

旧独立生产入口 `33148/y2k-nav`、`18820/fufu-act` 已退休；主服务继续使用外部端口 `38473`。

## 阶段

1. `check`：过滤 tag 与 directive。
2. `verify`：安装统一 app 前端依赖，运行根目录 `npm test` 守护测试；需要额外检查 Go workspace 时运行 `go test -count=1 ./tests/workspace`。
3. `docker`：构建并推送 GHCR 镜像。
4. `deploy`：SSH 上传 compose/env，执行 `docker compose pull && docker compose up -d --remove-orphans`。

## GitHub Variables

| Name | 说明 |
| --- | --- |
| `SSH_HOST` / `DEPLOY_HOST` | VPS IP/域名 |
| `SSH_PORT` / `DEPLOY_PORT` | SSH 端口，默认 `22` |
| `SSH_USER` / `DEPLOY_USER` | SSH 用户 |
| `HOST_BIND` | 默认 `0.0.0.0` |
| `DEPLOY_PATH` | 统一根路径；为空时使用默认路径 |
| `NETWORK_DETECT_HOST_PORT` / `DETECT_PORT` | 统一工具站继续复用原 network 对外端口变量，默认 `38473` |
| `NETWORK_DETECT_DEPLOY_PATH` / `DETECT_DEPLOY_PATH` | 可选：复用原 network 部署路径变量 |

## 业务 Variables / Secrets

| Name | 用途 |
| --- | --- |
| `NEWAPI_API_SITE_URL` | 状态面板/合卡/activity 可共用的 API 次数站 URL |
| `NEWAPI_API_SITE_TOKEN` | API 次数站 token |
| `NEWAPI_TOKEN_SITE_URL` | Token 站 URL |
| `NEWAPI_TOKEN_SITE_TOKEN` | Token 站 token |
| `NEWAPI_MANAGED_API_SITES` | 完整站点配置，可选 |
| `NEWAPI_MANAGED_API_CONFIG` | 完整站点配置，可选；适合放非密钥 JSON 配置 |
| `CONNECTIVITY_API_URLS` | URL 检测列表，可选多地址覆盖；留空时复用非内网 `NEWAPI_API_SITE_URL` |
| `CONNECTIVITY_TOKEN_URLS` | URL 检测列表，可选多地址覆盖；留空时复用非内网 `NEWAPI_TOKEN_SITE_URL` |
| `FUFU_API_BASE_URL` | activity 可覆盖 NewAPI URL |
| `FUFU_API_TOKEN` | activity 可覆盖 NewAPI token |
| `FUFU_API_USER_ID` | activity NewAPI user id，默认 `1` |
| `FUFU_QUOTA_UNIT` | 默认 `500000` |
| `MCY_BASE_URL` / `MCY_COOKIE` / `MCY_USERNAME` / `MCY_PASSWORD` | activity 商城校验，可选 |
| `MCY_LOGIN_ENDPOINT` / `MCY_UPLOAD_ENDPOINT` | activity 商城登录和上架接口路径 |
| `ADMIN_TOKEN` | activity 后台 token；未设置时后台接口拒绝访问 |

其中 token、password、cookie、`ADMIN_TOKEN` 这类敏感值优先放 GitHub Secrets；URL、端口、路径等非敏感值放 Variables。`CONNECTIVITY_*` 不是必填重复变量，只有需要同一站点多地址探测，或确实要让浏览器检测内网/本机地址时才配置。

## 本地检查

```powershell
npm test
go test -count=1 ./tests/workspace

docker compose -f infra/deploy/fufu-tool-site/docker-compose.yml config --quiet
```
