# GitHub Actions CI/CD Setup

This repository includes a GitHub Actions workflow in [`.github/workflows/ci-cd.yml`](../.github/workflows/ci-cd.yml). GitHub Actions verifies required app files, builds the Docker image on Git tags, pushes the image to GitHub Container Registry, and can deploy over SSH when `DEPLOY_ENABLED=true`.

## What The Workflow Does

1. `verify`
   Runs on pull requests, branch pushes, tags, and manual workflow runs. It checks required frontend/backend files and runs `npm test`.
2. `build_image`
   Runs only on Git tags. It builds the Docker image from [`Dockerfile`](../Dockerfile), then pushes both the Git tag and `latest` tags to GitHub Container Registry.

The current release policy is:

1. Pull requests and branch pushes run `verify`.
2. Git tags run `verify` and `build_image`.
3. The `deploy` job runs only on Git tags after image publishing and only when `DEPLOY_ENABLED=true`.

## Image Registry

By default, the workflow pushes to GitHub Container Registry:

```text
ghcr.io/<owner>/<repo>:<git-tag>
ghcr.io/<owner>/<repo>:latest
```

For this repository, the default image path is:

```text
ghcr.io/z189250666629/network-detect
```

You can override the image path with the repository variable `APP_IMAGE`. If you override it to a different registry host, the current workflow must also be adjusted to log in to that registry with matching credentials.

## Required GitHub Secrets

No custom repository secrets are required for the default GHCR image build.

The workflow uses GitHub's built-in `GITHUB_TOKEN` with `packages: write` permission to push the image.

If `DEPLOY_ENABLED=true` is used for tagged deployment, add these runtime secrets to the `docker` environment, or repository secrets if you do not use environment-scoped secrets:

| Secret | Required for deploy | Notes |
| --- | --- | --- |
| `SSH_PRIVATE_KEY` or `SSH_PRIVATE_KEY_B64` | Yes | SSH private key for the target server. |
| `SSH_KNOWN_HOSTS` | Recommended | Known hosts entry for the target server. |
| `NEWAPI_API_SITE_TOKEN` | Yes | API 次数站 NewAPI token. |
| `NEWAPI_TOKEN_SITE_TOKEN` | Yes | Token 站 NewAPI token. |

The deploy job writes these values into the remote compose env file before running `docker compose`, so the running container can read them.

## Optional GitHub Variables

Set these in `Settings -> Secrets and variables -> Actions -> Variables` only when you need to override defaults.

| Variable | Required | Default | Notes |
| --- | --- | --- | --- |
| `APP_IMAGE` | No | `ghcr.io/<owner>/<repo>` | Full image repository path. Keep it on `ghcr.io` unless you also update the login step. |
| `APP_TAG` | No | Git tag name | Override only if you deliberately want to publish a different tag. |
| `BASE_IMAGE` | No | `node:22-alpine` | Dockerfile base image override. Use this when the runner cannot reach Docker Hub directly. |
| `DEPLOY_ENABLED` | No | `false` | Set to `true` to deploy tagged builds over SSH. |
| `SSH_HOST` / `DEPLOY_HOST` | Required for deploy | none | Target server host. |
| `SSH_USER` / `DEPLOY_USER` | Required for deploy | none | Target server user. |
| `DEPLOY_PATH` | No | `/data/docker/network-detact` | Remote compose directory. |
| `HOST_PORT` / `DEPLOY_APP_PORT` | No | `8080` | Host port exposed by Compose. |
| `NEWAPI_API_SITE_URL` | No | checked-in config URL | Override API 次数站 URL. |
| `NEWAPI_TOKEN_SITE_URL` | No | checked-in config URL | Override Token 站 URL. |
| `CONNECTIVITY_API_URLS` | No | `NEWAPI_API_SITE_URL` | Comma-separated API URL detection targets; overrides `NEWAPI_API_SITE_URL` for connectivity checks. |
| `CONNECTIVITY_TOKEN_URLS` | No | `NEWAPI_TOKEN_SITE_URL` | Comma-separated Token URL detection targets; overrides `NEWAPI_TOKEN_SITE_URL` for connectivity checks. |

## GitHub Repository Settings

Before relying on image publishing:

1. Keep `Settings -> Actions -> General -> Workflow permissions` at `Read and write permissions`, or ensure package write permission is available to `GITHUB_TOKEN`.
2. Enable GitHub Packages for the repository or organization.

## Useful Commands

Create and push a release tag:

```bash
git tag v1.0.1
git push origin v1.0.1
```

Pull and run the published image:

```bash
docker pull ghcr.io/z189250666629/network-detect:v1.0.1
docker run --rm -p 8080:8080 ghcr.io/z189250666629/network-detect:v1.0.1
```

## Notes On The Included Workflow

- [`.github/workflows/ci-cd.yml`](../.github/workflows/ci-cd.yml) pushes images only for Git tags.
- Branch and pull request workflows intentionally do not publish images.
- The deploy job uploads `docker-compose.yml` and a generated compose env file to the server when `DEPLOY_ENABLED=true`.
- The container health check uses `/api/health` from the Node backend.
