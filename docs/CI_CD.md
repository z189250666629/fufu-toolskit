# CI/CD Runbook

本仓库的部署参考 `tte-system` 的 tag-gated 模式：生产部署只由 `v*` tag 触发，并且 tag 指向的 commit message 必须包含明确的 deploy directive。

## 核心规则

- 普通 branch push 不部署。
- 生产部署只认 `v*` tag。
- pre-release tag 不部署：任何包含 `-` 的 tag（如 `v1.2.3-rc1`）都会跳过。
- tag 还必须匹配 commit directive；没有 directive 的 tag 只跑 gate，然后跳过部署。
- 三个应用分成三份 workflow，互不阻塞：
  - `.github/workflows/deploy-combine.yml`
  - `.github/workflows/deploy-act.yml`
  - `.github/workflows/deploy-network.yml`

## 三段式部署

| Workflow | 目录 | 镜像 | 默认远端路径 | 默认端口 | Directive |
| --- | --- | --- | --- | ---: | --- |
| `deploy-combine` | `apps/fufu-combine` | `ghcr.io/<owner>/<repo>-fufu-combine` | `/data/docker/fufu-combine` | `3456` | `[deploy combine]` / `[deploy fufu-combine]` |
| `deploy-act` | `apps/fufu-act` | `ghcr.io/<owner>/<repo>-fufu-act` | `/data/docker/fufu-act` | `18820` | `[deploy act]` / `[deploy activity]` / `[deploy fufu-act]` |
| `deploy-network` | `apps/network-detect` | `ghcr.io/<owner>/<repo>-network-detect` | `/data/docker/network-detact` | `8080` | `[deploy network]` / `[deploy network-detect]` / `[deploy network_detect]` |

`[deploy all]` 会同时触发三份部署。

## 示例

只部署活动系统：

```bash
git commit -m "chore: release activity [deploy act]"
git tag v1.2.3
git push origin main
git push origin v1.2.3
```

同时部署三部分：

```bash
git commit -m "chore: release toolkit [deploy all]"
git tag v1.2.4
git push origin main
git push origin v1.2.4
```

预发布 tag 会跳过部署：

```bash
git tag v1.2.5-rc1
git push origin v1.2.5-rc1
```

## 每份 workflow 的阶段

每份部署 workflow 都包含：

1. `check`：检查 tag、过滤 pre-release、检查 commit directive。
2. `verify`：运行对应 app 的基础检查。
3. `docker`：构建并推送 GHCR 镜像。
4. `deploy`：SSH 到 VPS，上传 compose/env/config，执行 `docker compose pull && docker compose up -d`，等待健康检查。

## 远端 Docker Compose

compose 模板在：

```text
infra/deploy/fufu-combine/docker-compose.yml
infra/deploy/fufu-act/docker-compose.yml
infra/deploy/network-detect/docker-compose.yml
```

通用部署脚本：

```text
scripts/deploy-docker-app.sh
```

脚本会在远端部署目录写入：

- `.env`：compose 环境变量。
- `config.json`：仅 `fufu-combine` 需要，由 GitHub Secrets/Variables 生成。
- `data/`：运行数据目录。

## GitHub Variables

在 GitHub Repo → Settings → Secrets and variables → Actions 配置。

### 通用 Variables

| Name | 说明 |
| --- | --- |
| `SSH_HOST` | VPS IP/域名 |
| `SSH_PORT` | SSH 端口，默认 `22` |
| `SSH_USER` | SSH 用户 |
| `HOST_BIND` | 默认绑定地址，默认 `0.0.0.0` |

也兼容旧命名：`DEPLOY_HOST`、`DEPLOY_PORT`、`DEPLOY_USER`。

### 每个服务的可选 Variables

| Name | 默认值 |
| --- | --- |
| `FUFU_COMBINE_DEPLOY_PATH` | `/data/docker/fufu-combine` |
| `FUFU_COMBINE_HOST_PORT` | `3456` |
| `FUFU_ACT_DEPLOY_PATH` | `/data/docker/fufu-act` |
| `FUFU_ACT_HOST_PORT` | `18820` |
| `NETWORK_DETECT_DEPLOY_PATH` | `/data/docker/network-detact` |
| `NETWORK_DETECT_HOST_PORT` | `8080` |

### 业务配置 Variables

| Name | 用途 |
| --- | --- |
| `FUFU_COMBINE_API_URL` | fufu-combine 上游 NewAPI URL；未设置时回退 `NEWAPI_API_SITE_URL` |
| `FUFU_COMBINE_USER_ID` | fufu-combine `config.json` 的 `userId`，默认 `1` |
| `FUFU_COMBINE_QUOTA_UNIT` | 默认 `500000` |
| `FUFU_API_BASE_URL` | fufu-act NewAPI URL；未设置时回退 `NEWAPI_API_SITE_URL` |
| `FUFU_API_USER_ID` | fufu-act NewAPI user id，默认 `1` |
| `FUFU_QUOTA_UNIT` | 默认 `500000` |
| `MCY_BASE_URL` | fufu-act 可选商城地址 |
| `MCY_LOGIN_ENDPOINT` | 默认 `/admin/login` |
| `MCY_UPLOAD_ENDPOINT` | 默认 `/plugin/virtual-card-ship/card/add` |
| `NEWAPI_API_SITE_URL` | network-detect / fufu-act 可共用 |
| `NEWAPI_TOKEN_SITE_URL` | network-detect |
| `CONNECTIVITY_API_URLS` | network-detect URL 检测列表 |
| `CONNECTIVITY_TOKEN_URLS` | network-detect URL 检测列表 |

## GitHub Secrets

### 通用 Secrets

| Name | 说明 |
| --- | --- |
| `SSH_PRIVATE_KEY_B64` | 推荐；CI SSH 私钥的 base64 单行内容 |
| `SSH_PRIVATE_KEY` | 可选；明文私钥，优先级低于 `SSH_PRIVATE_KEY_B64` |
| `SSH_KNOWN_HOSTS` | 可选；未设置时 workflow 会 `ssh-keyscan` |
| `GHCR_TOKEN` | 可选；远端拉 GHCR 私有镜像用。未设置时使用 workflow 的 `github.token` |
| `GHCR_USERNAME` | 可选；默认 `github.actor` |

也兼容旧命名：`DEPLOY_SSH_KEY`。

### 业务 Secrets

| Name | 用途 |
| --- | --- |
| `FUFU_COMBINE_API_TOKEN` | fufu-combine 上游 NewAPI token；未设置时回退 `NEWAPI_API_SITE_TOKEN` |
| `FUFU_API_TOKEN` | fufu-act NewAPI token；未设置时回退 `NEWAPI_API_SITE_TOKEN` |
| `NEWAPI_API_SITE_TOKEN` | network-detect / fallback token |
| `NEWAPI_TOKEN_SITE_TOKEN` | network-detect token 站 token |
| `NEWAPI_MANAGED_API_SITES` | network-detect 完整 JSON 配置，可选 |
| `NEWAPI_DASHBOARD_VIEW_KEY` | network-detect 浏览器查看密钥，可选 |
| `MCY_COOKIE` | fufu-act 商城 session cookie，可选 |
| `MCY_USERNAME` | fufu-act 商城账号，可选 |
| `MCY_PASSWORD` | fufu-act 商城密码，可选 |

## VPS 前置条件

- VPS 已安装 Docker 与 Docker Compose plugin。
- SSH 用户能执行 Docker 命令，并能写入各 `DEPLOY_PATH`。
- 对外端口建议只允许上游反代机 IP 访问。
- 首次部署前确认部署目录所在磁盘空间足够。

## 本地检查

```powershell
npm test
```

本机如果安装 Docker，可以额外检查：

```bash
docker compose -f infra/deploy/fufu-combine/docker-compose.yml config --quiet
docker compose -f infra/deploy/fufu-act/docker-compose.yml config --quiet
docker compose -f infra/deploy/network-detect/docker-compose.yml config --quiet
```
