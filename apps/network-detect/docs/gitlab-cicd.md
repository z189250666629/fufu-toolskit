# GitLab CI/CD Setup

This repository includes a GitLab CI/CD pipeline in [`.gitlab-ci.yml`](../.gitlab-ci.yml), a production image definition in [`Dockerfile`](../Dockerfile), a Node backend, a separated frontend, and a deployment manifest in [`docker-compose.yml`](../docker-compose.yml).

The app is served by Node. The backend reads fixed NewAPI-managed API site config, fetches usage logs and channel metadata, and returns sanitized dashboard data to the frontend.

## What The Pipeline Does

1. `verify`
   Runs lightweight checks: required frontend/backend files must exist and `npm test` must pass.
2. `build_image`
   Runs only on Git tags. It builds the production image from [`Dockerfile`](../Dockerfile) and pushes it to the GitLab Container Registry.
3. `deploy_production`
   Runs only on Git tags after `build_image`. It uploads [`docker-compose.yml`](../docker-compose.yml) and a generated `.env` file to the target host, logs in to the registry on that host, pulls the new image, restarts the stack, then waits for the container health check to become `healthy`.

The current release policy is:

1. Merge requests and branch pushes run `verify`.
2. The default branch also runs only `verify`.
3. Git tags run `verify`, `build_image`, and `deploy_production`.

## Runtime Layout

The Docker image uses `node:22-alpine` by default.

Included files:

- `/app/backend/server.js`
- `/app/frontend/index.html`
- `/app/frontend/app.js`
- `/app/frontend/styles.css`
- `/app/newapi-managed-api-sites.example.json`

Runtime endpoints:

- `/` serves the separated frontend.
- `/api/health` returns health status for Docker checks.
- `/api/client` returns client IP and server time.
- `/api/connectivity/targets` returns the fixed fufu Base URLs.
- `/api/newapi/sites` returns sanitized configured API sites.
- `/api/newapi/overview` returns usage logs, stats, and model availability.

## Required GitLab Variables

Set these in `Settings -> CI/CD -> Variables`.

| Variable | Required | Example | Notes |
| --- | --- | --- | --- |
| `SSH_PRIVATE_KEY` | Conditionally | Private key text | Private key used by the deploy job to SSH into the target host. This pipeline supports both a normal text variable and a GitLab File variable. The job copies or writes it into `~/.ssh/id_ed25519` and then forces `ssh/scp` to use that key. Mark as `Masked` and `Protected`. |
| `SSH_PRIVATE_KEY_B64` | Conditionally | `LS0tLS1CRUdJTi...` | Base64-encoded private key. This is the recommended option when GitLab multi-line variables keep breaking the key format. If set, it takes precedence over `SSH_PRIVATE_KEY`. Mark as `Masked` and `Protected`. |
| `SSH_KNOWN_HOSTS` | Yes | `example.com ssh-ed25519 AAAAC3...` | Output of `ssh-keyscan -p 22 your-host`. Keeps SSH host key checking enabled. |
| `SSH_HOST` | Yes | `203.0.113.10` | Target server hostname or IP. |
| `SSH_USER` | Yes | `deploy` | SSH login user. It needs permission to run `docker` and `docker compose`. |
| `SSH_PORT` | No | `22` | Defaults to `22`. |
| `DEPLOY_PATH` | No | `/data/docker/network-detact` | Remote startup directory where the pipeline uploads `docker-compose.yml` and `.env`. |
| `COMPOSE_FILE` | No | `docker-compose.yml` | Local compose file path inside the repo. The deploy job uploads this file every run. |
| `COMPOSE_SERVICE_NAME` | No | `network-detact` | Service name used by the health-check wait loop after `docker compose up -d`. |
| `CONTAINER_NAME` | No | `network-detact` | Final Docker container name written into the remote `.env`. |
| `HOST_PORT` | No | `8080` | Host port exposed by Compose. |
| `APP_IMAGE` | No | `registry.gitlab.com/group/project` | Defaults to `$CI_REGISTRY_IMAGE`. This pipeline still logs in to `$CI_REGISTRY`, so only override this when the image stays on the same registry host and you only need a different repository path. |
| `APP_TAG` | No | `v1.2.3` | Defaults to Git tag on tag pipelines, otherwise commit SHA. Use only if you deliberately want to deploy a different tag. |
| `BASE_IMAGE` | No | `registry.example.com/node:22-alpine` | Optional Dockerfile base image override. Use this when you already host a reachable Node base image and want to bypass direct Docker Hub access. |
| `DEFAULT_BASE_IMAGE` | No | `node:22-alpine` | Default base image used when `BASE_IMAGE` is not set. Defined in `.gitlab-ci.yml`. |
| `REGISTRY_MIRRORS` | No | `https://mirror.example.com` | Optional comma-separated mirror list. The dind service uses these as registry mirrors, and the build job also tries them as prefixes for the base image when Docker Hub is unreachable and `BASE_IMAGE` is not explicitly set. |
| `NEWAPI_MANAGED_API_SITES` | No | `{"managedApiSites":[...]}` | Optional one-line JSON source config for the fixed API station dashboard. Mark as `Masked` and `Protected` because it contains NewAPI access tokens. |
| `NEWAPI_MANAGED_API_CONFIG` | No | `/run/secrets/newapi-managed-api-sites.json` | Optional in-container config path. Use this when mounting the source config instead of passing JSON as an environment variable. |
| `NEWAPI_API_SITE_TOKEN` | Yes for checked-in config | Token text | API 次数站 token. Mark as `Masked` and `Protected`. |
| `NEWAPI_TOKEN_SITE_TOKEN` | Yes for checked-in config | Token text | Token 站 token. Mark as `Masked` and `Protected`. |
| `NEWAPI_API_SITE_URL` | No | `https://api.fufuflower.top` | Optional API 次数站 URL override. |
| `NEWAPI_TOKEN_SITE_URL` | No | `https://token.fufuflower.top` | Optional Token 站 URL override. |
| `CONNECTIVITY_API_URLS` | No | comma-separated URLs | Optional URL detection targets for API sites. |
| `CONNECTIVITY_TOKEN_URLS` | No | comma-separated URLs | Optional URL detection targets for Token sites. |
| `DEPLOY_REGISTRY_USER` | No | `gitlab+deploy-token-123` | Recommended when the target host should use a long-lived registry credential instead of the short-lived job token. |
| `DEPLOY_REGISTRY_PASSWORD` | No | Token text | Paired with `DEPLOY_REGISTRY_USER`. Mark as `Masked` and `Protected`. |

## Registry Variables Already Provided By GitLab

These do not need manual creation when GitLab Container Registry is enabled:

- `CI_REGISTRY`
- `CI_REGISTRY_IMAGE`
- `CI_REGISTRY_USER`
- `CI_REGISTRY_PASSWORD`

The pipeline uses those built-in variables for image push. The deploy step also falls back to them for remote `docker login` if you do not provide `DEPLOY_REGISTRY_USER` and `DEPLOY_REGISTRY_PASSWORD`.

If you need to push or pull from a completely different registry host, this pipeline is not enough as-is. You would also need to parameterize the login target instead of relying on `$CI_REGISTRY`.
If your runner cannot reach Docker Hub reliably, prefer setting `BASE_IMAGE` to a reachable image path. `REGISTRY_MIRRORS` is a fallback helper, not a guarantee that every mirror layout matches Docker Hub exactly.
For SSH private keys, prefer `SSH_PRIVATE_KEY_B64` when possible. It avoids the most common formatting mistakes caused by copying multi-line OpenSSH keys into GitLab variables.

## GitLab Runner Requirements

The `build_image` job uses `docker:27-cli` with `docker:27-dind`. Your GitLab runner must support Docker-in-Docker for that job to work.

Minimum runner prerequisites:

1. Docker executor with `privileged = true`, or an equivalent runner configuration that supports `docker:dind`.
2. A service alias named `docker` reachable from the job container. This pipeline starts dind with `dockerd-entrypoint.sh --tls=false --host=tcp://0.0.0.0:2375` and connects through `DOCKER_HOST=tcp://docker:2375`.
3. Network access from the runner to the GitLab Container Registry.
4. Enough disk space for the image build and layer cache.

If your runner cannot satisfy those requirements, change the image build strategy before relying on this pipeline.
The build job waits for `docker info` to succeed before running `docker build`, so daemon startup failures show up earlier and with a clearer error.

## Server Requirements

Before the first deploy, prepare the target host:

1. Install `docker` and `docker compose`.
2. Ensure the deploy user can run Docker commands.
3. Open the host port you configured, default `8080`.
4. Make sure the server can reach `registry.gitlab.com`.

You do not need to pre-create the compose file on the server. The pipeline uploads it on every deploy to avoid config drift.
The default startup directory is `/data/docker/network-detact`. Keep any future bind mounts or app data under this directory so data stays with the deployment root.
The deploy job waits up to about 150 seconds for the container to become healthy, and if it still fails it prints `docker compose logs` plus `docker inspect` output from the remote host.

## Useful Commands

Generate `SSH_KNOWN_HOSTS` locally:

```bash
ssh-keyscan -p 22 your-host.example.com
```

Run the production image locally after building:

```bash
docker build -t network-detact:local .
docker run --rm -p 8080:8080 network-detact:local
```

Inspect the deployment on the server:

```bash
cd /data/docker/network-detact
docker compose --env-file .env -f docker-compose.yml ps
docker compose --env-file .env -f docker-compose.yml logs --tail=200
```

## Notes On The Included Comments

- [`.gitlab-ci.yml`](../.gitlab-ci.yml) includes inline comments explaining workflow rules, Docker defaults, mirror handling, and deploy behavior.
- [`docker-compose.yml`](../docker-compose.yml) includes inline comments explaining how image/tag variables arrive on the server.
- [`Dockerfile`](../Dockerfile) keeps the runtime simple: Node serving the backend API, separated frontend assets, and `/api/health`.
