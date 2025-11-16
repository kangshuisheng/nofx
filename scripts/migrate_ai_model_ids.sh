#!/bin/bash

# 遷移舊格式的 AI 模型 ID（user123_deepseek → deepseek）
# 問題：舊版本代碼生成了 "userID_provider" 格式的 model_id
# 修復：將其遷移為純 provider 格式，與前端保持一致
# 作者：Claude Code
# 日期：2025-11-15

set -e

echo "🔄 遷移 AI 模型 ID 格式"
echo "========================================="
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
echo "1️⃣ 診斷舊格式的 model_id"
echo "=========================="
echo ""

echo "📊 當前 ai_models 表數據："
sqlite3 "$DB_FILE" <<EOF
.mode column
.headers on
SELECT model_id, user_id, provider, enabled FROM ai_models ORDER BY model_id;
EOF
echo ""

echo "🔍 檢查舊格式的 model_id（包含下劃線的）："
OLD_FORMAT_COUNT=$(sqlite3 "$DB_FILE" "
SELECT COUNT(*)
FROM ai_models
WHERE model_id LIKE '%\_%'
  AND model_id NOT LIKE 'gpt-%'
  AND model_id NOT LIKE 'claude-%';
")

if [ "$OLD_FORMAT_COUNT" -gt 0 ]; then
    echo "⚠️  發現 $OLD_FORMAT_COUNT 個舊格式的 model_id！"
    echo ""
    echo "詳細信息："
    sqlite3 "$DB_FILE" <<EOF
.mode column
.headers on
SELECT model_id, user_id, provider, enabled
FROM ai_models
WHERE model_id LIKE '%\_%'
  AND model_id NOT LIKE 'gpt-%'
  AND model_id NOT LIKE 'claude-%'
ORDER BY model_id;
EOF
    echo ""
else
    echo "✅ 沒有舊格式的 model_id"
    echo ""
    echo "✅ 數據庫狀態正常，無需遷移！"
    exit 0
fi

# 2️⃣ 遷移階段
echo ""
echo "2️⃣ 開始遷移"
echo "============="
echo ""

echo "📋 遷移策略："
echo "   - 將 'userID_provider' 格式改為純 'provider' 格式"
echo "   - 例如: 'user123_deepseek' → 'deepseek'"
echo "   - 保留 'gpt-4', 'claude-3' 等已包含連字符的模型名稱"
echo ""

# 執行遷移
sqlite3 "$DB_FILE" <<EOF
BEGIN TRANSACTION;

-- 更新所有 "userID_provider" 格式的 model_id
-- 提取下劃線後的部分作為新的 model_id
UPDATE ai_models
SET model_id = SUBSTR(model_id, INSTR(model_id, '_') + 1)
WHERE model_id LIKE '%\_%'
  AND model_id NOT LIKE 'gpt-%'
  AND model_id NOT LIKE 'claude-%'
  AND INSTR(model_id, '_') > 0;

COMMIT;
EOF

if [ $? -eq 0 ]; then
    echo "✅ 成功遷移 model_id 格式"
    echo ""

    # 驗證遷移結果
    echo ""
    echo "3️⃣ 驗證遷移結果"
    echo "================"
    echo ""

    REMAINING_OLD_FORMAT=$(sqlite3 "$DB_FILE" "
    SELECT COUNT(*)
    FROM ai_models
    WHERE model_id LIKE '%\_%'
      AND model_id NOT LIKE 'gpt-%'
      AND model_id NOT LIKE 'claude-%';
    ")

    if [ "$REMAINING_OLD_FORMAT" -eq 0 ]; then
        echo "✅ 遷移成功！沒有舊格式的 model_id 了"
    else
        echo "⚠️  仍有 $REMAINING_OLD_FORMAT 個舊格式的 model_id，請檢查"
    fi

    echo ""
    echo "📊 遷移後的統計："
    sqlite3 "$DB_FILE" <<EOF
.mode column
.headers on
SELECT
    (SELECT COUNT(*) FROM ai_models) as total_models,
    (SELECT COUNT(DISTINCT provider) FROM ai_models) as unique_providers,
    (SELECT COUNT(*) FROM ai_models WHERE enabled=1) as enabled_models;
EOF

    echo ""
    echo "📊 遷移後的 ai_models 表："
    sqlite3 "$DB_FILE" <<EOF
.mode column
.headers on
SELECT model_id, user_id, provider, enabled FROM ai_models ORDER BY model_id;
EOF

    echo ""
    echo "💾 備份文件保存在: $BACKUP_FILE"
    echo ""
    echo "✅ 遷移完成！建議重啟應用："
    echo "   docker-compose restart"

else
    echo "❌ 遷移失敗，恢復備份..."
    cp "$BACKUP_FILE" "$DB_FILE"
    echo "✅ 已恢復備份"
    exit 1
fi
