#!/bin/bash
# NOFX 启动脚本（自动加载环境变量）

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
ENV_FILE="$PROJECT_ROOT/.env"

# 检查 .env 文件是否存在
if [ ! -f "$ENV_FILE" ]; then
    echo "❌ .env 文件不存在"
    echo "请先运行: ./scripts/setup-env.sh"
    exit 1
fi

# 加载环境变量
echo "🔐 加载环境变量..."
export $(grep -v '^#' "$ENV_FILE" | grep -v '^$' | xargs)

# 验证必需的环境变量
if [ -z "$DATA_ENCRYPTION_KEY" ]; then
    echo "❌ DATA_ENCRYPTION_KEY 未设置"
    exit 1
fi

echo "✅ 环境变量已加载"
echo ""

# 启动应用
cd "$PROJECT_ROOT"

if [ "$1" == "backend" ]; then
    echo "🚀 启动后端..."
    go run main.go
elif [ "$1" == "frontend" ]; then
    echo "🚀 启动前端..."
    cd web
    npm run dev
else
    echo "🚀 启动完整应用..."
    go run main.go
fi
