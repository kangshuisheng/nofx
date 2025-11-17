# 🚀 NOFX Coolify 部署指南

## 📋 部署前准备

### 1. 环境变量设置
在 Coolify 中设置以下环境变量：

```bash
NOFX_ADMIN_PASSWORD=your_secure_password
TZ=Asia/Shanghai
AI_MAX_TOKENS=4000
```

### 2. 选择部署方案

## 🎯 方案一：Docker Compose 部署（推荐）

### 步骤 1: 使用优化的 Docker Compose
```bash
# 重命名配置文件
mv docker-compose.yml docker-compose.original.yml
mv docker-compose.coolify.yml docker-compose.yml
```

### 步骤 2: 在 Coolify 中配置
1. 选择 "Docker Compose" 部署类型
2. 设置 Git 仓库地址
3. 设置环境变量
4. 端口映射：`3001:80`（前端服务）
5. 域名指向前端服务

### 步骤 3: 部署
- Coolify 会自动构建并启动服务
- 访问你的域名应该能看到 NOFX 界面

## 🎯 方案二：单容器部署（简单）

### 步骤 1: 使用单容器 Dockerfile
```bash
# 重命名 Dockerfile
mv Dockerfile.single Dockerfile
```

### 步骤 2: 在 Coolify 中配置
1. 选择 "Dockerfile" 部署类型
2. 设置 Git 仓库地址
3. 设置环境变量
4. 端口映射：`3001:80`
5. 健康检查：`/health`

## 🔧 故障排除

### 404 错误解决方案

#### 1. 检查服务状态
```bash
# 在 Coolify 容器中执行
docker ps
docker logs <container_name>
```

#### 2. 检查 Nginx 配置
```bash
# 进入前端容器
docker exec -it <frontend_container> sh
cat /etc/nginx/conf.d/default.conf
```

#### 3. 检查后端连接
```bash
# 测试后端 API
curl http://localhost:8080/api/health
# 或从前端容器测试
curl http://nofx-backend:8080/api/health
```

#### 4. 检查网络连接
```bash
# 检查容器网络
docker network ls
docker network inspect <network_name>
```

### 常见问题

#### 问题 1: 前端加载但 API 调用失败
**原因**: Nginx 代理配置错误
**解决**: 检查 `nginx.conf` 中的 `proxy_pass` 地址

#### 问题 2: 容器启动失败
**原因**: 环境变量缺失或配置文件错误
**解决**: 检查环境变量和 `config.json`

#### 问题 3: 数据库连接失败
**原因**: 数据卷挂载问题
**解决**: 确保 `nofx-data` 卷正确挂载

### 🔐 密钥与提示词（Coolify 特殊说明）

 - Secrets（RSA 私钥）:
   - Coolify 上的仓库目录有时会以只读方式挂载；如果 `secrets/` 目录被标记为只读，后端无法自动生成 RSA 私钥，报错示例: "open secrets/rsa_key: read-only file system"。
   - 解决方法：
     1. 在 Coolify 的环境配置中把 `secrets` 设置为可写卷（移除 :ro），或使用 Docker 命名卷：`secrets:/app/secrets`
     2. 或者在部署前手动生成密钥并将 `secrets/rsa_key` 和 `secrets/rsa_key.pub` 添加到仓库/卷（推荐通过脚本 `./scripts/setup_encryption.sh` 生成）。
     3. 另一个可选方式：在 Coolify 环境变量中设置 `RSA_PRIVATE_KEY`（私钥 PEM 文本），应用会在无法写入时自动加载该环境变量作为私钥（出于安全考虑，使用 Coolify 的 Secret 功能保存 PEM 值）。

 - Prompts:
   - 如果你在 Coolify 的 `docker-compose` 中挂载了 `./prompts:/app/prompts`，但宿主机（Coolify）上该目录为空，会覆盖镜像里内置的提示词，导致日志: "提示词目录 prompts 中没有找到 .txt 文件"。
   - 解决方法：
     1. 避免在 Coolify 上绑定挂载 `./prompts`，使用镜像自带资源（删除 `prompts` 卷映射）；或
     2. 将 `prompts` 目录中的 `.txt` 文件推到仓库并确保 Coolify 在部署时有这些文件，或使用命名卷持久化。

## 📊 验证部署

### 1. 健康检查
访问：`https://your-domain.com/health`
应该返回：`OK`

### 2. API 检查
访问：`https://your-domain.com/api/health`
应该返回：`{"status":"ok"}`

### 3. 前端检查
访问：`https://your-domain.com`
应该看到 NOFX 交易界面

## 🔐 安全配置

### 1. 设置管理员密码
```json
{
  "admin_mode": true,
  "jwt_secret": "your-jwt-secret"
}
```

### 2. 环境变量
```bash
NOFX_ADMIN_PASSWORD=your_secure_password
```

## 📝 配置文件示例

### config.json（最小配置）
```json
{
  "leverage": {
    "btc_eth_leverage": 5,
    "altcoin_leverage": 5
  },
  "use_default_coins": true,
  "api_server_port": 8080,
  "admin_mode": true,
  "jwt_secret": "your-jwt-secret-here"
}
```

## 🚀 部署后配置

1. 访问 Web 界面
2. 配置 AI 模型（DeepSeek/Qwen API）
3. 配置交易所（Binance/Hyperliquid）
4. 创建交易员
5. 开始交易

## 📞 获取帮助

如果遇到问题：
1. 检查 Coolify 日志
2. 检查容器日志
3. 参考项目文档
4. 加入 Telegram 群组：https://t.me/nofx_dev_community