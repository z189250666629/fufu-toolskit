# 合并记录

本次合并把工具集合整理为 3 个对外应用：`network-detect`、`fufu-act`、`y2k-nav`。`fufu-combine` 不再独立部署，页面与 API 都由 `network-detect` 承载。

## 来源

- `apps/fufu-combine`：已迁移至 `apps/network-detect` 与 `packages/go/fufu/combine`。
- `apps/network-detect`：来自 GitHub 仓库 `z189250666629/network-detect`。
- `apps/fufu-act`：来自本机目录 `C:\Users\z1892\project\fufu_act`，即用户所说的 `activity`。

## 处理过的内容

- 未复制 `.git/`、`node_modules/`。
- 未复制 `apps/fufu-act/data/*.db*` 运行数据库。
- 未复制 `apps/fufu-act/.claude/` 和 `apps/fufu-act/backups/`。
- 给 `fufu-act` 添加了空的 `data/.gitkeep`。
- 根目录新增统一 `package.json`、`.gitignore`、`.env.example`、`README.md`、`go.work`。
- 后端统一迁移到 Go；前端继续复用现有静态 HTML/CSS/JS。
- `fufu-act` 查卡和发奖通过共享 `tokens` 服务，不再直接拼 NewAPI 请求。

## 当前边界

这次没有把前端改成 React，也没有把三个对外应用强行合成一个 HTTP 进程。后续如果需要，可以再做：

1. 根目录统一反向代理或网关。
2. 统一 Docker Compose，一键部署三个服务。
3. 前端 React/Vite/TypeScript 重构。
4. 统一配置中心和密钥管理。
