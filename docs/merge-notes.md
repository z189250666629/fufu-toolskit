# 合并记录

本次合并把三个项目整理到 `apps/` 下，保留独立运行方式。

## 来源

- `apps/fufu-combine`：来自 GitHub 仓库 `z189250666629/fufu-combine`。
- `apps/network-detect`：来自 GitHub 仓库 `z189250666629/network-detect`。
- `apps/fufu-act`：来自本机目录 `C:\Users\z1892\project\fufu_act`，即用户所说的 `activity`。

## 处理过的内容

- 未复制 `.git/`、`node_modules/`。
- 未复制 `apps/fufu-act/data/*.db*` 运行数据库。
- 未复制 `apps/fufu-act/.claude/` 和 `apps/fufu-act/backups/`。
- 给 `fufu-act` 和 `fufu-combine` 添加了空的 `data/.gitkeep`。
- 根目录新增统一 `package.json`、`.gitignore`、`.env.example`、`README.md`。
- `apps/fufu-act/scripts/api-act.mjs` 原先依赖外部本机路径 `../../skills/fufu-shop/scripts/api.mjs`，合并后已改为自包含实现，避免搬迁后模块解析失败。

## 当前边界

这次没有把三个服务强行合成一个 HTTP 进程，也没有把 CI/CD 从子项目提升到仓库根目录。后续如果需要，可以再做：

1. 根目录统一反向代理或网关。
2. 统一 Docker Compose，一键部署三个服务。
3. 统一 GitHub Actions / GitLab CI。
4. 统一配置中心和密钥管理。
