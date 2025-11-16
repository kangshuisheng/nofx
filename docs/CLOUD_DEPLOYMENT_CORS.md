# 雲部署 CORS 配置指南

## 🚀 快速開始

對於大多數雲部署場景（AWS、GCP、Azure、Railway、Render 等），**無需配置 CORS**！

系統默認處於**開發模式**，會自動允許：
- ✅ localhost（任何端口）
- ✅ .local 域名（mDNS）
- ✅ 私有 IP（10.x, 172.16-31.x, 192.168.x）
- ✅ 其他任何來源（記錄警告但允許）

---

## 📖 部署場景指南

### 場景 1：開發環境 / 測試環境

**配置**：無需任何配置

**行為**：
```bash
# .env 中無需設置 ENVIRONMENT 或 CORS_ALLOWED_ORIGINS
```

啟動日誌：
```
🔧 [CORS] 開發模式啟動：自動允許 localhost、.local 域名和私有 IP
```

**適用於**：
- Docker Compose 本地部署
- 內網測試環境
- 雲端測試環境（未設 ENVIRONMENT=production）

---

### 場景 2：生產環境（已知域名）

**配置**：設置 `.env`

```bash
# 啟用生產模式
ENVIRONMENT=production

# 配置允許的前端域名（必需！）
CORS_ALLOWED_ORIGINS=https://yourdomain.com,https://app.yourdomain.com
```

**行為**：
- ✅ 只允許白名單中的域名
- ❌ 拒絕其他所有來源（403 Forbidden）

啟動日誌：
```
🔒 [CORS] 生產模式啟動：嚴格執行白名單
    允許的來源：[https://yourdomain.com https://app.yourdomain.com]
```

**適用於**：
- 公網生產環境
- 有固定域名的部署

---

### 場景 3：生產環境（未配置域名）

如果設置了 `ENVIRONMENT=production` 但未配置 `CORS_ALLOWED_ORIGINS`，系統會顯示警告：

```
╔═══════════════════════════════════════════════════════════════════╗
║  ⚠️  警告：生產模式下未配置 CORS！                                ║
╟───────────────────────────────────────────────────────────────────╢
║  當前狀態：                                                        ║
║    • ENVIRONMENT=production（生產模式）                           ║
║    • CORS_ALLOWED_ORIGINS 未設置                                  ║
║                                                                   ║
║  預期行為：                                                        ║
║    ✅ localhost:3000, localhost:5173 可正常訪問                   ║
║    ❌ 其他所有來源將被 403 拒絕（包括域名、公網 IP）              ║
║                                                                   ║
║  解決方案（選擇其一）：                                            ║
║                                                                   ║
║  1️⃣  配置允許的前端域名（推薦用於生產環境）：                     ║
║      在 .env 中添加：                                             ║
║      CORS_ALLOWED_ORIGINS=https://yourdomain.com                 ║
║                                                                   ║
║  2️⃣  切換回開發模式（不設置 ENVIRONMENT 或設為其他值）：           ║
║      移除或註釋 .env 中的：                                       ║
║      # ENVIRONMENT=production                                    ║
║                                                                   ║
║  3️⃣  完全禁用 CORS（僅限安全的內網環境）：                        ║
║      在 .env 中添加：                                             ║
║      DISABLE_CORS=true                                           ║
╚═══════════════════════════════════════════════════════════════════╝
```

---

### 場景 4：內網生產環境（禁用 CORS）

**配置**：

```bash
# 完全禁用 CORS 檢查
DISABLE_CORS=true
```

**行為**：
- ✅ 允許所有來源訪問
- ⚠️ 無任何 CORS 保護

啟動日誌：
```
⚠️  [CORS] CORS 檢查已完全禁用 (DISABLE_CORS=true)
```

**適用於**：
- 完全隔離的內網環境
- 反向代理已處理 CORS（如 Nginx）
- 測試環境

**⚠️ 安全警告**：
- 不要在公網使用！
- 確保網絡層有其他安全措施

---

## 🌍 常見雲服務配置範例

### Vercel + Railway

**前端（Vercel）**：
- URL: `https://your-app.vercel.app`

**後端（Railway）**：
```bash
# .env
ENVIRONMENT=production
CORS_ALLOWED_ORIGINS=https://your-app.vercel.app
```

---

### AWS EC2

**單機部署**：
```bash
# .env
# 不設置 ENVIRONMENT（默認開發模式）
# Docker Compose 部署，前後端同機器
```

**分離部署（前端 S3 + CloudFront，後端 EC2）**：
```bash
# .env
ENVIRONMENT=production
CORS_ALLOWED_ORIGINS=https://d123456789.cloudfront.net
```

---

### Render

**Web Service 部署**：
```bash
# Environment Variables (Render Dashboard)
ENVIRONMENT=production
CORS_ALLOWED_ORIGINS=https://your-app.onrender.com
```

---

### Fly.io

**全球部署**：
```bash
# .env
ENVIRONMENT=production
CORS_ALLOWED_ORIGINS=https://your-app.fly.dev
```

---

## 🔧 故障排除

### 問題 1：前端訪問報錯 "Origin not allowed"

**症狀**：
```
GET http://api.example.com/api/traders 403 (Forbidden)
{ error: "Origin not allowed", origin: "https://app.example.com" }
```

**原因**：生產模式下，該來源不在白名單中

**解決**：
1. 檢查 `.env` 中的 `CORS_ALLOWED_ORIGINS`
2. 確保包含完整的 URL（含協議和端口）
3. 重啟容器：`docker-compose restart`

**範例**：
```bash
# .env
CORS_ALLOWED_ORIGINS=https://app.example.com,https://admin.example.com
```

---

### 問題 2：開發環境也被拒絕

**症狀**：
即使是 `http://localhost:3000` 也被拒絕

**原因**：設置了 `ENVIRONMENT=production` 但未配置白名單

**解決**：
```bash
# 方案 A：移除生產模式設置
# ENVIRONMENT=production  # 註釋掉這行

# 方案 B：添加 localhost 到白名單
CORS_ALLOWED_ORIGINS=http://localhost:3000,http://localhost:5173
```

---

### 問題 3：手機訪問被拒絕（192.168.x.x）

**症狀**：
```
Origin: http://192.168.1.100:3000
Error: Origin not allowed
```

**原因**：生產模式下，私有 IP 也需要顯式白名單

**解決**：
```bash
# 方案 A：切換回開發模式（推薦測試環境）
# ENVIRONMENT=production  # 註釋掉

# 方案 B：添加私有 IP 到白名單
CORS_ALLOWED_ORIGINS=http://192.168.1.100:3000
```

---

## 📝 配置檢查清單

部署前檢查：

- [ ] 確定部署環境（開發/生產）
- [ ] 確認前端訪問 URL（含協議、域名、端口）
- [ ] 配置 `.env` 文件
  - [ ] `ENVIRONMENT=production`（生產環境）
  - [ ] `CORS_ALLOWED_ORIGINS=<前端URL>`（生產環境必需）
- [ ] 測試訪問
  - [ ] 前端能正常加載
  - [ ] API 請求成功（無 403 錯誤）
  - [ ] 瀏覽器 Console 無 CORS 錯誤

---

## 🔍 調試技巧

### 查看啟動日誌

```bash
# Docker Compose
docker-compose logs api | grep CORS

# Kubernetes
kubectl logs <pod-name> | grep CORS
```

**健康的開發模式日誌**：
```
🔧 [CORS] 開發模式啟動：自動允許 localhost、.local 域名和私有 IP
```

**健康的生產模式日誌**：
```
🔒 [CORS] 生產模式啟動：嚴格執行白名單
    允許的來源：[https://yourdomain.com]
```

### 測試 CORS 請求

```bash
# 測試允許的來源
curl -H "Origin: https://yourdomain.com" \
     -H "Access-Control-Request-Method: GET" \
     -X OPTIONS \
     http://your-api.com/api/traders

# 預期返回：200 OK + CORS headers

# 測試不允許的來源（生產模式）
curl -H "Origin: https://malicious.com" \
     -X GET \
     http://your-api.com/api/traders

# 預期返回：403 Forbidden
```

---

## 📚 相關文檔

- `.env.example` - 環境變量範例
- `ISSUE_ANALYSIS_2025-11-15.md` - CORS 雲部署問題分析
- `api/server.go:40-110` - CORS 配置代碼
- `api/server.go:160-251` - CORS Middleware 實現

---

**生成者**: Claude Code
**版本**: 1.0
**更新日期**: 2025-11-15
