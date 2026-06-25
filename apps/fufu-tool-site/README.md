# fufu-tool-site 内部结构

这是当前唯一生产 Go 服务，负责编排导航、状态页、合卡、活动和统一后台。

## 目录

- `config/`：默认非密钥 JSON 配置与样例。
- `web/status/`：API / NewAPI 模型状态原生静态页。
- `web/combine/`：合卡原生静态页。
- `ui/`：统一首页与后台 React 源码；构建产物是 ignored 的 `ui-dist/`。
- `data/`、`logs/`、`node_modules/`、二进制和 `ui-dist/` 都是本地运行/构建产物，不属于源码结构。

## Go 文件分组

- `main.go`：进程入口和顶层启动流程。
- `runtime.go` / `runtime_state.go`：运行时初始化、嵌入模块路径、共享状态和缓存。
- `http_routes.go` / `api_routes.go` / `static.go`：路由表与静态资源 adapter。
- `admin_*`：统一后台配置、持久化、会话和运行时应用。
- `model_status_*`：模型状态抓取、缓存、站点构建和响应投影。
- `model_manual_*`：手动模型测试 handler、runner 和缓存投影。
- `navigation_*`：导航配置和 API。

能抽成纯函数/纯对象规则的代码应优先进入 `packages/go/fufu/*core`；需要 HTTP、SQLite、env 或嵌入模块状态的代码留在本 app。
