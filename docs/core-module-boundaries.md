# Core 模块边界整理

这次整理使用同一把尺子：能只用数字、字符串、数组、map/struct 等普通对象测完的规则进入 core；需要 HTTP、DB、文件、环境变量、浏览器、NewAPI、MCY 或其它外部站点的逻辑留在 app 外层。

当前 core 只保留业务字段和普通 DTO，不导入 `fufu/newapi`，也不导入带 NewAPI service 语义的 `fufu/tokens`。`admincore` 用 `FlatManagedSite` 表达后台站点配置扁平形态，`modelcore` 用 `PublicSite` / `PricingSite` 表达状态页需要的站点字段；和真实 `newapi.Site` / `newapi.PublicSite` 的转换都放在 app 层适配器里。

## 已抽出的 core 包

| core 包 | 涉及业务模块 | 进 core 的内容 | 留在外层的内容 |
| --- | --- | --- | --- |
| `packages/go/fufu/admincore` | 统一后台、NewAPI 站点配置、MCY 配置展示 | 普通 DTO 的站点分组、多 base_url 去重、token 沿用规则、MCY base URL 清理、secret 脱敏响应 | `newapi.Site` 转换、SQLite 配置库、旧 JSON 迁移、env 首次种子、admin session、HTTP handler、运行态 apply |
| `packages/go/fufu/navcore` | 首页导航、后台工具入口配置 | 工具卡片默认值、card/link 归一化、accent 校验、标题生成 ID、导航响应对象 | `/api/nav/*` 路由、运行时配置读取、首页渲染和浏览器交互 |
| `packages/go/fufu/modelcore` | API/模型状态页、手动模型测试、历史 `network-detect` | 普通 DTO 的站点状态、日志/频道 raw 数据清洗、分组、状态统计、模型行汇总、候选频道排序、测试 endpoint/key 生成 | `newapi.Site`/`newapi.PublicSite` 转换、NewAPI 拉日志/频道/价格、缓存、冷却、手动测试 HTTP 调用、状态页路由 |
| `packages/go/fufu/connectivitycore` | 首页/状态页连通性检测目标配置 | 公网 URL origin 规范化、私网/localhost 过滤、CONNECTIVITY_TARGETS 普通对象清洗、env 候选值优先级和默认分组对象生成 | `os.Getenv`、HTTP handler、托管站点读取、浏览器 no-cors 探测 |
| `packages/go/fufu/salecore` | 补卡、上架计划、统一后台补卡配置 | 卡种模板、slot 归类、计划校验、schedule 归一化、创建 token body、slug、生成结果 key 抽取 | `tokens.Service` 调用、NewAPI 创建 token、MCY 库存查询/上传、schedule 文件读写、常驻 scheduler、admin API |
| `packages/go/fufu/mcycore` | MCY 商城协议 | 签名字段筛选、签名、AES-CBC 编解密、PKCS7、payload 成功/消息判断、endpoint path 规范化 | MCY 登录、cookie 保存/刷新、HTTP 请求、真实库存/订单查询 |
| `packages/go/fufu/scratchcore` | 活动刮刮乐 | 格子 JSON 解析、格子合法性、踩雷/安全格判断、奖励档位、结束状态、响应对象组装 | 卡密资格校验、DB 读写、派奖队列、随机布雷、HTTP handler |
| `packages/go/fufu/dragonboatcore` | 活动端午捕粽 | 捕捞次数消耗、命中粽子计数、可剥粽子判断、剥粽开奖状态推进，并委托 `prizepoolcore` 按统一奖池抽取奖项 | 卡密资格校验、DB 读写、NewAPI token 复核、派奖队列、奖池余额读取、HTTP handler、前台动画 |
| `packages/go/fufu/probabilitycore` | 活动概率算法 | 只根据 outcome value/weight、目标期望值、实际期望值、抽奖次数计算下一轮概率权重 | 奖项名称、大奖/二奖/三奖展示、卡档路由、活动窗口、DB、HTTP、真实派奖 |
| `packages/go/fufu/prizepoolcore` | 活动奖池 | 奖项 DTO、大奖/二奖/三奖展示字段、奖池归一化、把概率权重套回奖池、按权重抽取 | 目标/实际期望值反馈算法、卡档路由、活动窗口、DB、HTTP、真实派奖 |
| `packages/go/fufu/lotterycore` | 活动抽奖兼容门面 | 兼容旧调用名，转发到 `probabilitycore` 和 `prizepoolcore` | 新增算法细节、奖池展示规则、外部适配 |
| `packages/go/fufu/spincore` | 活动老虎机抽奖流程 | 保底、强制中奖、1000 奖 retry 限制，并委托 `prizepoolcore` 应用统一奖池概率和随机抽取 | 卡密资格校验、DB 写入、剩余次数、派奖队列、HTTP handler |

## 仍作为外层适配器的模块

| 外层模块 | 角色 | 不进 core 的原因 |
| --- | --- | --- |
| `apps/fufu-tool-site` | 统一生产入口、后台、首页、状态页、合卡/activity 嵌入 | 它负责 HTTP、SQLite、env 种子、cookie/session、静态资源和运行态装配，测试不能只靠普通对象完成。 |
| `apps/fufu-act` | 活动 API、抽奖/刮刮卡执行、补卡执行、MCY 访问 | 它依赖 DB、锁、事务、NewAPI token 服务、MCY 登录/上传和 HTTP handler。 |
| `legacy/network-detect` | 历史状态面板源码和迁移参考 | 它的纯统计规则已指向 `modelcore`，剩余部分是历史 app 壳、HTTP、配置和静态资源；不进入活跃 `go.work` 或生产 Docker context。 |
| `apps/y2k-nav` | 独立导航历史资源 | 当前主要是静态服务和浏览器主题行为，纯导航配置已由 `navcore` 承担。 |

## 本轮已分发拆分的 app 层

| 模块 | 拆分结果 | 为什么不进 core |
| --- | --- | --- |
| `apps/fufu-act/login*` | `login.go` 保留 HTTP 入口，拆出 request、token/NewAPI 查询、MCY 购买查询编排、入库 store、响应构建和登录计划测试。 | 登录依赖 HTTP 请求、NewAPI token 服务、MCY 查询和 SQLite；其中登录计划虽可对象测试，但还直接使用 `tokens.Token`、活动运行配置和 app 错误语义，先留在 app 层更稳。 |
| `apps/fufu-act/credit*` | `credit.go` 保留入队，拆出 worker、SQLite store、NewAPI 加额度 adapter、失败重试/错误文案规则。 | DB 队列和 `tokens.Service.AddQuota` 都是外部依赖；纯状态规则已隔离成普通对象函数，但暂不单独升 core，避免把派奖队列的 app 语义提前公共化。 |
| `apps/fufu-tool-site/model_status_*` | `buildModelStatus` 拆为 cache/inflight、fetch orchestration、per-site build、projection/manual 投影。 | NewAPI 拉日志、频道、价格和运行时缓存都不能进 core；模型统计的纯规则继续复用 `modelcore`。 |
| `apps/fufu-tool-site/admin_config_store*` | store 生命周期、DB 读写、旧文件迁移、env seed、load/save 编排、normalize 入口分开。 | SQLite、旧 JSON 文件、env 首次种子和运行态 apply 都是外部边界；纯归一化继续委托 `admincore`。 |
| `apps/fufu-tool-site/ui/src/admin/*` | `AdminPage.tsx` 缩成认证、加载、保存和布局；站点/导航、MCY、售卡、活动配置、通用 UI 组件各自成文件；`siteNavigationConfigCore.ts` / `activityConfigCore.ts` / `saleCardConfigCore.ts` 只保留可对象测试的前端配置规则。 | React 组件、HeroUI、fetch 和页面状态不进 core；前端编辑态的纯配置变换允许保留在同目录 TS core，并用 node 表驱动测试约束。 |
| `legacy/network-detect/model_status_*` | 历史状态页的 `buildModelStatus` 同样拆为 cache、fetch orchestration、per-site build、projection/manual 投影。 | 它仍是历史 app 壳，负责 config 文件、NewAPI HTTP 拉取、缓存和静态页面，不进 core；纯统计规则继续复用 `modelcore`。 |

## 后续拆分顺序

1. 继续把 app 层薄包装的旧函数名逐步替换为直接调用 core 包；只有完全普通对象可测的规则才允许进入 core。
2. 对 `apps/fufu-act` 剩余 handler 做同样的入口/编排/store/response 拆薄，但已有 `spin`、`scratch`、`dragonboat`、`shop`、`sale-card`、`login`、`credit` 的主边界，不需要再把外部依赖硬塞进 core。
3. `legacy/network-detect` 若最终只作为迁移参考保留，可再评估是否冻结为 legacy fixture，避免继续和 `fufu-tool-site` 双写状态页前端。
