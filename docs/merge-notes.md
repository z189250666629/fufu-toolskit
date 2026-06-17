# Merge Notes

本次合并后的生产形态是一个对外应用：`fufu-tool-site`。它把导航页、API/模型状态、合卡、活动前台和统一管理后台合并到同一个服务中。

## 来源与模块边界

- `apps/fufu-tool-site`：统一生产服务，复用原 `network-detect` 的状态面板和合卡后端能力。
- `apps/y2k-nav`：保留为首页导航静态资源模块，被 `fufu-tool-site` 的 `/` 嵌入。
- `apps/fufu-act`：保留为 activity 后端与 `public/` 前台静态资源模块，被 `fufu-tool-site` 的 `/activity` 和 `/api/admin/*` 等路由嵌入。
- `apps/network-detect`：历史模块源码保留，生产部署不再单独指向它。
- `apps/fufu-combine`：已迁移至统一服务与 `packages/go/fufu/combine`，不再独立部署。

## 未迁移/未提交内容

- 未复制运行数据库，如 `apps/**/data/*.db*`。
- 不提交 `.env*`、`config.json`、cookie、token 或商城账号密码。
- `apps/mcy-card-upload/` 若存在本地硬编码凭据，不进入统一前端；集成前必须先改为服务端/env 配置。

## 当前生产入口

- `/`：fufu 工具站导航页。
- `/status`：API/模型状态。
- `/combine`：合卡。
- `/activity`：活动前台。
- `/admin`：统一管理后台。
- `/activity-admin`：旧 activity 后台兼容地址，重定向到 `/admin`。

旧 `33148/y2k-nav` 与 `18820/fufu-act` 独立生产入口已退休，统一外部端口继续使用 `38473`。
