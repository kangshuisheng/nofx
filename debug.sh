#!/bin/bash

# NOFX 部署诊断脚本
# 用于排查 Gateway Timeout 和部署问题

echo "🔍 NOFX 部署诊断工具"
echo "========================"

# 检查容器状态
echo ""
echo "📦 检查容器状态..."
docker ps -a --filter "name=nofx" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"

# 检查容器健康状态
echo ""
echo "🏥 检查容器健康状态..."
docker ps --filter "name=nofx" --format "table {{.Names}}\t{{.Status}}"

# 检查后端容器日志
echo ""
echo "📋 检查后端容器最近日志..."
docker logs --tail 10 nofx-backend 2>&1

# 检查前端容器日志
echo ""
echo "📋 检查前端容器最近日志..."
docker logs --tail 10 nofx-frontend 2>&1

# 测试后端健康检查
echo ""
echo "🔍 测试后端健康检查..."
if docker exec nofx-backend curl -f --max-time 5 http://localhost:8080/health 2>/dev/null; then
    echo "✅ 后端健康检查通过"
else
    echo "❌ 后端健康检查失败"
fi

# 测试容器间网络
echo ""
echo "🌐 测试容器间网络连接..."
if docker exec nofx-frontend curl -f --max-time 5 http://nofx-backend:8080/health 2>/dev/null; then
    echo "✅ 前端到后端网络连接正常"
else
    echo "❌ 前端到后端网络连接失败"
fi

# 检查端口监听
echo ""
echo "🔌 检查后端端口监听..."
docker exec nofx-backend netstat -tlnp 2>/dev/null | grep 8080 || echo "❌ 后端端口8080未监听"

echo ""
echo "✅ 诊断完成"