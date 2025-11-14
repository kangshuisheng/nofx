# 用戶數據遷移指南

**問題**: z-dev-v2 升級後「創建交易員失敗 (500 錯誤)」

---

## 🔍 問題根本原因

**數據庫 Schema 不兼容**:
```
table exchanges_new has 14 columns but 16 values were supplied
```

### 為什麼會發生？

z-dev-v2 引入了**多配置架構 (Multi-Config Architecture)**：
- ✅ 支持用戶創建多個 AI 模型配置
- ✅ 支持用戶創建多個交易所配置
- ✅ `ai_models` 和 `exchanges` 表從 TEXT ID 改為 INTEGER auto-increment ID

**舊版本數據庫結構**：
- `ai_models`: 使用 TEXT ID（例如 "deepseek"、"openai"）
- `exchanges`: 使用 TEXT ID（例如 "binance"、"hyperliquid"）

**新版本數據庫結構**：
- `ai_models`: 使用 INTEGER ID（1, 2, 3...）+ `model_id` 字段
- `exchanges`: 使用 INTEGER ID（1, 2, 3...）+ `exchange_id` 字段

---

## 📊 對用戶的影響

### 情況 A：全新安裝（沒有舊數據）

**影響**: ✅ **無影響**
- 自動創建新數據庫
- 一切正常運行

---

### 情況 B：從舊版本升級（有現有數據）

**影響**: ⚠️ **數據遷移失敗**

**症狀**:
1. ❌ 創建交易員返回 500 錯誤
2. ❌ 日誌顯示遷移錯誤
3. ❌ 舊的交易員配置可能無法訪問

**用戶數據狀態**:
- ⚠️ **舊的 trader 記錄**: 仍然存在，但可能無法正常讀取
- ⚠️ **AI 模型配置**: 需要重新配置
- ⚠️ **交易所配置**: 需要重新配置
- ❌ **交易歷史 (decision_log.db)**: **可能丟失**（如果文件被刪除）

---

## 🔧 用戶端解決方案

### 方案 A：完全重置（推薦給測試用戶）⭐

**適用**: 測試環境、沒有重要數據的用戶

```bash
# 1. 停止服務
./start.sh stop

# 2. 備份舊數據庫（可選）
mv config.db config.db.backup_$(date +%Y%m%d_%H%M%S)
mv decision_log.db decision_log.db.backup_$(date +%Y%m%d_%H%M%S) 2>/dev/null

# 3. 重新啟動（自動創建新數據庫）
./start.sh start

# 4. 重新配置
# - 登錄 Web 界面
# - 重新添加 AI 模型配置
# - 重新添加交易所配置
# - 創建新交易員
```

**結果**:
- ✅ 系統正常工作
- ❌ 舊數據丟失

---

### 方案 B：手動數據遷移（保留數據）⭐⭐⭐

**適用**: 生產環境、有重要數據的用戶

#### 步驟 1: 備份所有數據

```bash
# 停止服務
./start.sh stop

# 備份數據庫
cp config.db config.db.backup
cp decision_log.db decision_log.db.backup 2>/dev/null

# 記錄當前配置
sqlite3 config.db <<EOF
.mode insert traders
SELECT * FROM traders;
.mode insert ai_models
SELECT * FROM ai_models;
.mode insert exchanges
SELECT * FROM exchanges;
EOF
```

#### 步驟 2: 執行遷移腳本

**我們提供了自動遷移腳本**：`scripts/fix_traders_table_migration.sh`

```bash
# 運行遷移腳本（自動修復 traders 表結構）
./scripts/fix_traders_table_migration.sh config.db
```

**遷移腳本會做什麼**:
1. ✅ 自動備份數據庫（帶時間戳）
2. ✅ 檢查是否存在遷移問題（ai_model_id_old, exchange_id_old）
3. ✅ 創建新表結構（只保留正確的列）
4. ✅ 自動遷移舊數據（映射 TEXT ID → INTEGER ID）
5. ✅ 清理 WAL 緩存文件
6. ✅ 驗證數據完整性

#### 步驟 3: 重啟服務

```bash
# 重啟服務
./start.sh start
```

---

### 方案 C：使用 Docker Volume 持久化（預防措施）⭐⭐⭐⭐⭐

**目的**: 防止容器重啟導致數據丟失

**修改 `docker-compose.yml`**:

```yaml
services:
  nofx:
    volumes:
      - ./config.db:/app/config.db
      - ./decision_logs:/app/decision_logs
      - ./secrets:/app/secrets:ro
      - ./.env:/app/.env:ro
```

**好處**:
- ✅ 數據持久化在宿主機
- ✅ 容器重啟不丟失數據
- ✅ 方便備份和遷移

---

## 🚨 緊急恢復步驟（Docker 用戶）

如果你正在使用 Docker 並且遇到 500 錯誤：

```bash
# 1. 停止並刪除容器（不刪除 volume）
docker-compose down

# 2. 進入容器 volume 目錄
docker volume ls | grep nofx
# 找到 volume 名稱，例如 nofx_config

# 3. 刪除損壞的數據庫（如果使用 volume）
docker run --rm -v nofx_config:/data alpine rm -f /data/config.db

# 4. 重新啟動
docker-compose up -d
```

---

## 📋 數據遷移檢查清單

### 升級前

- [ ] 備份 `config.db`
- [ ] 備份 `decision_logs/` 目錄
- [ ] 記錄當前的交易員配置
- [ ] 記錄 AI 模型和交易所 API 密鑰

### 升級後

- [ ] 檢查日誌是否有遷移錯誤
- [ ] 測試創建新交易員
- [ ] 驗證舊交易員是否可訪問
- [ ] 檢查交易歷史數據是否完整

---

## 🔮 未來改進建議

### 1. 自動化遷移腳本

**需要開發**:
```bash
scripts/
  ├── migrate_database.sh       # 主遷移腳本
  ├── backup_database.sh        # 自動備份
  └── verify_migration.sh       # 驗證遷移結果
```

### 2. 數據庫版本控制

**實現 Schema Versioning**:
```go
// config/database.go
const CurrentSchemaVersion = 2

func (db *Database) CheckSchemaVersion() error {
    version := db.GetSchemaVersion()
    if version < CurrentSchemaVersion {
        return db.MigrateFromVersion(version, CurrentSchemaVersion)
    }
    return nil
}
```

### 3. 向後兼容模式

**支持舊版本數據結構**:
```go
// 自動檢測並轉換舊數據格式
func (db *Database) LoadTraders() error {
    if db.IsLegacySchema() {
        return db.LoadTradersLegacy()
    }
    return db.LoadTradersV2()
}
```

### 4. 用戶通知機制

**在 Web 界面顯示遷移提示**:
```tsx
{migrationRequired && (
  <Alert severity="warning">
    數據庫需要升級。請備份數據並運行遷移腳本。
    <Button onClick={runMigration}>自動遷移</Button>
  </Alert>
)}
```

---

## 💬 通知用戶的公告模板

```markdown
📢 重要：z-dev-v2 數據庫升級通知

親愛的用戶，

z-dev-v2 版本引入了多配置架構，需要升級數據庫結構。

### 如果您遇到「創建交易員失敗 (500 錯誤)」：

**快速解決方案（會丟失舊數據）**:
```bash
./start.sh stop
mv config.db config.db.backup
./start.sh start
```

**保留數據方案（推薦）**:
1. 備份數據：`cp config.db config.db.backup`
2. 聯繫我們獲取遷移腳本
3. 或等待下一個版本的自動遷移功能

### 如果您是新用戶：

無需任何操作，系統會自動創建正確的數據庫結構。

---

**我們正在開發**:
- ✅ 自動遷移腳本（下一版本）
- ✅ 數據備份工具
- ✅ 一鍵恢復功能

感謝您的理解和支持！

有問題請在 GitHub Issues 反饋。
```

---

## 📞 技術支持

**遇到問題？**

1. **查看日誌**: `./start.sh logs`
2. **檢查錯誤**: `docker logs nofx-trading 2>&1 | grep -i error`
3. **提交 Issue**: https://github.com/the-dev-z/nofx/issues

**提供以下信息**:
- 錯誤日誌
- 數據庫文件大小（`ls -lh config.db`）
- 升級前的版本號
- 是否有現有交易員數據

---

**文檔生成時間**: 2025-01-13 23:47
**適用版本**: z-dev-v2 (commit 1e5bb99f)
