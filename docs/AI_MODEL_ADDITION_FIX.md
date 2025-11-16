# AI 模型無法添加問題分析與修復

**日期**: 2025-11-15
**問題**: 用戶反饋「dev2还是没法添加模型啊，添加交易所是可以的」
**狀態**: ✅ 已修復

---

## 🔍 問題根因

### 前端發送的請求格式

**文件**: `web/src/hooks/useTraderActions.ts:410-422`

```typescript
const request = {
  models: Object.fromEntries(
    updatedModels.map((model) => [
      model.provider, // ⚠️ 使用 provider 而不是 id
      {
        enabled: model.enabled,
        api_key: model.apiKey || '',
        custom_api_url: model.customApiUrl || '',
        custom_model_name: model.customModelName || '',
      },
    ])
  ),
}
```

**實際發送的 JSON**:
```json
{
  "models": {
    "deepseek": { "enabled": true, "api_key": "sk-xxx..." },
    "openai": { "enabled": true, "api_key": "sk-yyy..." }
  }
}
```

**Key 是 `provider`**（如 "deepseek", "openai"），**不是完整的 model_id**。

---

### 後端的錯誤邏輯（修復前）

**文件**: `config/database.go:1353-1356`（修復前）

```go
newModelID := id  // id = "deepseek"
if id == provider {  // "deepseek" == "deepseek" ✅
    newModelID = fmt.Sprintf("%s_%s", userID, provider)
    // ❌ 生成了 "user123_deepseek"
}

INSERT INTO ai_models (model_id, user_id, name, provider, ...)
VALUES (newModelID, userID, name, provider, ...)
// 插入 ("user123_deepseek", "user123", "DeepSeek AI", "deepseek", ...)
```

**問題流程**：

1️⃣ **第一次添加模型**：
   - 前端發送 `"deepseek": { ... }`
   - 後端接收 `id = "deepseek"`
   - 生成 `model_id = "user123_deepseek"`
   - 插入數據庫 ✅

2️⃣ **第二次更新模型**：
   - 前端又發送 `"deepseek": { ... }`（key 還是 "deepseek"）
   - 後端接收 `id = "deepseek"`
   - 嘗試查找：`SELECT model_id FROM ai_models WHERE model_id = 'deepseek'`
   - ❌ **找不到！**（數據庫中是 "user123_deepseek"）
   - 又嘗試創建新記錄
   - 可能觸發 UNIQUE 約束失敗，或創建重複記錄

---

## 🆚 對比：為什麼交易所能正常工作？

### UpdateExchange 的邏輯（正確）

**文件**: `config/database.go:1604-1616`

```go
// UpdateExchange 創建新記錄時
if hasExchangeIDColumn > 0 {
    INSERT INTO exchanges (exchange_id, user_id, name, type, ...)
    VALUES (id, userID, name, typ, ...)
    // ✅ 直接使用 id（"binance", "hyperliquid", "aster"）
}
```

**區別**：
- **Exchange**: 直接使用前端傳來的 `id`（"binance"）作為 `exchange_id`
- **Model（錯誤）**: 生成新的 `model_id`（"user123_deepseek"），與前端的 key 不一致

---

## ✅ 修復方案

### 修改後的邏輯

**文件**: `config/database.go:1353-1358`（修復後）

```go
// 🔧 修復：直接使用 id 作為 model_id，不生成新的 ID
// 這樣與前端發送的 provider 保持一致（如 "deepseek", "openai"）
// 下次更新時才能正確找到記錄
newModelID := id

log.Printf("✓ 创建新的 AI 模型配置: ID=%s, Provider=%s, Name=%s", newModelID, provider, name)
result, err := d.db.Exec(`
    INSERT INTO ai_models (model_id, user_id, name, provider, ...)
    VALUES (?, ?, ?, ?, ...)
`, newModelID, userID, name, provider, ...)
```

### 修復效果

1️⃣ **第一次添加模型**：
   - 前端發送 `"deepseek": { ... }`
   - 後端接收 `id = "deepseek"`
   - 使用 `model_id = "deepseek"`（不再生成新 ID）
   - 插入數據庫 ✅

2️⃣ **第二次更新模型**：
   - 前端發送 `"deepseek": { ... }`
   - 後端接收 `id = "deepseek"`
   - 查找：`SELECT model_id FROM ai_models WHERE model_id = 'deepseek'`
   - ✅ **找到了！**
   - 執行 UPDATE 成功 ✅

---

## 🧪 驗證步驟

### 1. 清理舊數據（如果需要）

```bash
docker exec -it nofx-api-1 sqlite3 /data/nofx.db <<EOF
-- 查看當前的 AI 模型記錄
SELECT model_id, user_id, provider FROM ai_models;

-- 如果有 "user123_deepseek" 格式的舊記錄，刪除它們
DELETE FROM ai_models WHERE model_id LIKE '%\_%';
EOF
```

### 2. 測試添加新模型

1. 登錄前端
2. 進入「AI 交易員」頁面
3. 點擊「添加 AI 模型」
4. 選擇「DeepSeek」，輸入 API Key
5. 點擊「保存」
6. ✅ 應該成功創建

### 3. 測試更新模型

1. 再次點擊「DeepSeek」模型
2. 修改 API Key 或 Custom URL
3. 點擊「保存」
4. ✅ 應該成功更新（不會創建新記錄）

### 4. 查看後端日誌

```bash
docker-compose logs -f api | grep "AI Model"
```

**預期日誌**（第一次添加）：
```
🔧 [AI Model] UpdateAIModel 開始: userID=xxx, id=deepseek, enabled=true, ...
   表結構檢查: hasModelIDColumn=1 (1=新結構, 0=舊結構)
   使用新結構邏輯（有 model_id 列）
   未找到 model_id 精確匹配，嘗試 provider 匹配...
✓ 创建新的 AI 模型配置: ID=deepseek, Provider=deepseek, Name=DeepSeek AI
✅ [AI Model] 創建新配置成功，影響行數: 1
```

**預期日誌**（第二次更新）：
```
🔧 [AI Model] UpdateAIModel 開始: userID=xxx, id=deepseek, enabled=true, ...
   表結構檢查: hasModelIDColumn=1
   使用新結構邏輯（有 model_id 列）
✓ [AI Model] 找到現有配置（model_id匹配）: deepseek, 執行更新
✅ [AI Model] 更新成功，影響行數: 1
```

---

## 📊 影響範圍

### 修改的文件

- `config/database.go:1353-1356` - 移除錯誤的 ID 生成邏輯

### 不受影響的功能

- ✅ 舊結構（沒有 model_id 列）：本來就是正確的（直接使用 id）
- ✅ Exchange 配置：邏輯保持不變
- ✅ 現有的模型更新（如果 model_id 已經是 provider 格式）

### 可能受影響的舊數據

如果用戶已經有 `"user123_deepseek"` 格式的舊記錄：
- 後端會視為「不存在」，重新創建 `"deepseek"` 記錄
- 舊記錄會變成孤立數據（不影響使用）
- **建議**：啟動時檢測並遷移舊格式的 model_id

---

## 🔮 未來改進建議

### 1. 數據遷移腳本

創建 `scripts/migrate_ai_model_ids.sh`：

```bash
#!/bin/bash
# 遷移舊格式的 model_id（user123_deepseek → deepseek）

docker exec -it nofx-api-1 sqlite3 /data/nofx.db <<EOF
BEGIN TRANSACTION;

-- 更新所有 "userID_provider" 格式的 model_id
UPDATE ai_models
SET model_id = SUBSTR(model_id, INSTR(model_id, '_') + 1)
WHERE model_id LIKE '%\_%'
  AND INSTR(model_id, '_') > 0;

COMMIT;
EOF
```

### 2. 前端改進

考慮在前端顯示實際的 `model_id` 而非 `provider`，方便調試：

```typescript
// useTraderActions.ts
console.log('📤 Sending model config:', {
  modelId: model.provider,  // 實際發送的 key
  data: { enabled, api_key, ... }
})
```

### 3. 後端驗證

在 `UpdateAIModel` 開頭添加驗證：

```go
// 驗證 ID 格式（應該是純 provider，不包含 userID）
if strings.Contains(id, "_") && strings.HasPrefix(id, userID) {
    log.Printf("⚠️  檢測到舊格式的 model_id: %s，自動遷移為: %s", id, provider)
    id = provider  // 自動修正
}
```

---

## 📝 相關提交

- **Commit**: 0307e7e8
- **分支**: z-dev-v2
- **日期**: 2025-11-15

---

**生成者**: Claude Code
**版本**: 1.0
