#!/bin/bash
# NOFX 安全检查脚本
# 检查密钥配置、文件权限、敏感文件泄露等问题

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

echo "🔍 NOFX 安全检查"
echo "================"
echo ""

WARNINGS=0
ERRORS=0

# 1. 检查 .secrets 目录
echo "📁 检查 .secrets 目录..."
if [ ! -d "$PROJECT_ROOT/.secrets" ]; then
    echo "   ⚠️  .secrets 目录不存在"
    WARNINGS=$((WARNINGS + 1))
else
    PERMS=$(stat -f "%OLp" "$PROJECT_ROOT/.secrets" 2>/dev/null || stat -c "%a" "$PROJECT_ROOT/.secrets" 2>/dev/null)
    if [ "$PERMS" != "700" ]; then
        echo "   ❌ .secrets 目录权限不正确：$PERMS（应为 700）"
        ERRORS=$((ERRORS + 1))
    else
        echo "   ✅ .secrets 目录权限正确"
    fi
fi

# 2. 检查 .env 文件
echo ""
echo "📄 检查 .env 文件..."
if [ ! -f "$PROJECT_ROOT/.env" ]; then
    echo "   ⚠️  .env 文件不存在"
    WARNINGS=$((WARNINGS + 1))
else
    # 检查权限
    PERMS=$(stat -f "%OLp" "$PROJECT_ROOT/.env" 2>/dev/null || stat -c "%a" "$PROJECT_ROOT/.env" 2>/dev/null)
    if [ "$PERMS" != "600" ] && [ "$PERMS" != "644" ]; then
        echo "   ⚠️  .env 文件权限：$PERMS（建议 600）"
        WARNINGS=$((WARNINGS + 1))
    else
        echo "   ✅ .env 文件权限正确"
    fi

    # 检查密钥配置
    if grep -q "DATA_ENCRYPTION_KEY=PLEASE_GENERATE" "$PROJECT_ROOT/.env" 2>/dev/null; then
        echo "   ❌ DATA_ENCRYPTION_KEY 未配置！"
        ERRORS=$((ERRORS + 1))
    elif grep -q "DATA_ENCRYPTION_KEY=" "$PROJECT_ROOT/.env" 2>/dev/null; then
        KEY_VALUE=$(grep "^DATA_ENCRYPTION_KEY=" "$PROJECT_ROOT/.env" | cut -d'=' -f2)
        if [ ${#KEY_VALUE} -lt 32 ]; then
            echo "   ⚠️  DATA_ENCRYPTION_KEY 长度不足（当前：${#KEY_VALUE}，建议：44+）"
            WARNINGS=$((WARNINGS + 1))
        else
            echo "   ✅ DATA_ENCRYPTION_KEY 已配置"
        fi
    fi
fi

# 3. 检查数据库文件权限
echo ""
echo "💾 检查数据库文件..."
DB_FILES=$(find "$PROJECT_ROOT" -maxdepth 1 -name "*.db" 2>/dev/null)
if [ -z "$DB_FILES" ]; then
    echo "   ℹ️  未找到数据库文件"
else
    for DB in $DB_FILES; do
        PERMS=$(stat -f "%OLp" "$DB" 2>/dev/null || stat -c "%a" "$DB" 2>/dev/null)
        if [ "$PERMS" != "600" ]; then
            echo "   ⚠️  $(basename $DB) 权限：$PERMS（建议 600）"
            WARNINGS=$((WARNINGS + 1))
        else
            echo "   ✅ $(basename $DB) 权限正确"
        fi
    done
fi

# 4. 检查敏感文件是否被 Git 追踪
echo ""
echo "🔐 检查敏感文件泄露..."
cd "$PROJECT_ROOT"

TRACKED_SECRETS=$(git ls-files | grep -E '\.env$|\.secrets/|\.key$|\.pem$|DATA_ENCRYPTION_KEY' 2>/dev/null || true)
if [ -n "$TRACKED_SECRETS" ]; then
    echo "   ❌ 以下敏感文件被 Git 追踪："
    echo "$TRACKED_SECRETS" | while read file; do
        echo "      - $file"
    done
    ERRORS=$((ERRORS + 1))
else
    echo "   ✅ 未发现敏感文件被 Git 追踪"
fi

# 5. 检查 .gitignore
echo ""
echo "📝 检查 .gitignore..."
if [ ! -f "$PROJECT_ROOT/.gitignore" ]; then
    echo "   ❌ .gitignore 文件不存在！"
    ERRORS=$((ERRORS + 1))
else
    REQUIRED=(".env" ".secrets/" "*.key" "*.db")
    for PATTERN in "${REQUIRED[@]}"; do
        if grep -q "$PATTERN" "$PROJECT_ROOT/.gitignore"; then
            echo "   ✅ 已忽略：$PATTERN"
        else
            echo "   ⚠️  未忽略：$PATTERN"
            WARNINGS=$((WARNINGS + 1))
        fi
    done
fi

# 6. 检查环境变量
echo ""
echo "🌍 检查环境变量..."
if [ -z "$DATA_ENCRYPTION_KEY" ]; then
    echo "   ⚠️  DATA_ENCRYPTION_KEY 环境变量未设置"
    echo "      （运行时需要加载 .env 文件）"
    WARNINGS=$((WARNINGS + 1))
else
    echo "   ✅ DATA_ENCRYPTION_KEY 已设置"
fi

# 7. 总结
echo ""
echo "================"
echo "📊 检查结果"
echo "================"
echo "❌ 错误：$ERRORS"
echo "⚠️  警告：$WARNINGS"
echo ""

if [ $ERRORS -gt 0 ]; then
    echo "🚨 发现 $ERRORS 个严重错误，请立即修复！"
    exit 1
elif [ $WARNINGS -gt 0 ]; then
    echo "⚠️  发现 $WARNINGS 个警告，建议修复"
    exit 0
else
    echo "✅ 所有检查通过！"
    exit 0
fi
