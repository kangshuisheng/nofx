#!/bin/sh
# init-db.sh - 初始化數據庫文件
# 確保 config.db 是文件而非目錄

set -e

DB_FILE="./config.db"

echo "🔍 檢查數據庫文件..."

# 檢查 config.db 是否存在
if [ ! -e "$DB_FILE" ]; then
    echo "📝 創建空的數據庫文件: $DB_FILE"
    touch "$DB_FILE"
    echo "✅ 數據庫文件已創建"
elif [ -d "$DB_FILE" ]; then
    echo "❌ 錯誤：$DB_FILE 是目錄而非文件！"
    echo "🔧 修復：備份並重新創建為文件..."

    # 備份目錄
    BACKUP_DIR="${DB_FILE}.broken_$(date +%Y%m%d_%H%M%S)"
    mv "$DB_FILE" "$BACKUP_DIR"
    echo "📦 已備份到: $BACKUP_DIR"

    # 創建文件
    touch "$DB_FILE"
    echo "✅ 數據庫文件已重新創建"
elif [ -f "$DB_FILE" ]; then
    echo "✅ 數據庫文件存在: $DB_FILE ($(du -h "$DB_FILE" | cut -f1))"
else
    echo "⚠️  警告：$DB_FILE 存在但類型未知"
fi

echo "✅ 數據庫文件檢查完成"
