#!/bin/bash
# NOFX 环境配置脚本
# 用途：生成加密密钥并配置环境变量

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
SECRETS_DIR="$PROJECT_ROOT/.secrets"
ENV_FILE="$PROJECT_ROOT/.env"

echo "🔐 NOFX 加密密钥配置"
echo "===================="

# 1. 创建 .secrets 目录
if [ ! -d "$SECRETS_DIR" ]; then
    echo "📁 创建 .secrets 目录..."
    mkdir -p "$SECRETS_DIR"
    chmod 700 "$SECRETS_DIR"
fi

# 2. 检查是否已有 .env 文件
if [ -f "$ENV_FILE" ]; then
    echo "⚠️  .env 文件已存在"
    read -p "是否覆盖? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "❌ 已取消操作"
        exit 0
    fi
fi

# 3. 复制 .env.example 到 .env
echo "📄 创建 .env 文件..."
cp "$PROJECT_ROOT/.env.example" "$ENV_FILE"

# 4. 生成 DATA_ENCRYPTION_KEY
echo "🔑 生成 DATA_ENCRYPTION_KEY..."
DATA_KEY=$(openssl rand -base64 32)
echo "$DATA_KEY" > "$SECRETS_DIR/master.key"
chmod 600 "$SECRETS_DIR/master.key"

# 5. 更新 .env 文件
echo "✏️  更新 .env 配置..."
if [[ "$OSTYPE" == "darwin"* ]]; then
    # macOS
    sed -i '' "s|DATA_ENCRYPTION_KEY=.*|DATA_ENCRYPTION_KEY=$DATA_KEY|" "$ENV_FILE"
else
    # Linux
    sed -i "s|DATA_ENCRYPTION_KEY=.*|DATA_ENCRYPTION_KEY=$DATA_KEY|" "$ENV_FILE"
fi

# 6. 生成 JWT_SECRET（可选）
echo "🔑 生成 JWT_SECRET..."
JWT_KEY=$(openssl rand -base64 64)
if [[ "$OSTYPE" == "darwin"* ]]; then
    sed -i '' "s|# JWT_SECRET=|JWT_SECRET=$JWT_KEY|" "$ENV_FILE"
else
    sed -i "s|# JWT_SECRET=|JWT_SECRET=$JWT_KEY|" "$ENV_FILE"
fi

echo ""
echo "✅ 配置完成！"
echo ""
echo "📋 生成的文件："
echo "   - $SECRETS_DIR/master.key"
echo "   - $ENV_FILE"
echo ""
echo "🚀 启动方式："
echo "   方式1（手动加载）："
echo "     export \$(grep -v '^#' .env | xargs)"
echo "     go run main.go"
echo ""
echo "   方式2（使用脚本）："
echo "     ./scripts/run-with-env.sh"
echo ""
echo "⚠️  重要提醒："
echo "   - 请勿将 .env 和 .secrets/ 提交到 Git"
echo "   - 备份 .secrets/master.key 到安全位置"
echo "   - 生产环境建议使用密钥管理服务（KMS）"
echo ""
