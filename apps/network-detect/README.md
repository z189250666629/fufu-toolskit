# API Site Usage Dashboard

Frontend/backend separated dashboard for fixed NewAPI-managed API sites. The backend reads configured NewAPI admin sources, fetches usage logs and channel metadata, and returns sanitized aggregate data to the browser.

## Files

- `backend/server.js`: Node.js backend and static frontend server.
- `frontend/index.html`: frontend entry.
- `frontend/app.js`: dashboard UI logic.
- `frontend/styles.css`: dashboard styles.
- `newapi-managed-api-sites.json`: committed deployment config with real URLs and token environment references.
- `newapi-managed-api-sites.example.json`: sample managed API site config without secrets.
- `Dockerfile`: production Node image.
- `docker-compose.yml`: production deployment manifest used by CI/CD.
- `.gitlab-ci.yml`: GitLab pipeline for verify, image build, and production deploy.
- `.github/workflows/docker-release.yml`: GitHub Actions pipeline for verify and image build.
- `docs/gitlab-cicd.md`: GitLab CI/CD setup and release notes.
- `docs/github-cicd.md`: GitHub Actions setup and release notes.

Legacy PHP files are kept only for reference and are no longer used by the Node dashboard.

## Local Development

```powershell
npm start
```

Then open `http://127.0.0.1:8080/`.

Syntax check:

```powershell
npm test
```

## NewAPI Config

Configure fixed managed API sites in this priority order:

1. `NEWAPI_MANAGED_API_SITES`: full JSON config, useful when the deployment platform supports multiline secrets.
2. Deployment-friendly per-site env vars, useful for GitHub/GitLab release deploys.
3. `NEWAPI_MANAGED_API_CONFIG`: path to a JSON config file.
4. `newapi-managed-api-sites.json` in the project root. The committed file keeps the real site URLs and reads tokens from environment variables.

Example:

```powershell
$env:NEWAPI_API_SITE_URL = 'https://api.fufuflower.top'
$env:NEWAPI_API_SITE_TOKEN = '<admin-token>'
$env:NEWAPI_TOKEN_SITE_URL = 'https://token.fufuflower.top'
$env:NEWAPI_TOKEN_SITE_TOKEN = '<admin-token>'
npm start
```

The accepted config format is compatible with `newapi-manager` style arrays: `managedApiSites`, `sources`, `instances`, and `admin_instances`.

Each source supports:

- `name`: display name.
- `url`: NewAPI base URL.
- `token`: admin or user access token.
- `tokenEnv`: environment variable name that contains the token, preferred for deployment.
- `userId`: NewAPI user id, default `1`.
- `channelListEndpoint`: optional channel endpoint override, for example `/api/channel/search?keyword=&p=1&page_size=500`.
- `quotaUnit`, `currency`, `rechargeRatio`: used to convert quota to display value.
- `skipUserHeader`: set true for instances that reject `New-Api-User`.

Set `NEWAPI_DASHBOARD_VIEW_KEY` to require a browser view key before logs are shown.

### Deployment Env Vars

For tagged deployments, set these variables in the deployment environment instead of editing files in the container:

| Variable | Purpose |
| --- | --- |
| `NEWAPI_API_SITE_URL` | API 次数站 NewAPI base URL. |
| `NEWAPI_API_SITE_TOKEN` | API 次数站 admin/user access token. |
| `NEWAPI_TOKEN_SITE_URL` | Token 站 NewAPI base URL. |
| `NEWAPI_TOKEN_SITE_TOKEN` | Token 站 admin/user access token. |
| `NEWAPI_API_SITE_RECHARGE_RATIO` | Optional, defaults to `0.1`. |
| `NEWAPI_TOKEN_SITE_RECHARGE_RATIO` | Optional, defaults to `1`. |

The committed `newapi-managed-api-sites.json` already points to the real fufu URLs and references `NEWAPI_API_SITE_TOKEN` / `NEWAPI_TOKEN_SITE_TOKEN`, so those two token variables are enough for the checked-in config to become active after deployment. If these token variables are absent in local development, the backend skips that template and falls back to the local Downloads config when present.

For this repository's GitHub Actions deploy job, store `NEWAPI_API_SITE_TOKEN` and `NEWAPI_TOKEN_SITE_TOKEN` as GitHub Secrets in the `docker` environment, or as repository secrets if you do not use environment-scoped secrets. The deploy script writes them into the remote compose env file before starting the container.

If you bypass GitHub Actions and run `docker compose` yourself, set the same variables in the server-side compose `.env` file.

For more than two managed sites, use numbered variables:

```text
NEWAPI_MANAGED_SITE_1_NAME=站点名称
NEWAPI_MANAGED_SITE_1_URL=https://example.com
NEWAPI_MANAGED_SITE_1_TOKEN=<admin-token>
NEWAPI_MANAGED_SITE_1_RECHARGE_RATIO=1
```

The numbered form supports indexes `1` through `10` and the same optional suffixes: `_USER_ID`, `_KIND`, `_CHANNEL_LIST_ENDPOINT`, `_QUOTA_UNIT`, `_CURRENCY`, `_SKIP_USER_HEADER`, and `_NOTE`.

### URL Detection Targets

The URL detection panel can also be driven by deployment env vars:

```text
# Optional: defaults to NEWAPI_API_SITE_URL when unset
CONNECTIVITY_API_URLS=https://api.fufuapi.top,https://api.fufuflower.top
# Optional: defaults to NEWAPI_TOKEN_SITE_URL when unset
CONNECTIVITY_TOKEN_URLS=https://token.fufuapi.top,https://token.fufuflower.top
```

For a full custom target list, set `CONNECTIVITY_TARGETS` to JSON:

```json
[
  { "id": "api", "name": "API 次数站", "urls": ["https://api.example.com"] },
  { "id": "token", "name": "Token 站", "urls": ["https://token.example.com"] }
]
```

## Backend API

- `GET /api/health`: health check.
- `GET /api/client`: client IP and server time.
- `GET /api/connectivity/targets`: fixed fufu target URLs.
- `GET /api/newapi/sites`: sanitized managed site list.
- `GET /api/newapi/overview`: usage logs, stats, and model availability matrix.

`/api/newapi/overview` tries `/api/log/self` and `/api/log/self/stat` first, then falls back to admin `/api/log/` and `/api/log/stat`. Model availability is aggregated from the channel list by model, returning only counts, groups, latency, and names, never channel keys or upstream tokens.

## Docker

Build and run locally:

```powershell
docker build -t network-detact:local .
docker run --rm -p 8080:8080 network-detact:local
```

Health check:

```powershell
curl http://127.0.0.1:8080/api/health
```
