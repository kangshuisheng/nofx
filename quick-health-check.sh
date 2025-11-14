#!/bin/bash

# 快速健康检查测试脚本
echo "🔍 快速健康检查测试"
echo "===================="

# 测试API健康检查
echo ""
echo "测试 /api/health 端点..."
if curl -f --max-time 5 http://localhost:8080/api/health 2>/dev/null; then
    echo "✅ /api/health 端点正常"
else
    echo "❌ /api/health 端点失败"
fi

# 测试根路径健康检查（如果存在）
echo ""
echo "测试 /health 端点..."
if curl -f --max-time 5 http://localhost:8080/health 2>/dev/null; then
    echo "✅ /health 端点正常"
else
    echo "❌ /health 端点不存在或失败"
fi

echo ""
echo "✅ 测试完成"