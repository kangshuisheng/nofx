#!/bin/bash
#
# 修復 traders 表的數據庫遷移問題
# 問題：遷移失敗導致同時存在舊列（ai_model_id_old, exchange_id_old）和新列（ai_model_id, exchange_id）
# 解決：刪除舊列，保留新列，遷移數據
#

set -e

DB_FILE="${1:-config.db}"

if [ ! -f "$DB_FILE" ]; then
    echo "❌ 錯誤：找不到數據庫文件 $DB_FILE"
    exit 1
fi

echo "📋 開始修復 traders 表的遷移問題..."
echo "📂 數據庫文件: $DB_FILE"

# 備份數據庫
BACKUP_FILE="${DB_FILE}.backup_$(date +%Y%m%d_%H%M%S)"
echo "💾 備份數據庫到 $BACKUP_FILE ..."
cp "$DB_FILE" "$BACKUP_FILE"

# 檢查是否存在問題
HAS_OLD_COLUMNS=$(sqlite3 "$DB_FILE" "PRAGMA table_info(traders);" | grep -c "_old" || true)

if [ "$HAS_OLD_COLUMNS" -eq 0 ]; then
    echo "✅ traders 表已經是正確的結構，無需修復"
    exit 0
fi

echo "⚠️  發現 $HAS_OLD_COLUMNS 個舊列，需要修復"

# 執行修復 SQL
sqlite3 "$DB_FILE" <<'EOF'
BEGIN TRANSACTION;

-- 創建新表（沒有舊列）
CREATE TABLE traders_new (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL DEFAULT 'default',
    name TEXT NOT NULL,
    ai_model_id INTEGER,
    exchange_id INTEGER,
    initial_balance REAL NOT NULL,
    scan_interval_minutes INTEGER DEFAULT 3,
    is_running BOOLEAN DEFAULT 0,
    btc_eth_leverage INTEGER DEFAULT 5,
    altcoin_leverage INTEGER DEFAULT 5,
    trading_symbols TEXT DEFAULT '',
    use_coin_pool BOOLEAN DEFAULT 0,
    use_oi_top BOOLEAN DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    custom_prompt TEXT DEFAULT '',
    override_base_prompt BOOLEAN DEFAULT 0,
    is_cross_margin BOOLEAN DEFAULT 1,
    use_default_coins BOOLEAN DEFAULT 1,
    custom_coins TEXT DEFAULT '',
    system_prompt_template TEXT DEFAULT 'default',
    use_inside_coins BOOLEAN DEFAULT 0,
    taker_fee_rate REAL DEFAULT 0.0004,
    maker_fee_rate REAL DEFAULT 0.0002,
    timeframes TEXT DEFAULT '4h',
    order_strategy TEXT DEFAULT 'conservative_hybrid',
    limit_price_offset REAL DEFAULT -0.03,
    limit_timeout_seconds INTEGER DEFAULT 60
);

-- 複製數據（自動映射舊 ID 到新 ID）
INSERT INTO traders_new
SELECT
    id,
    user_id,
    name,
    CASE
        WHEN ai_model_id IS NOT NULL THEN ai_model_id
        ELSE (
            SELECT id FROM ai_models
            WHERE model_id = ai_model_id_old AND ai_models.user_id = traders.user_id
            LIMIT 1
        )
    END as ai_model_id,
    CASE
        WHEN exchange_id IS NOT NULL THEN exchange_id
        ELSE (
            SELECT id FROM exchanges
            WHERE exchange_id = exchange_id_old AND exchanges.user_id = traders.user_id
            LIMIT 1
        )
    END as exchange_id,
    initial_balance,
    scan_interval_minutes,
    is_running,
    btc_eth_leverage,
    altcoin_leverage,
    trading_symbols,
    use_coin_pool,
    use_oi_top,
    created_at,
    updated_at,
    custom_prompt,
    override_base_prompt,
    is_cross_margin,
    use_default_coins,
    custom_coins,
    system_prompt_template,
    use_inside_coins,
    taker_fee_rate,
    maker_fee_rate,
    timeframes,
    order_strategy,
    limit_price_offset,
    limit_timeout_seconds
FROM traders;

-- 刪除舊表
DROP TABLE traders;

-- 重命名新表
ALTER TABLE traders_new RENAME TO traders;

COMMIT;
EOF

if [ $? -eq 0 ]; then
    echo "✅ traders 表修復成功"

    # 清理 WAL 緩存
    echo "🧹 清理 WAL 緩存..."
    sqlite3 "$DB_FILE" "PRAGMA wal_checkpoint(FULL); VACUUM;"
    rm -f "${DB_FILE}-wal" "${DB_FILE}-shm"

    # 驗證結果
    echo ""
    echo "📊 修復後的統計："
    sqlite3 "$DB_FILE" "SELECT COUNT(*) as total_traders FROM traders;"

    echo ""
    echo "✅ 修復完成！備份文件保存在: $BACKUP_FILE"
else
    echo "❌ 修復失敗，恢復備份..."
    cp "$BACKUP_FILE" "$DB_FILE"
    exit 1
fi
