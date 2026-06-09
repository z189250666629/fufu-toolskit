# API Site Usage Dashboard

Go backend + existing static frontend dashboard for fixed NewAPI-managed API sites. The app also serves the migrated fufu-combine tool at `/combine` and reuses the same NewAPI token/config layer.

## Local Development

```powershell
npm start
```

Then open:

- Dashboard: `http://127.0.0.1:8080/`
- Combine: `http://127.0.0.1:8080/combine`

Checks:

```powershell
npm test
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

```powershell
docker build -f apps/network-detect/Dockerfile -t network-detect:local .
docker run --rm -p 8080:8080 network-detect:local
```
