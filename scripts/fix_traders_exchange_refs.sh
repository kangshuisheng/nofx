#!/bin/bash

set -e

DB_FILE="config.db"

echo "🔍 開始修復 traders 表的 exchange_id 外鍵引用..."
echo ""

# 備份數據庫
BACKUP_FILE="${DB_FILE}.backup_traders_$(date +%Y%m%d_%H%M%S)"
echo "💾 備份數據庫到 $BACKUP_FILE ..."
cp "$DB_FILE" "$BACKUP_FILE"
echo "✅ 備份完成"
echo ""

# 檢查當前 traders 表的 exchange_id 引用
echo "🔍 當前 traders 表的 exchange_id 引用："
sqlite3 "$DB_FILE" "SELECT id, name, exchange_id FROM traders;" | head -10
echo ""

# 檢查 exchanges 表的實際 ID
echo "🔍 當前 exchanges 表的 ID 分配："
sqlite3 "$DB_FILE" "SELECT id, exchange_id, name, user_id FROM exchanges;"
echo ""

echo "⚠️  開始修復外鍵引用..."
echo ""

# 執行修復
sqlite3 "$DB_FILE" <<'EOF'
BEGIN TRANSACTION;

-- 修復策略：
-- 1. 找出舊版 traders.exchange_id 的值（例如 0 或文本 ID）
-- 2. 根據 exchanges.exchange_id 和 user_id 匹配，找到新的 exchanges.id
-- 3. 更新 traders.exchange_id 為新的整數 ID

-- 先檢查需要修復的記錄數
SELECT '需要修復的 traders 記錄數: ' || COUNT(*)
FROM traders t
LEFT JOIN exchanges e ON t.exchange_id = e.id AND t.user_id = e.user_id
WHERE e.id IS NULL;

-- 情況1: traders.exchange_id = 0（舊版系統可能使用的默認值）
-- 需要根據 trader ID 命名規則推斷交易所
-- 例如: "binance_userID_modelID" -> exchange_id = 'binance'

UPDATE traders
SET exchange_id = (
    SELECT e.id
    FROM exchanges e
    WHERE e.user_id = traders.user_id
    AND e.exchange_id = CASE
        -- 從 trader ID 提取交易所名稱（格式: exchangeName_userId_modelId）
        WHEN traders.id LIKE 'binance%' THEN 'binance'
        WHEN traders.id LIKE 'hyperliquid%' THEN 'hyperliquid'
        WHEN traders.id LIKE 'aster%' THEN 'aster'
        ELSE NULL
    END
    LIMIT 1
)
WHERE traders.exchange_id = 0
OR traders.exchange_id NOT IN (SELECT id FROM exchanges WHERE exchanges.user_id = traders.user_id);

COMMIT;
EOF

if [ $? -eq 0 ]; then
    echo ""
    echo "✅ 修復成功！"
    echo ""
    echo "🔍 修復後的 traders 表："
    sqlite3 "$DB_FILE" "SELECT t.id, t.name, t.exchange_id, e.exchange_id as exchange_name, e.name as exchange_display_name FROM traders t JOIN exchanges e ON t.exchange_id = e.id AND t.user_id = e.user_id;" 2>/dev/null || {
        echo "⚠️  無法顯示關聯數據，檢查原始數據："
        sqlite3 "$DB_FILE" "SELECT id, name, exchange_id FROM traders;"
    }
    echo ""
    echo "📊 數據統計："
    echo "  - Traders 總數: $(sqlite3 "$DB_FILE" "SELECT COUNT(*) FROM traders;")"
    echo "  - 有效外鍵引用: $(sqlite3 "$DB_FILE" "SELECT COUNT(*) FROM traders t JOIN exchanges e ON t.exchange_id = e.id AND t.user_id = e.user_id;")"
    echo "  - 無效外鍵引用: $(sqlite3 "$DB_FILE" "SELECT COUNT(*) FROM traders t LEFT JOIN exchanges e ON t.exchange_id = e.id AND t.user_id = e.user_id WHERE e.id IS NULL;")"
else
    echo ""
    echo "❌ 修復失敗！正在恢復備份..."
    cp "$BACKUP_FILE" "$DB_FILE"
    echo "✅ 已恢復備份"
    exit 1
fi

echo ""
echo "✅ 修復完成！"
