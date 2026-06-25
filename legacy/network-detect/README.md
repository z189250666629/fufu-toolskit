# network-detect（legacy）

历史 API / NewAPI 状态面板源码，保留为迁移参考和回归样本。生产入口已经收束到 `apps/fufu-tool-site`，本目录不进入 `go.work` 活跃模块，也不进入生产 Docker context；不要再把它作为独立生产服务部署。

## Local Development

```powershell
npm start
```

Then open:

- Dashboard: `http://127.0.0.1:8080/`
- Combine: `http://127.0.0.1:8080/combine`

Checks:

```powershell
go test -count=1 .
```

## NewAPI Config

Configure fixed managed API sites in this priority order:

1. `NEWAPI_MANAGED_API_SITES`
2. Deployment-friendly env vars such as `NEWAPI_API_SITE_URL` + `NEWAPI_API_SITE_TOKEN`
3. `NEWAPI_MANAGED_API_CONFIG`
4. `newapi-managed-api-sites.json`

The checked-in config references `NEWAPI_API_SITE_TOKEN` and `NEWAPI_TOKEN_SITE_TOKEN` and does not contain secrets.

## Backend API

- `GET /api/health`
- `GET /api/client`
- `GET /api/connectivity/targets`
- `GET /api/newapi/sites`
- `GET /api/newapi/overview`
- `GET /api/newapi/model-status`
- `POST /api/newapi/model-status/test`

Migrated combine-compatible APIs are also served by this app:

- `POST /api/auth`
- `GET /api/session`
- `POST /api/search-keys`
- `POST /api/public-merge`
- `POST /api/merge`
- `GET /api/merge-status/:jobId`
- `POST /api/generate`
- `DELETE /api/token/:id`

## Docker

本目录的 Dockerfile / compose 只作为历史部署参考保留。根目录 `.dockerignore` 会排除 `legacy/`，生产镜像只构建 `apps/fufu-tool-site`。
