# fufu-toolskit

`fufu-toolskit` 是把 FuFu 工具集合合并后的 monorepo：

| 子项目 | 目录 | 默认端口 | 说明 |
| --- | --- | ---: | --- |
| y2k-nav | `apps/y2k-nav` | `33148` | 静态导航页 |
| fufu-combine | `apps/fufu-combine` | `3456` | Go 后端 + 原生单页前端，用于合卡/生成卡 |
| fufu-act / activity | `apps/fufu-act` | `18820` | Node.js Express 活动抽奖/刮刮卡服务 |
| network-detect | `apps/network-detect` | `8080` | Node.js 网络检测和 NewAPI 用量看板 |

当前合并方式是“同仓库、独立应用”：导航页和三个服务各自保留原入口、依赖和配置，避免强行改成单个进程导致功能互相影响。

## 目录结构

```text
apps/
  y2k-nav/          # 导航页
  fufu-combine/      # 原 fufu-combine
  fufu-act/          # 原 activity / fufu_act
  network-detect/    # 原 network-detect / network_detect
scripts/
  start-all.mjs      # 同时启动三个服务
.env.example         # 环境变量示例，不含真实密钥
package.json         # 根目录统一脚本
```

## 安装依赖

根目录执行：

```powershell
npm run deps
```

也可以按需安装：

```powershell
npm run deps:combine
npm run deps:act
npm run deps:network
```

要求：

- Node.js 20+
- Go（`apps/fufu-combine/go.mod` 声明 Go 1.25.0）

## 启动

分别启动：

```powershell
npm run start:combine
npm run start:act
npm run start:network
```

同时启动三个服务：

```powershell
npm run start:all
```

访问地址：

- fufu-combine: `http://127.0.0.1:3456/`
- fufu-act/activity: `http://127.0.0.1:18820/`
- network-detect: `http://127.0.0.1:8080/`

## 配置

### fufu-combine

```powershell
Copy-Item apps/fufu-combine/config.example.json apps/fufu-combine/config.json
```

然后编辑 `apps/fufu-combine/config.json`。真实 `config.json` 已被 `.gitignore` 忽略。

### fufu-act / activity

复制 `.env.example` 为 `.env`，或在启动前设置环境变量：

```powershell
$env:FUFU_API_BASE_URL = 'https://api.fufuflower.top'
$env:FUFU_API_TOKEN = '<newapi-admin-token>'
$env:FUFU_API_USER_ID = '1'
$env:SLOT_PORT = '18820'
```

如果需要通过 MCY 商城校验购买时间或执行自动补货，还需要：

```powershell
$env:MCY_BASE_URL = 'https://shop.example.com'
$env:MCY_COOKIE = '<session-cookie>'
# 或者使用账号密码登录：
$env:MCY_USERNAME = '<username>'
$env:MCY_PASSWORD = '<password>'
```

`apps/fufu-act/scripts/api-act.mjs` 已改为自包含模块，不再依赖原本本机上级目录里的 `skills/fufu-shop`。

### network-detect

参考 `apps/network-detect/README.md`。常用变量：

```powershell
$env:NEWAPI_API_SITE_TOKEN = '<api-site-admin-token>'
$env:NEWAPI_TOKEN_SITE_TOKEN = '<token-site-admin-token>'
$env:PORT = '8080'
```

## 检查

```powershell
npm test
```

该命令会运行：

- `go test ./...`（fufu-combine）
- `node --check`（fufu-act）
- `npm --prefix apps/network-detect test`（network-detect）

## 不应提交的内容

根目录 `.gitignore` 已忽略：

- 真实密钥：`.env*`、`config.json`
- 依赖目录：`node_modules/`
- 运行数据库：`apps/**/data/*.db*`
- 构建产物和日志

`apps/fufu-act/data/.gitkeep` 和 `apps/fufu-combine/data/.gitkeep` 只是为了保留运行数据目录。

## CI/CD

打标后的部署已拆分为 y2k-nav、fufu-combine、fufu-act/activity、network-detect。详见 docs/CI_CD.md。

