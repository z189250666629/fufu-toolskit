# fufu-toolskit 需求整理

本文是项目整理的起点。后续目录、模块、接口、后台和部署整理，都要先能对应到这里的需求与验收标准。

## 谁用，怎么用，达成什么效果

### 普通访问用户

普通用户从 `fufu-tool-site` 首页进入工具集合，不需要知道后端曾经拆成多个独立应用。

用户怎么用：

- 打开 `/`，查看 API 次数站、Token 站、Web Terminal、Build、状态页、合卡、活动入口。
- 进入 `/status` 检查 API / NewAPI 模型状态、连通性、手动测试结果。
- 进入 `/combine` 自助合并额度卡。
- 进入 `/activity` 使用抽奖、刮刮卡等活动能力。

要达成的效果：

- 一个统一入口能完成常用工具导航。
- 首页线路跟后台配置保持一致；没有配置时仍显示默认公开线路。
- 用户不需要切换多个端口，也不需要理解 `network-detect`、`fufu-act`、`y2k-nav` 的历史拆分。

### 运营管理员

运营管理员从 `/admin` 维护业务配置和运行状态。

管理员怎么用：

- 登录统一后台。
- 配置 NewAPI 次数站 / Token 站：一个站点配置一次 token，可维护多条 `base_url` 线路。
- 查看首页实际展示线路，以及状态页/合卡实际运行站点。
- 配置 MCY 商城登录，用于活动核销等既有流程；自动补卡与库存检测暂时不对接商城。
- 配置首页工具入口，包括 Web Terminal 多线路、Build、状态页、合卡、活动等卡片。
- 查看活动卡档参考；自动补卡、手动补卡和 MCY 库存检测暂时下线。
- 查看活动统计、运行时概率、活动窗口、各玩法的目标期望值/实际期望值/抽奖次数配置、卡档玩法配置，以及大奖/二奖/三奖等可展示奖项。

要达成的效果：

- 站点、合卡、状态页、首页线路复用同一套 NewAPI 配置，不重复维护。
- 敏感值只在后台保存和使用，不在首页或普通接口明文暴露。
- 业务配置保存后直接应用，重启或重新部署不需要重新填环境变量。
- `/activity-admin` 不再暴露 activity 原始后台；旧地址统一重定向到 `/admin`。

### 运维/发布人员

运维人员只部署一个生产服务：`fufu-tool-site`。

运维怎么用：

- 本地使用 `npm run start:tool-site` 或 `npm run start:all` 启动统一服务。
- 发布时使用 tag + deploy directive 触发 `deploy-fufu-tool-site`。
- GitHub Environment 固定使用 `toolskit`。
- 通过 `ADMIN_TOKEN`、NewAPI、MCY 等既有变量完成首次启动种子配置。

要达成的效果：

- 生产只暴露统一服务端口 `38473`，旧的 `33148/y2k-nav`、`18820/fufu-act` 不再作为独立生产入口。
- CI/CD、Docker context、忽略规则都围绕统一服务收束。
- 不新增重复、猜测或历史遗留的环境变量。

### 开发维护人员

开发者需要保留模块化边界，而不是把所有代码压成一个不可维护的大文件。

开发者怎么用：

- 在 `apps/fufu-tool-site` 维护统一生产服务和业务入口。
- 在 `apps/fufu-act` 维护 activity 模块能力与独立测试边界。
- 在 `apps/y2k-nav` 维护导航模块的历史资源与测试边界。
- 在 `apps/network-detect` 保留历史状态面板源码，生产不再单独部署。
- 在 `packages/go/fufu/*` 维护共享能力。

要达成的效果：

- 生产入口统一，但代码仍按业务能力和共享能力分层。
- 公共逻辑优先进入 `packages/go/fufu/*`，避免在 app 间复制。
- 每次整理都能通过根级测试和模块级测试验证。

## 需求边界

### 必须满足

- 生产入口只有 `apps/fufu-tool-site`。
- `/`、`/status`、`/combine`、`/activity`、`/admin` 在同一服务内可用。
- `/admin` 是统一管理后台，覆盖站点配置、首页线路、首页工具入口、状态/合卡运行站点、MCY、补卡、活动统计和活动配置。
- `/activity-admin` 只作为旧地址兼容跳转到 `/admin`，不再公开原始 activity 后台。
- NewAPI 配置采用“一个站点 token + 多条 base_url 线路”的模型。
- 首页 API/Token 线路从运行配置读取；未配置时使用默认公开线路。
- 管理后台保存的配置写入 SQLite `tool-config.db`；环境变量只作为首次启动种子。
- 敏感信息不得提交到仓库，不得在普通用户页面泄露。
- `fufu-act`、`y2k-nav`、`network-detect` 的保留必须有明确理由：嵌入、测试、历史兼容或后续迁移参考。
- 构建产物、运行数据库、`node_modules`、日志、临时文件不得进入 git 或 Docker build context。

### 暂不纳入

- 不恢复 `y2k-nav`、`fufu-act`、`network-detect` 的独立生产部署。
- 不新增旧式额外访问密钥变量，除非明确重新设计访问控制。
- 不把 `apps/mcy-card-upload/` 这类本地敏感工具直接并入前端；若要集成，必须先改成服务端配置和脱敏展示。
- 不把所有模块合并成单目录；统一的是入口和运营体验，不是抹掉模块边界。

## 业务功能清单

### 首页导航

用户故事：

- 普通用户打开 `/`，可以看到 API 次数站、Token 站和各工具入口。
- 运营配置 NewAPI 多线路后，首页展示对应线路名称和 URL。
- 运营配置首页工具入口后，首页展示后台保存的工具卡片；接口异常时保留静态 fallback，避免首页不可用。

验收标准：

- `/api/nav/lines` 返回 `api`、`token` 两类线路。
- 配置存在时返回后台配置的线路。
- 配置不存在时返回默认公开线路。
- 首页仅展示 URL 和线路名，不展示 token。
- `/api/nav/tools` 返回后台保存的工具卡片；未配置时返回默认工具入口。

### API / 模型状态

用户故事：

- 用户进入 `/status` 查看站点连通性、模型可用性和测试结果。
- 管理员在后台修改 NewAPI 站点后，状态页读取同一套配置。

验收标准：

- 状态页、连通性检测、模型状态使用统一配置。
- 配置缺失时给出明确错误，不把内部错误或敏感数据暴露给前端。
- 手动测试有冷却和错误脱敏。

### 合卡工具

用户故事：

- 用户进入 `/combine` 合并额度卡。
- 合卡默认复用后台配置的 API 次数站主线路。

验收标准：

- 合卡不维护独立 NewAPI 配置。
- NewAPI 站点配置变化后，合卡运行配置同步更新。
- 合并失败、删除失败、回滚失败等风险状态有明确错误信息。

### 活动前台

用户故事：

- 用户进入 `/activity` 使用抽奖、刮刮卡和福利入口。

验收标准：

- 活动前台继续由 activity 模块承载。
- 卡密校验、抽奖、刮刮卡、打款队列等能力保留模块测试。
- 前端错误展示要脱敏，不能直接渲染上游 5xx 原文。

### 统一管理后台

用户故事：

- 管理员进入 `/admin`，在一个后台完成站点、补卡和活动配置。

验收标准：

- 登录态使用 `fufu_admin_session`，后台接口仍可兼容 Bearer `ADMIN_TOKEN`。
- NewAPI 站点配置支持 API 次数站 / Token 站分组，每组一份 token、多条 URL。
- 后台能展示首页导航线路和实际运行站点。
- 后台能编辑首页工具入口，不再把 Web Terminal、Build 等入口硬编码为唯一数据源。
- MCY 密码留空表示沿用原值；展示时只显示已设置/脱敏。
- 补卡卡档配置仍可读取给活动配置使用；补卡运行接口和库存检测接口暂时返回下线提示，不对接 NewAPI/MCY。
- 活动配置保存后立即作用于运行态。

### 部署与运维

用户故事：

- 运维通过一个 workflow 部署统一服务。

验收标准：

- 只有 `deploy-fufu-tool-site` 作为生产部署 workflow。
- 旧 deploy directive 只作为兼容别名触发统一服务。
- Docker compose 默认外部端口为 `38473`。
- `toolskit` 是固定 GitHub Environment。
- token、password、cookie、`ADMIN_TOKEN` 优先放 Secrets；URL、端口、路径放 Variables。

## 模块边界

| 模块 | 职责 | 不负责 |
| --- | --- | --- |
| `apps/fufu-tool-site` | 统一生产服务、路由整合、统一后台、首页 UI、状态页、合卡入口、activity 嵌入 | 承担所有底层业务算法 |
| `apps/fufu-act` | 活动后端、抽奖/刮刮卡/补卡接口、activity 前台静态资源和测试边界 | 独立生产部署、原始管理后台 |
| `apps/y2k-nav` | 导航模块历史资源、视觉/交互测试边界 | 独立生产部署 |
| `apps/network-detect` | 历史状态面板源码与迁移参考 | 独立生产部署 |
| `packages/go/fufu/config` | 配置加载、站点配置归一化 | UI 表现 |
| `packages/go/fufu/newapi` | NewAPI 客户端与响应处理 | 业务页面路由 |
| `packages/go/fufu/tokens` | token 查询、校验、变更等共享能力 | 页面渲染 |
| `packages/go/fufu/combine` | 合卡业务核心和 API handler | 统一 app 入口路由 |
| `packages/go/fufu/activity` | 活动配置、统一奖池、玩法参数和抽奖规则装配 | activity HTTP 路由 |
| `packages/go/fufu/probabilitycore` | 期望值反馈概率算法 | 奖池展示、活动 HTTP 路由 |
| `packages/go/fufu/prizepoolcore` | 奖池定义、展示奖项、抽取 | 期望值反馈算法、活动 HTTP 路由 |
| `packages/go/fufu/auth` | 管理员 token 检查等轻量鉴权工具 | session cookie 存储 |
| `packages/go/fufu/webutil` | HTTP/静态资源/JSON/端口等通用工具 | 业务决策 |

## 整理顺序

1. 固化需求和验收标准，也就是本文。
2. 补结构护栏测试，防止生产入口、环境变量、构建产物和模块边界回退。
3. 按业务入口整理 `/admin`、`/api/nav/lines`、NewAPI 配置、补卡、活动配置等契约。
4. 清理重复文档、历史入口和部署说明，确保 README、CI/CD、merge notes 说法一致。
5. 再做目录或代码重构，优先抽共享逻辑到 `packages/go/fufu/*`。
6. 最后跑根级验证：`npm test`、`npm run build:tool-site`，必要时再做本地或 Mac 隧道验证。

## 当前风险与待确认

- `apps/network-detect` 保留多少源码作为迁移参考，需要后续按文件粒度再审。
- 原始 activity 管理后台已撤掉；后续只需要确认旧 `/activity-admin` 跳转兼容地址是否也删除。
- MCY 相关配置已经进入统一后台，但如果要集成本地敏感工具，必须先完成服务端化和脱敏。
- 如果后续恢复自动补卡，需要重新确认后台常驻 scheduler、库存检测、补卡执行日志和失败重试策略。
