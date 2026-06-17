# mcy-card-upload

萌次元商城（fufu 小店）虚拟卡密管理脚本。两个 Node.js 脚本共用 `mcy-client.mjs` 加密客户端，无第三方依赖（仅用内置 `crypto` / `fs`）。

## 凭据

登录凭据**从环境变量读取**，不要写死在代码里：

| 变量 | 说明 |
| --- | --- |
| `MCY_USERNAME` | 商城后台登录邮箱（必填） |
| `MCY_PASSWORD` | 商城后台登录密码（必填） |
| `MCY_BASE_URL` | 可选，默认 `https://shop.fufuflower.top` |

PowerShell 示例：

```powershell
$env:MCY_USERNAME = 'you@example.com'
$env:MCY_PASSWORD = '***'
```

## upload.mjs — 批量上传卡密

```bash
node upload.mjs --list                                              # 列出商品和 SKU
node upload.mjs --item <item_id> --sku <sku_id> --file <cards.txt>  # 上传
#   [--remark <备注>] [--unique]                                     # 可选
```

卡密文件一行一个，按每批 500 张分批提交。

## fufu-shop.mjs — 查询商品 / 库存 / 卡密

```bash
node fufu-shop.mjs            # 总览：商品列表 + 库存统计
node fufu-shop.mjs --stock    # 各 SKU 可用库存汇总
node fufu-shop.mjs --cards [--item <id>] [--sku <id>] [--status <0|1>] [--page <n>] [--limit <n>]
```
