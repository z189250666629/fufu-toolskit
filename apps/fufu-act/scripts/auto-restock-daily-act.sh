#!/bin/bash
# 每日特惠补货自动化脚本（Activity 版 — 每张随机 $55~$75 整数额度）

set -e

SHOP_DIR="/root/clawd/skills/fufu-shop"
ACT_DIR="/root/clawd/fufu_act"
PUSH_SCRIPT="/root/clawd/scripts/discord-push-fufu.py"
TARGET_STOCK=25
CARD_NAME="FuFu 55次混合特惠卡"
BACKUP_DIR="/root/clawd/fufu_act/backups"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)

# 额度范围
MIN_DOLLARS=55
MAX_DOLLARS=75

# 确保备份目录存在
mkdir -p "$BACKUP_DIR"

# ============================================================
# 步骤 0: 查询 Shop 库存并计算需要补货的数量
# ============================================================
echo "=== 步骤 0: 查询 Shop 库存 ==="

# 使用 fufu-shop.mjs --stock 命令获取库存
STOCK_OUTPUT=$(cd "$SHOP_DIR" && node scripts/fufu-shop.mjs --stock 2>&1)
SHOP_STOCK=$(echo "$STOCK_OUTPUT" | grep "$CARD_NAME" | grep -oP '\d+(?= 张)')

if [ -z "$SHOP_STOCK" ]; then
    echo "ERROR: 无法获取 Shop 库存数量"
    echo "$STOCK_OUTPUT"
    exit 1
fi

echo "Shop 当前库存: $SHOP_STOCK 张"

# 计算需要补货的数量
if [ "$SHOP_STOCK" -ge "$TARGET_STOCK" ]; then
    echo "库存充足 (>= $TARGET_STOCK 张)，跳过补货"
    # 推送当前库存状态
    python3 "$PUSH_SCRIPT"
    exit 0
fi

RESTOCK_COUNT=$((TARGET_STOCK - SHOP_STOCK))
echo "需要补货: $RESTOCK_COUNT 张 (目标: $TARGET_STOCK 张)"
echo "额度范围: \$$MIN_DOLLARS ~ \$$MAX_DOLLARS (随机整数)"

# ============================================================
# 第一部分：NewAPI 生成卡密（每张随机额度）
# ============================================================
echo ""
echo "=== 步骤 1: NewAPI 生成卡密 (随机额度 act 版) ==="
GENERATED_KEYS=$(cd "$ACT_DIR" && node -e "
import { generateCardsActivityRandom } from './scripts/api-act.mjs';
const result = await generateCardsActivityRandom('$CARD_NAME', $RESTOCK_COUNT, $MIN_DOLLARS, $MAX_DOLLARS, '每日特惠补货');
console.log(JSON.stringify(result, null, 2));
" 2>&1)

if ! echo "$GENERATED_KEYS" | grep -q '"success": true'; then
    echo "ERROR: 生成卡密失败"
    echo "$GENERATED_KEYS"
    exit 1
fi

# 提取生成的 keys
KEYS_JSON=$(echo "$GENERATED_KEYS" | jq -r '.keys')
GENERATED_COUNT=$(echo "$KEYS_JSON" | jq -r 'length')
echo "生成成功: $GENERATED_COUNT 张"

# 显示额度分布
echo "额度分布:"
echo "$GENERATED_KEYS" | jq -r '.details[] | "  $\(.dollars) → \(.key[0:20])..."'

# ============================================================
# 第二部分：保存留底
# ============================================================
echo ""
echo "=== 步骤 2: 保存留底 ==="
BACKUP_FILE="$BACKUP_DIR/daily-act-restock_${TIMESTAMP}.json"
echo "$GENERATED_KEYS" > "$BACKUP_FILE"
echo "已保存到: $BACKUP_FILE"

# 同时保存纯文本格式（方便查看）
BACKUP_TXT="$BACKUP_DIR/daily-act-restock_${TIMESTAMP}.txt"
echo "$KEYS_JSON" | jq -r '.[]' > "$BACKUP_TXT"
echo "纯文本备份: $BACKUP_TXT"

# ============================================================
# 第三部分：上传到 Shop
# ============================================================
echo ""
echo "=== 步骤 3: 上传到 Shop ==="
UPLOAD_RESULT=$(cd "$ACT_DIR" && node -e "
import { uploadCards } from './scripts/api-act.mjs';
const keys = $KEYS_JSON;
const result = await uploadCards('$CARD_NAME', keys, '每日特惠补货');
console.log(JSON.stringify(result, null, 2));
" 2>&1)

if ! echo "$UPLOAD_RESULT" | grep -q '"success": true'; then
    echo "ERROR: 上传失败"
    echo "$UPLOAD_RESULT"
    exit 1
fi

UPLOADED_COUNT=$(echo "$UPLOAD_RESULT" | jq -r '.uploaded')
echo "上传成功: $UPLOADED_COUNT 张"

# ============================================================
# 推送到 Discord
# ============================================================
echo ""
echo "=== 步骤 4: 推送到 Discord ==="
NOTE="• $CARD_NAME  +$UPLOADED_COUNT 张 (\$$MIN_DOLLARS~\$$MAX_DOLLARS 随机额度)"
python3 "$PUSH_SCRIPT" --note "$NOTE"

echo ""
echo "✅ 全部完成！"
echo "   Shop 原库存: $SHOP_STOCK 张"
echo "   生成: $GENERATED_COUNT 张 (随机额度 \$$MIN_DOLLARS~\$$MAX_DOLLARS)"
echo "   备份: $BACKUP_FILE"
echo "   上传: $UPLOADED_COUNT 张"
echo "   Shop 新库存: $((SHOP_STOCK + UPLOADED_COUNT)) 张"
