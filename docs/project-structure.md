# 项目目录结构约定

这份文档只回答一个问题：代码应该放到哪里。后续整理目录时先对照这里，避免再把生产应用、历史源码和本地脚本混在一起。

## 顶层分层

| 目录 | 放什么 | 不放什么 |
| --- | --- | --- |
| `apps/` | 当前仍参与统一工具站运行、嵌入或本地启动的业务模块 | 历史参考代码、本地一次性脚本、共享库 |
| `packages/` | 可被多个 app 复用的共享库；当前 Go 共享库在 `packages/go/fufu` | HTTP 入口、SQLite/env 适配、页面静态资源 |
| `legacy/` | 已退出生产入口、只作为迁移参考或回归样本保留的历史模块 | `go.work` 活跃模块、生产 Docker context、根级测试入口 |
| `tools/` | 本地运营/维护脚本，例如商城批量操作辅助 | 用户前端、生产服务、Docker runtime |
| `scripts/` | 仓库级启动、测试、部署脚本 | 业务逻辑、页面代码 |
| `infra/` | 当前生产部署编排 | 历史服务 compose |
| `docs/` | 需求、边界、部署和结构说明 | 可执行代码 |
| `tests/` | 仓库级结构/工作区守护测试，当前 Go workspace 测试在 `tests/workspace` | 业务模块源码、运行产物 |

## 当前应用归属

```text
apps/
  fufu-tool-site/   # 唯一生产服务：导航、状态页、合卡、活动嵌入、统一后台
    config/         # 默认配置样例与非密钥 JSON 配置
    web/status/     # 状态页原生静态资源
    web/combine/    # 合卡页原生静态资源
    ui/             # 统一首页与后台 React 源码
  fufu-act/         # activity 后端与 public 静态资源，被 fufu-tool-site 嵌入
  y2k-nav/          # 导航页历史资源与交互资源，被 fufu-tool-site 嵌入

packages/go/fufu/   # 共享 Go 包

tests/
  workspace/        # 仓库级 Go workspace/部署结构守护测试

legacy/
  network-detect/   # 历史状态面板源码；不进 go.work，不进生产 Docker context

tools/
  mcy-card-upload/  # 本地 MCY 运营脚本；不进 apps，不进生产前端
```

## `apps/fufu-tool-site` 内部分层

`fufu-tool-site` 是 Go `package main`，HTTP adapter 必须留在同一个目录编译；因此先靠文件前缀和职责边界分层，不把路由、运行时状态和业务规则混在入口文件里。

| 文件/目录 | 职责 |
| --- | --- |
| `main.go` | 进程入口、端口解析和顶层启动流程 |
| `runtime.go` | 初始化嵌入模块、数据目录、统一配置和 HTTP server adapter |
| `runtime_state.go` | 包级运行时状态、缓存、跨文件共享类型别名 |
| `http_routes.go` / `api_routes.go` / `static.go` | 页面路由、API 路由表、静态资源分发 |
| `admin_*` | 统一后台配置、会话、限流和运行时应用 |
| `model_status_*` | 模型状态缓存、抓取、站点构建、投影 |
| `model_manual_*` | 手动模型测试 handler、runner、缓存投影 |
| `navigation_*` | 首页导航配置和导航 API |
| `web/status` / `web/combine` / `ui` | 原生状态页、原生合卡页、React 统一首页/后台 |

新逻辑优先按现有前缀归位：纯对象规则放进 `packages/go/fufu/*core`，HTTP、SQLite、env、嵌入模块 glue 留在 app adapter。

## 判断规则

1. 能被普通用户或生产服务直接访问的入口，才允许放进 `apps/`。
2. 只依赖普通对象、适合复用的业务规则，优先放进 `packages/go/fufu/*`。
3. 已下线但暂时要保留的模块，放进 `legacy/`，并从活跃 workspace 和 Docker context 隔离。
4. 带人工操作、批量导入、商城凭据或临时维护语义的脚本，放进 `tools/`。
5. 构建产物、运行数据库、日志和临时文件只允许出现在 ignored 路径，不作为目录结构的一部分。
6. 根目录只放仓库入口文件和通用配置；测试文件归入 `tests/`，不要在根目录堆放临时二进制、coverage 或日志。
