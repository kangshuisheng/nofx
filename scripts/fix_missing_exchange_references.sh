#!/bin/bash

# 修復「交易所不存在」錯誤的診斷和修復腳本
# 問題：traders.exchange_id 引用了不存在的 exchanges.id
# 作者：Claude Code
# 日期：2025-11-15

set -e

echo "🔍 診斷和修復「交易所不存在」錯誤"
echo "========================================"
echo ""

# 檢測數據庫文件位置
if [ -f "/data/nofx.db" ]; then
    DB_FILE="/data/nofx.db"
    echo "📁 數據庫位置: $DB_FILE (Docker 容器內部)"
elif [ -f "./data/nofx.db" ]; then
    DB_FILE="./data/nofx.db"
    echo "📁 數據庫位置: $DB_FILE (本地)"
else
    echo "❌ 錯誤：找不到數據庫文件 nofx.db"
    echo "   請確保："
    echo "   1. Docker 容器正在運行（從容器內執行此腳本）"
    echo "   2. 或者從項目根目錄執行（使用本地數據庫）"
    exit 1
fi

# 創建備份
BACKUP_FILE="${DB_FILE}.backup.$(date +%Y%m%d_%H%M%S)"
echo "💾 創建備份: $BACKUP_FILE"
cp "$DB_FILE" "$BACKUP_FILE"
echo ""

# 1️⃣ 診斷階段
echo "1️⃣ 診斷數據庫狀態"
echo "===================="
echo ""

echo "📊 當前 exchanges 表數據："
sqlite3 "$DB_FILE" <<EOF
.mode column
.headers on
SELECT id, exchange_id, name, enabled, user_id FROM exchanges ORDER BY id;
EOF
echo ""

echo "📊 traders 表中引用的 exchange_id："
sqlite3 "$DB_FILE" <<EOF
.mode column
.headers on
SELECT DISTINCT exchange_id, COUNT(*) as trader_count
FROM traders
GROUP BY exchange_id
ORDER BY exchange_id;
EOF
echo ""

echo "🔍 檢查孤立的 trader 記錄（引用不存在的 exchange_id）："
ORPHANED=$(sqlite3 "$DB_FILE" "
SELECT COUNT(*)
FROM traders t
WHERE NOT EXISTS (
    SELECT 1 FROM exchanges e WHERE e.id = t.exchange_id
);
")

if [ "$ORPHANED" -gt 0 ]; then
    echo "⚠️  發現 $ORPHANED 個孤立的 trader 記錄！"
    echo ""
    echo "詳細信息："
    sqlite3 "$DB_FILE" <<EOF
.mode column
.headers on
SELECT t.id, t.name, t.exchange_id as missing_exchange_id, t.user_id
FROM traders t
WHERE NOT EXISTS (
    SELECT 1 FROM exchanges e WHERE e.id = t.exchange_id
)
ORDER BY t.exchange_id;
EOF
    echo ""
else
    echo "✅ 沒有孤立的 trader 記錄"
    echo ""
    echo "✅ 數據庫狀態正常，無需修復！"
    exit 0
fi

# 2️⃣ 修復階段
echo ""
echo "2️⃣ 開始修復"
echo "============"
echo ""

echo "選擇修復策略："
echo ""
echo "方案 A：刪除孤立的 trader 記錄（推薦 - 安全且簡單）"
echo "        影響：刪除 $ORPHANED 個無效的交易員配置"
echo ""
echo "方案 B：嘗試修復外鍵引用（複雜 - 需要分析數據）"
echo "        適用：如果您知道正確的 exchange_id 映射關係"
echo ""

# 自動選擇方案 A（最安全）
echo "自動執行方案 A：刪除孤立記錄"
echo ""

# 顯示將要刪除的記錄
echo "📋 將要刪除的 traders："
sqlite3 "$DB_FILE" <<EOF
.mode list
SELECT '  - ID: ' || t.id || ', 名稱: ' || t.name || ', exchange_id: ' || t.exchange_id
FROM traders t
WHERE NOT EXISTS (
    SELECT 1 FROM exchanges e WHERE e.id = t.exchange_id
);
EOF
echo ""

# 執行刪除
sqlite3 "$DB_FILE" <<EOF
BEGIN TRANSACTION;

DELETE FROM traders
WHERE id IN (
    SELECT t.id
    FROM traders t
    WHERE NOT EXISTS (
        SELECT 1 FROM exchanges e WHERE e.id = t.exchange_id
    )
);

COMMIT;
EOF

if [ $? -eq 0 ]; then
    echo "✅ 成功刪除孤立的 trader 記錄"
    echo ""

    # 清理 WAL 緩存
    echo "🧹 清理 WAL 緩存..."
    sqlite3 "$DB_FILE" "PRAGMA wal_checkpoint(FULL); VACUUM;"
    rm -f "${DB_FILE}-wal" "${DB_FILE}-shm" 2>/dev/null || true

    # 驗證修復結果
    echo ""
    echo "3️⃣ 驗證修復結果"
    echo "================"
    echo ""

    REMAINING_ORPHANED=$(sqlite3 "$DB_FILE" "
    SELECT COUNT(*)
    FROM traders t
    WHERE NOT EXISTS (
        SELECT 1 FROM exchanges e WHERE e.id = t.exchange_id
    );
    ")

    if [ "$REMAINING_ORPHANED" -eq 0 ]; then
        echo "✅ 修復成功！沒有孤立記錄了"
    else
        echo "⚠️  仍有 $REMAINING_ORPHANED 個孤立記錄，請檢查"
    fi

    echo ""
    echo "📊 修復後的統計："
    sqlite3 "$DB_FILE" <<EOF
.mode column
.headers on
SELECT
    (SELECT COUNT(*) FROM exchanges WHERE enabled=1) as active_exchanges,
    (SELECT COUNT(*) FROM traders) as total_traders,
    (SELECT COUNT(*) FROM traders WHERE is_running=1) as running_traders;
EOF

    echo ""
    echo "💾 備份文件保存在: $BACKUP_FILE"
    echo ""
    echo "✅ 修復完成！請重啟 Docker 容器："
    echo "   docker-compose restart"

else
    echo "❌ 修復失敗，恢復備份..."
    cp "$BACKUP_FILE" "$DB_FILE"
    echo "✅ 已恢復備份"
    exit 1
fi
