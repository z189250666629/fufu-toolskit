#!/bin/bash
# 混合月卡补货自动化脚本（Activity 版 — 命名规则: {额度}-act-{xxxx}）

set -e

ACT_DIR="/root/clawd/fufu_act"
SHOP_DIR="/root/clawd/skills/fufu-shop"
PUSH_SCRIPT="/root/clawd/scripts/discord-push-fufu.py"
TARGET_STOCK=200
BACKUP_DIR="/root/clawd/fufu_act/backups"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)

# 5 种月卡
CARD_TYPES=(
  "混合卡 月一百次卡"
  "混合卡 月一百五十次卡"
  "混合卡 月三百次卡"
  "混合卡 月五百次卡"
  "混合卡 月一千次卡"
)

# 确保备份目录存在
mkdir -p "$BACKUP_DIR"

# 记录补货信息
RESTOCK_LINES=()
TOTAL_GENERATED=0
TOTAL_UPLOADED=0

echo "=== 混合月卡补货任务 ==="
echo "目标库存: 每种 $TARGET_STOCK 张"
echo ""

# ============================================================
# 步骤 0: 查询所有月卡的库存
# ============================================================
echo "=== 步骤 0: 查询库存 ==="
STOCK_OUTPUT=$(cd "$SHOP_DIR" && node scripts/fufu-shop.mjs --stock 2>&1)

# 遍历每种月卡
for CARD_NAME in "${CARD_TYPES[@]}"; do
  echo ""
  echo "--- 处理: $CARD_NAME ---"
  
  # 提取当前库存
  CURRENT_STOCK=$(echo "$STOCK_OUTPUT" | grep "$CARD_NAME" | grep -oP '\d+(?= 张)')
  
  if [ -z "$CURRENT_STOCK" ]; then
    echo "⚠️  无法获取库存，跳过"
    continue
  fi
  
  echo "当前库存: $CURRENT_STOCK 张"
  
  # 判断是否需要补货
  if [ "$CURRENT_STOCK" -ge "$TARGET_STOCK" ]; then
    echo "✅ 库存充足，跳过"
    continue
  fi
  
  RESTOCK_COUNT=$((TARGET_STOCK - CURRENT_STOCK))
  echo "需要补货: $RESTOCK_COUNT 张"
  
  # ============================================================
  # 第一部分：NewAPI 生成卡密
  # ============================================================
  echo "步骤 1: 生成卡密 (act 命名)..."
  GENERATED_KEYS=$(cd "$ACT_DIR" && node -e "
import { generateCardsActivity } from './scripts/api-act.mjs';
const result = await generateCardsActivity('$CARD_NAME', $RESTOCK_COUNT, '月卡补货');
console.log(JSON.stringify(result, null, 2));
" 2>&1)
  
  if ! echo "$GENERATED_KEYS" | grep -q '"success": true'; then
    echo "❌ 生成失败，跳过"
    echo "$GENERATED_KEYS"
    continue
  fi
  
  KEYS_JSON=$(echo "$GENERATED_KEYS" | jq -r '.keys')
  GENERATED_COUNT=$(echo "$KEYS_JSON" | jq -r 'length')
  echo "生成成功: $GENERATED_COUNT 张"
  
  # ============================================================
  # 第二部分：保存留底
  # ============================================================
  echo "步骤 2: 保存留底..."
  # 使用卡类型名称作为文件名的一部分（去掉空格）
  CARD_NAME_SAFE=$(echo "$CARD_NAME" | tr ' ' '_')
  BACKUP_FILE="$BACKUP_DIR/monthly-act-restock_${CARD_NAME_SAFE}_${TIMESTAMP}.json"
  echo "$GENERATED_KEYS" > "$BACKUP_FILE"
  
  BACKUP_TXT="$BACKUP_DIR/monthly-act-restock_${CARD_NAME_SAFE}_${TIMESTAMP}.txt"
  echo "$KEYS_JSON" | jq -r '.[]' > "$BACKUP_TXT"
  echo "已保存: $BACKUP_FILE"
  
  # ============================================================
  # 第三部分：上传到 Shop
  # ============================================================
  echo "步骤 3: 上传到 Shop..."
  UPLOAD_RESULT=$(cd "$ACT_DIR" && node -e "
import { uploadCards } from './scripts/api-act.mjs';
const keys = $KEYS_JSON;
const result = await uploadCards('$CARD_NAME', keys, '月卡补货');
console.log(JSON.stringify(result, null, 2));
" 2>&1)
  
  if ! echo "$UPLOAD_RESULT" | grep -q '"success": true'; then
    echo "❌ 上传失败"
    echo "$UPLOAD_RESULT"
    continue
  fi
  
  UPLOADED_COUNT=$(echo "$UPLOAD_RESULT" | jq -r '.uploaded')
  echo "上传成功: $UPLOADED_COUNT 张"
  
  # 记录补货信息
  RESTOCK_LINES+=("• $CARD_NAME  +$UPLOADED_COUNT 张")
  TOTAL_GENERATED=$((TOTAL_GENERATED + GENERATED_COUNT))
  TOTAL_UPLOADED=$((TOTAL_UPLOADED + UPLOADED_COUNT))
done

# ============================================================
# 推送到 Discord
# ============================================================
echo ""
echo "=== 步骤 4: 推送到 Discord ==="

if [ ${#RESTOCK_LINES[@]} -gt 0 ]; then
  # 有补货记录，拼接成多行
  RESTOCK_NOTE=$(printf "%s\n" "${RESTOCK_LINES[@]}")
  python3 "$PUSH_SCRIPT" --note "$RESTOCK_NOTE"
  
  echo ""
  echo "✅ 补货完成！"
  echo "   补货种类: ${#RESTOCK_LINES[@]}"
  echo "   总生成: $TOTAL_GENERATED 张"
  echo "   总上传: $TOTAL_UPLOADED 张"
else
  # 没有补货，只推送当前库存
  python3 "$PUSH_SCRIPT"
  
  echo ""
  echo "✅ 所有月卡库存充足，无需补货"
fi
