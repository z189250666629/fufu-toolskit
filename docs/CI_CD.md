# CI/CD Runbook

生产部署沿用 tag-gated 模式：只有 `v*` tag 且 commit message 包含 deploy directive 时才部署。pre-release tag（包含 `-`）会跳过。

## 部署应用

| Workflow | 目录 | 镜像 | 默认远端路径 | 对外端口 | Directive |
| --- | --- | --- | --- | ---: | --- |
| `deploy-y2k-nav` | `apps/y2k-nav` | `ghcr.io/<owner>/<repo>-y2k-nav` | `/data/docker/y2k-nav` | `33148` | `[deploy y2k]` / `[deploy y2k-nav]` / `[deploy nav]` |
| `deploy-network` | `apps/network-detect` | `ghcr.io/<owner>/<repo>-network-detect` | `/data/docker/network-detect` | `38473` | `[deploy network]` / `[deploy network-detect]` / `[deploy combine]` |
| `deploy-act` | `apps/fufu-act` | `ghcr.io/<owner>/<repo>-fufu-act` | `/data/docker/fufu-act` | `18820` | `[deploy act]` / `[deploy activity]` / `[deploy fufu-act]` |

`[deploy all]` 会同时触发三个部署。`fufu-combine` 已并入 `network-detect`，不再独立部署；合卡页面由 network 服务的 `/combine` 承载。

## 阶段

1. `check`：过滤 tag 与 directive。
2. `verify`：运行 Go 测试。
3. `docker`：构建并推送 GHCR 镜像。
4. `deploy`：SSH 上传 compose/env，执行 `docker compose pull && docker compose up -d`。

## GitHub Variables

| Name | 说明 |
| --- | --- |
| `SSH_HOST` / `DEPLOY_HOST` | VPS IP/域名 |
| `SSH_PORT` / `DEPLOY_PORT` | SSH 端口，默认 `22` |
| `SSH_USER` / `DEPLOY_USER` | SSH 用户 |
| `HOST_BIND` | 默认 `0.0.0.0` |
| `DEPLOY_PATH` | 统一根路径；为空时各服务使用默认路径 |
| `Y2K_NAV_PORT` | 默认 `33148` |
| `NETWORK_DETECT_HOST_PORT` / `DETECT_PORT` | 默认 `38473` |
| `FUFU_ACT_HOST_PORT` / `ACT_PORT` | 默认 `18820` |

## 业务 Variables / Secrets

| Name | 用途 |
| --- | --- |
| `NEWAPI_API_SITE_URL` | network/合卡/fufu-act 可共用的 API 次数站 URL |
| `NEWAPI_API_SITE_TOKEN` | API 次数站 token |
| `NEWAPI_TOKEN_SITE_URL` | Token 站 URL |
| `NEWAPI_TOKEN_SITE_TOKEN` | Token 站 token |
| `NEWAPI_MANAGED_API_SITES` | network 完整站点配置，可选 |
| `CONNECTIVITY_API_URLS` | network URL 检测列表 |
| `CONNECTIVITY_TOKEN_URLS` | network URL 检测列表 |
| `FUFU_API_BASE_URL` | fufu-act 可覆盖 NewAPI URL |
| `FUFU_API_TOKEN` | fufu-act 可覆盖 NewAPI token |
| `FUFU_API_USER_ID` | fufu-act NewAPI user id，默认 `1` |
| `FUFU_QUOTA_UNIT` | 默认 `500000` |
| `MCY_BASE_URL` / `MCY_COOKIE` / `MCY_USERNAME` / `MCY_PASSWORD` | fufu-act 商城校验，可选 |
| `ADMIN_TOKEN` | fufu-act 后台 token；未设置时兼容旧默认值 |

## 本地检查

```powershell
npm test

docker compose -f infra/deploy/y2k-nav/docker-compose.yml config --quiet
docker compose -f infra/deploy/network-detect/docker-compose.yml config --quiet
docker compose -f infra/deploy/fufu-act/docker-compose.yml config --quiet
```
