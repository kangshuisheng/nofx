package config

import (
	"crypto/rand"
	"database/sql"
	"encoding/base32"
	"encoding/json"
	"fmt"
	"log"
	"nofx/crypto"
	"nofx/market"
	"os"
	"slices"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// DatabaseInterface 定义了数据库实现需要提供的方法集合
type DatabaseInterface interface {
	SetCryptoService(cs *crypto.CryptoService)
	CreateUser(user *User) error
	GetUserByEmail(email string) (*User, error)
	GetUserByID(userID string) (*User, error)
	GetAllUsers() ([]string, error)
	UpdateUserOTPVerified(userID string, verified bool) error
	GetAIModels(userID string) ([]*AIModelConfig, error)
	UpdateAIModel(userID, id string, enabled bool, apiKey, customAPIURL, customModelName string) error
	GetExchanges(userID string) ([]*ExchangeConfig, error)
	UpdateExchange(userID, id string, enabled bool, apiKey, secretKey string, testnet bool, hyperliquidWalletAddr, asterUser, asterSigner, asterPrivateKey string) error
	CreateAIModel(userID, id, name, provider string, enabled bool, apiKey, customAPIURL string) error
	CreateExchange(userID, id, name, typ string, enabled bool, apiKey, secretKey string, testnet bool, hyperliquidWalletAddr, asterUser, asterSigner, asterPrivateKey string) error
	CreateTrader(trader *TraderRecord) error
	GetTraders(userID string) ([]*TraderRecord, error)
	UpdateTraderStatus(userID, id string, isRunning bool) error
	UpdateTrader(trader *TraderRecord) error
	UpdateTraderInitialBalance(userID, id string, newBalance float64) error
	UpdateTraderCustomPrompt(userID, id string, customPrompt string, overrideBase bool) error
	DeleteTrader(userID, id string) error
	GetTraderConfig(userID, traderID string) (*TraderRecord, *AIModelConfig, *ExchangeConfig, error)
	GetSystemConfig(key string) (string, error)
	SetSystemConfig(key, value string) error
	CreateUserSignalSource(userID, coinPoolURL, oiTopURL string) error
	GetUserSignalSource(userID string) (*UserSignalSource, error)
	UpdateUserSignalSource(userID, coinPoolURL, oiTopURL string) error
	GetCustomCoins() []string
	GetAllTimeframes() []string
	LoadBetaCodesFromFile(filePath string) error
	ValidateBetaCode(code string) (bool, error)
	UseBetaCode(code, userEmail string) error
	GetBetaCodeStats() (total, used int, err error)
	Close() error
}

// Database 配置数据库
type Database struct {
	db            *sql.DB
	cryptoService *crypto.CryptoService
}

// NewDatabase 创建配置数据库
func NewDatabase(dbPath string) (*Database, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}

	// 🔒 启用 WAL 模式,提高并发性能和崩溃恢复能力
	// WAL (Write-Ahead Logging) 模式的优势:
	// 1. 更好的并发性能:读操作不会被写操作阻塞
	// 2. 崩溃安全:即使在断电或强制终止时也能保证数据完整性
	// 3. 更快的写入:不需要每次都写入主数据库文件
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("启用WAL模式失败: %w", err)
	}

	// 🔒 设置 synchronous=FULL 确保数据持久性
	// FULL (2) 模式: 确保数据在关键时刻完全写入磁盘
	// 配合 WAL 模式,在保证数据安全的同时获得良好性能
	if _, err := db.Exec("PRAGMA synchronous=FULL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("设置synchronous失败: %w", err)
	}

	database := &Database{db: db}
	if err := database.createTables(); err != nil {
		return nil, fmt.Errorf("创建表失败: %w", err)
	}

	// Automatically cleanup legacy _old columns for smooth upgrades
	if err := database.cleanupLegacyColumns(); err != nil {
		return nil, fmt.Errorf("清理遗留列失败: %w", err)
	}

	if err := database.initDefaultData(); err != nil {
		return nil, fmt.Errorf("初始化默认数据失败: %w", err)
	}

	log.Printf("✅ 数据库已启用 WAL 模式和 FULL 同步,数据持久性得到保证")
	return database, nil
}

// createTables 创建数据库表
func (d *Database) createTables() error {
	queries := []string{
		// AI模型配置表（使用自增ID支持多配置）
		`CREATE TABLE IF NOT EXISTS ai_models (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			model_id TEXT NOT NULL,
			user_id TEXT NOT NULL DEFAULT 'default',
			display_name TEXT DEFAULT '',
			name TEXT NOT NULL,
			provider TEXT NOT NULL,
			enabled BOOLEAN DEFAULT 0,
			api_key TEXT DEFAULT '',
			custom_api_url TEXT DEFAULT '',
			custom_model_name TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,

		// 交易所配置表（使用自增ID支持多配置）
		`CREATE TABLE IF NOT EXISTS exchanges (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			exchange_id TEXT NOT NULL,
			user_id TEXT NOT NULL DEFAULT 'default',
			display_name TEXT DEFAULT '',
			name TEXT NOT NULL,
			type TEXT NOT NULL, -- 'cex' or 'dex'
			enabled BOOLEAN DEFAULT 0,
			api_key TEXT DEFAULT '',
			secret_key TEXT DEFAULT '',
			testnet BOOLEAN DEFAULT 0,
			-- Hyperliquid 特定字段
			hyperliquid_wallet_addr TEXT DEFAULT '',
			-- Aster 特定字段
			aster_user TEXT DEFAULT '',
			aster_signer TEXT DEFAULT '',
			aster_private_key TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,

		// 用户信号源配置表
		`CREATE TABLE IF NOT EXISTS user_signal_sources (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			coin_pool_url TEXT DEFAULT '',
			oi_top_url TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			UNIQUE(user_id)
		)`,

		// 交易员配置表
		`CREATE TABLE IF NOT EXISTS traders (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL DEFAULT 'default',
			name TEXT NOT NULL,
			ai_model_id INTEGER NOT NULL,
			exchange_id INTEGER NOT NULL,
			initial_balance REAL NOT NULL,
			scan_interval_minutes INTEGER DEFAULT 3,
			is_running BOOLEAN DEFAULT 0,
			btc_eth_leverage INTEGER DEFAULT 5,
			altcoin_leverage INTEGER DEFAULT 5,
			trading_symbols TEXT DEFAULT '',
			use_coin_pool BOOLEAN DEFAULT 0,
			use_oi_top BOOLEAN DEFAULT 0,
			custom_prompt TEXT DEFAULT '',
			override_base_prompt BOOLEAN DEFAULT 0,
			system_prompt_template TEXT DEFAULT 'default',
			is_cross_margin BOOLEAN DEFAULT 1,
			taker_fee_rate REAL DEFAULT 0.0004,
			maker_fee_rate REAL DEFAULT 0.0002,
			order_strategy TEXT DEFAULT 'conservative_hybrid',
			limit_price_offset REAL DEFAULT -0.03,
			limit_timeout_seconds INTEGER DEFAULT 60,
			timeframes TEXT DEFAULT '4h',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY (ai_model_id) REFERENCES ai_models(id),
			FOREIGN KEY (exchange_id) REFERENCES exchanges(id)
		)`,

		// 用户表
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			email TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			otp_secret TEXT,
			otp_verified BOOLEAN DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// 系统配置表
		`CREATE TABLE IF NOT EXISTS system_config (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// 内测码表
		`CREATE TABLE IF NOT EXISTS beta_codes (
			code TEXT PRIMARY KEY,
			used BOOLEAN DEFAULT 0,
			used_by TEXT DEFAULT '',
			used_at DATETIME DEFAULT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// 触发器：自动更新 updated_at
		`CREATE TRIGGER IF NOT EXISTS update_users_updated_at
			AFTER UPDATE ON users
			BEGIN
				UPDATE users SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
			END`,

		`CREATE TRIGGER IF NOT EXISTS update_ai_models_updated_at
			AFTER UPDATE ON ai_models
			BEGIN
				UPDATE ai_models SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
			END`,

		`CREATE TRIGGER IF NOT EXISTS update_exchanges_updated_at
			AFTER UPDATE ON exchanges
			BEGIN
				UPDATE exchanges SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
			END`,

		`CREATE TRIGGER IF NOT EXISTS update_traders_updated_at
			AFTER UPDATE ON traders
			BEGIN
				UPDATE traders SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
			END`,

		`CREATE TRIGGER IF NOT EXISTS update_user_signal_sources_updated_at
			AFTER UPDATE ON user_signal_sources
			BEGIN
				UPDATE user_signal_sources SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
			END`,

		`CREATE TRIGGER IF NOT EXISTS update_system_config_updated_at
			AFTER UPDATE ON system_config
			BEGIN
				UPDATE system_config SET updated_at = CURRENT_TIMESTAMP WHERE key = NEW.key;
			END`,
	}

	for _, query := range queries {
		if _, err := d.db.Exec(query); err != nil {
			return fmt.Errorf("执行SQL失败 [%s]: %w", query, err)
		}
	}

	// 为现有数据库添加新字段（向后兼容）
	alterQueries := []string{
		`ALTER TABLE exchanges ADD COLUMN hyperliquid_wallet_addr TEXT DEFAULT ''`,
		`ALTER TABLE exchanges ADD COLUMN aster_user TEXT DEFAULT ''`,
		`ALTER TABLE exchanges ADD COLUMN aster_signer TEXT DEFAULT ''`,
		`ALTER TABLE exchanges ADD COLUMN aster_private_key TEXT DEFAULT ''`,
		`ALTER TABLE traders ADD COLUMN custom_prompt TEXT DEFAULT ''`,
		`ALTER TABLE traders ADD COLUMN override_base_prompt BOOLEAN DEFAULT 0`,
		`ALTER TABLE traders ADD COLUMN is_cross_margin BOOLEAN DEFAULT 1`,                 // 默认为全仓模式
		`ALTER TABLE traders ADD COLUMN use_default_coins BOOLEAN DEFAULT 1`,               // 默认使用默认币种
		`ALTER TABLE traders ADD COLUMN custom_coins TEXT DEFAULT ''`,                      // 自定义币种列表（JSON格式）
		`ALTER TABLE traders ADD COLUMN btc_eth_leverage INTEGER DEFAULT 5`,                // BTC/ETH杠杆倍数
		`ALTER TABLE traders ADD COLUMN altcoin_leverage INTEGER DEFAULT 5`,                // 山寨币杠杆倍数
		`ALTER TABLE traders ADD COLUMN trading_symbols TEXT DEFAULT ''`,                   // 交易币种，逗号分隔
		`ALTER TABLE traders ADD COLUMN use_coin_pool BOOLEAN DEFAULT 0`,                   // 是否使用COIN POOL信号源
		`ALTER TABLE traders ADD COLUMN use_oi_top BOOLEAN DEFAULT 0`,                      // 是否使用OI TOP信号源
		`ALTER TABLE traders ADD COLUMN system_prompt_template TEXT DEFAULT 'default'`,     // 系统提示词模板名称
		`ALTER TABLE traders ADD COLUMN taker_fee_rate REAL DEFAULT 0.0004`,                // Taker fee rate, default 0.0004
		`ALTER TABLE traders ADD COLUMN maker_fee_rate REAL DEFAULT 0.0002`,                // Maker fee rate, default 0.0002
		`ALTER TABLE traders ADD COLUMN order_strategy TEXT DEFAULT 'conservative_hybrid'`, // Order strategy: market_only, conservative_hybrid, limit_only
		`ALTER TABLE traders ADD COLUMN limit_price_offset REAL DEFAULT -0.03`,             // Limit order price offset percentage (e.g., -0.03 for -0.03%)
		`ALTER TABLE traders ADD COLUMN limit_timeout_seconds INTEGER DEFAULT 60`,          // Timeout in seconds before converting to market order
		`ALTER TABLE traders ADD COLUMN timeframes TEXT DEFAULT '4h'`,                      // 时间线选择 (逗号分隔，例如: "1m,4h,1d")
		`ALTER TABLE ai_models ADD COLUMN custom_api_url TEXT DEFAULT ''`,                  // 自定义API地址
		`ALTER TABLE ai_models ADD COLUMN custom_model_name TEXT DEFAULT ''`,               // 自定义模型名称
	}

	for _, query := range alterQueries {
		// 忽略已存在字段的错误
		d.db.Exec(query)
	}

	// 检查是否需要迁移exchanges表的主键结构
	err := d.migrateExchangesTable()
	if err != nil {
		log.Printf("⚠️ 迁移exchanges表失败: %v", err)
	}

	// 迁移到自增ID结构（支持多配置）
	err = d.migrateToAutoIncrementID()
	if err != nil {
		log.Printf("⚠️ 迁移自增ID失败: %v", err)
	}

	return nil
}

// initDefaultData 初始化默认数据
func (d *Database) initDefaultData() error {
	// 初始化AI模型（使用default用户）
	// 注意：遷移到自增 ID 後，需要使用 model_id 而不是 id
	aiModels := []struct {
		modelID, name, provider string
	}{
		{"deepseek", "DeepSeek", "deepseek"},
		{"qwen", "Qwen", "qwen"},
	}

	for _, model := range aiModels {
		// 檢查是否已存在（使用 model_id 和 user_id 組合）
		var count int
		err := d.db.QueryRow(`
			SELECT COUNT(*) FROM ai_models
			WHERE model_id = ? AND user_id = 'default'
		`, model.modelID).Scan(&count)
		if err != nil {
			return fmt.Errorf("检查AI模型失败: %w", err)
		}

		if count == 0 {
			// 不存在則插入，讓 id 自動遞增
			_, err = d.db.Exec(`
				INSERT INTO ai_models (user_id, model_id, name, provider, enabled)
				VALUES ('default', ?, ?, ?, 0)
			`, model.modelID, model.name, model.provider)
			if err != nil {
				return fmt.Errorf("初始化AI模型失败: %w", err)
			}
		}
	}

	// 初始化交易所（使用default用户）
	// 注意：需要兼容不同版本的表結構（遷移前後）
	exchanges := []struct {
		exchangeID, name, typ string
	}{
		{"binance", "Binance Futures", "binance"},
		{"hyperliquid", "Hyperliquid", "hyperliquid"},
		{"aster", "Aster DEX", "aster"},
	}

	// 檢查表結構，判斷是否已遷移到自增ID結構
	var hasExchangeIDColumn int
	err := d.db.QueryRow(`
		SELECT COUNT(*) FROM pragma_table_info('exchanges')
		WHERE name = 'exchange_id'
	`).Scan(&hasExchangeIDColumn)
	if err != nil {
		return fmt.Errorf("检查exchanges表结构失败: %w", err)
	}

	for _, exchange := range exchanges {
		var count int

		if hasExchangeIDColumn > 0 {
			// 新結構：使用 exchange_id
			err = d.db.QueryRow(`
				SELECT COUNT(*) FROM exchanges
				WHERE exchange_id = ? AND user_id = 'default'
			`, exchange.exchangeID).Scan(&count)
			if err != nil {
				return fmt.Errorf("检查交易所失败: %w", err)
			}

			if count == 0 {
				_, err = d.db.Exec(`
					INSERT INTO exchanges (user_id, exchange_id, name, type, enabled)
					VALUES ('default', ?, ?, ?, 0)
				`, exchange.exchangeID, exchange.name, exchange.typ)
				if err != nil {
					return fmt.Errorf("初始化交易所失败: %w", err)
				}
			}
		} else {
			// 舊結構：使用 id
			err = d.db.QueryRow(`
				SELECT COUNT(*) FROM exchanges
				WHERE id = ? AND user_id = 'default'
			`, exchange.exchangeID).Scan(&count)
			if err != nil {
				return fmt.Errorf("检查交易所失败: %w", err)
			}

			if count == 0 {
				_, err = d.db.Exec(`
					INSERT INTO exchanges (user_id, id, name, type, enabled)
					VALUES ('default', ?, ?, ?, 0)
				`, exchange.exchangeID, exchange.name, exchange.typ)
				if err != nil {
					return fmt.Errorf("初始化交易所失败: %w", err)
				}
			}
		}
	}

	// 初始化系统配置 - 创建所有字段，设置默认值，后续由config.json同步更新
	systemConfigs := map[string]string{
		"beta_mode":            "false",                                                                               // 默认关闭内测模式
		"api_server_port":      "8080",                                                                                // 默认API端口
		"use_default_coins":    "true",                                                                                // 默认使用内置币种列表
		"default_coins":        `["BTCUSDT","ETHUSDT","SOLUSDT","BNBUSDT","XRPUSDT","DOGEUSDT","ADAUSDT","HYPEUSDT"]`, // 默认币种列表（JSON格式）
		"max_daily_loss":       "10.0",                                                                                // 最大日损失百分比
		"max_drawdown":         "20.0",                                                                                // 最大回撤百分比
		"stop_trading_minutes": "60",                                                                                  // 停止交易时间（分钟）
		"btc_eth_leverage":     "5",                                                                                   // BTC/ETH杠杆倍数
		"altcoin_leverage":     "5",                                                                                   // 山寨币杠杆倍数
		"jwt_secret":           "",                                                                                    // JWT密钥，默认为空，由config.json或系统生成
		"registration_enabled": "true",                                                                                // 默认允许注册
	}

	for key, value := range systemConfigs {
		_, err := d.db.Exec(`
			INSERT OR IGNORE INTO system_config (key, value) 
			VALUES (?, ?)
		`, key, value)
		if err != nil {
			return fmt.Errorf("初始化系统配置失败: %w", err)
		}
	}

	return nil
}

// migrateExchangesTable 迁移exchanges表支持多用户
func (d *Database) migrateExchangesTable() error {
	// 检查表是否已经有 exchange_id 欄位（表示已經是新結構或已遷移）
	var hasExchangeIDColumn int
	err := d.db.QueryRow(`
		SELECT COUNT(*) FROM pragma_table_info('exchanges')
		WHERE name = 'exchange_id'
	`).Scan(&hasExchangeIDColumn)
	if err != nil {
		return err
	}

	// 如果表已經有 exchange_id 欄位，說明是新結構或已遷移，直接跳過
	if hasExchangeIDColumn > 0 {
		return nil
	}

	// 检查是否正在迁移中（exchanges_new 表存在）
	var migratingCount int
	err = d.db.QueryRow(`
		SELECT COUNT(*) FROM sqlite_master
		WHERE type='table' AND name='exchanges_new'
	`).Scan(&migratingCount)
	if err != nil {
		return err
	}

	// 如果正在迁移中，直接返回
	if migratingCount > 0 {
		return nil
	}

	log.Printf("🔄 开始迁移exchanges表（舊TEXT PRIMARY KEY -> 新TEXT複合主鍵）...")

	// 创建新的exchanges表，使用复合主键
	_, err = d.db.Exec(`
		CREATE TABLE exchanges_new (
			id TEXT NOT NULL,
			user_id TEXT NOT NULL DEFAULT 'default',
			name TEXT NOT NULL,
			type TEXT NOT NULL,
			enabled BOOLEAN DEFAULT 0,
			api_key TEXT DEFAULT '',
			secret_key TEXT DEFAULT '',
			testnet BOOLEAN DEFAULT 0,
			hyperliquid_wallet_addr TEXT DEFAULT '',
			aster_user TEXT DEFAULT '',
			aster_signer TEXT DEFAULT '',
			aster_private_key TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (id, user_id),
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return fmt.Errorf("创建新exchanges表失败: %w", err)
	}

	// 复制数据到新表（明确指定列名，兼容不同schema版本）
	_, err = d.db.Exec(`
		INSERT INTO exchanges_new (
			id, user_id, name, type, enabled, api_key, secret_key, testnet,
			hyperliquid_wallet_addr, aster_user, aster_signer, aster_private_key,
			created_at, updated_at
		)
		SELECT
			id, user_id, name, type, enabled, api_key, secret_key, testnet,
			hyperliquid_wallet_addr, aster_user, aster_signer, aster_private_key,
			created_at, updated_at
		FROM exchanges
	`)
	if err != nil {
		return fmt.Errorf("复制数据失败: %w", err)
	}

	// 删除旧表
	_, err = d.db.Exec(`DROP TABLE exchanges`)
	if err != nil {
		return fmt.Errorf("删除旧表失败: %w", err)
	}

	// 重命名新表
	_, err = d.db.Exec(`ALTER TABLE exchanges_new RENAME TO exchanges`)
	if err != nil {
		return fmt.Errorf("重命名表失败: %w", err)
	}

	// 重新创建触发器
	_, err = d.db.Exec(`
		CREATE TRIGGER IF NOT EXISTS update_exchanges_updated_at
			AFTER UPDATE ON exchanges
			BEGIN
				UPDATE exchanges SET updated_at = CURRENT_TIMESTAMP 
				WHERE id = NEW.id AND user_id = NEW.user_id;
			END
	`)
	if err != nil {
		return fmt.Errorf("创建触发器失败: %w", err)
	}

	log.Printf("✅ exchanges表迁移完成")
	return nil
}

// migrateToAutoIncrementID 迁移到自增ID结构（支持多配置）
func (d *Database) migrateToAutoIncrementID() error {
	// 检查是否已经迁移过（通过检查 ai_models 表是否有 model_id 列）
	var count int
	err := d.db.QueryRow(`
		SELECT COUNT(*) FROM pragma_table_info('ai_models')
		WHERE name = 'model_id'
	`).Scan(&count)
	if err != nil {
		return fmt.Errorf("检查迁移状态失败: %w", err)
	}

	// 如果已经迁移过，直接返回
	if count > 0 {
		return nil
	}

	log.Printf("🔄 开始迁移到自增ID结构（支持多配置）...")

	// === 步骤1：迁移 ai_models 表 ===
	if err := d.migrateAIModelsTable(); err != nil {
		return fmt.Errorf("迁移 ai_models 表失败: %w", err)
	}

	// === 步骤2：迁移 exchanges 表（再次，改为自增ID） ===
	if err := d.migrateExchangesTableToAutoIncrement(); err != nil {
		return fmt.Errorf("迁移 exchanges 表到自增ID失败: %w", err)
	}

	log.Printf("✅ 自增ID结构迁移完成")
	return nil
}

// migrateAIModelsTable 迁移 ai_models 表到自增ID结构
func (d *Database) migrateAIModelsTable() error {
	log.Printf("  🔄 迁移 ai_models 表...")

	// 1. 创建新表
	_, err := d.db.Exec(`
		CREATE TABLE ai_models_new (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			model_id TEXT NOT NULL,
			user_id TEXT NOT NULL DEFAULT 'default',
			display_name TEXT DEFAULT '',
			name TEXT NOT NULL,
			provider TEXT NOT NULL,
			enabled BOOLEAN DEFAULT 0,
			api_key TEXT DEFAULT '',
			custom_api_url TEXT DEFAULT '',
			custom_model_name TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return fmt.Errorf("创建新表失败: %w", err)
	}

	// 2. 迁移数据：从旧ID中提取 model_id
	// 旧ID格式："{user_id}_{model_id}" 或 "{model_id}"（default用户）
	rows, err := d.db.Query(`SELECT id, user_id, name, provider, enabled, api_key, custom_api_url, custom_model_name, created_at, updated_at FROM ai_models`)
	if err != nil {
		return fmt.Errorf("查询旧数据失败: %w", err)
	}
	defer rows.Close()

	// 创建映射表：旧ID -> 新ID
	oldToNewID := make(map[string]int)

	for rows.Next() {
		var oldID, userID, name, provider, apiKey, customAPIURL, customModelName string
		var enabled bool
		var createdAt, updatedAt time.Time

		if err := rows.Scan(&oldID, &userID, &name, &provider, &enabled, &apiKey, &customAPIURL, &customModelName, &createdAt, &updatedAt); err != nil {
			return fmt.Errorf("读取数据失败: %w", err)
		}

		// 提取 model_id：去掉前缀 "{user_id}_"
		modelID := oldID
		if strings.HasPrefix(oldID, userID+"_") {
			modelID = strings.TrimPrefix(oldID, userID+"_")
		}

		// 插入新表
		result, err := d.db.Exec(`
			INSERT INTO ai_models_new (model_id, user_id, name, provider, enabled, api_key, custom_api_url, custom_model_name, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, modelID, userID, name, provider, enabled, apiKey, customAPIURL, customModelName, createdAt, updatedAt)
		if err != nil {
			return fmt.Errorf("插入数据失败: %w", err)
		}

		// 获取新ID
		newID, _ := result.LastInsertId()
		oldToNewID[oldID] = int(newID)
	}

	// 3. 更新 traders 表中的 ai_model_id（使用临时列）
	_, err = d.db.Exec(`ALTER TABLE traders ADD COLUMN ai_model_id_new INTEGER`)
	if err != nil {
		return fmt.Errorf("添加临时列失败: %w", err)
	}

	// 更新外键引用
	for oldID, newID := range oldToNewID {
		_, err = d.db.Exec(`UPDATE traders SET ai_model_id_new = ? WHERE ai_model_id = ?`, newID, oldID)
		if err != nil {
			return fmt.Errorf("更新 traders 外键失败: %w", err)
		}
	}

	// 4. 删除旧表
	_, err = d.db.Exec(`DROP TABLE ai_models`)
	if err != nil {
		return fmt.Errorf("删除旧表失败: %w", err)
	}

	// 5. 重命名新表
	_, err = d.db.Exec(`ALTER TABLE ai_models_new RENAME TO ai_models`)
	if err != nil {
		return fmt.Errorf("重命名表失败: %w", err)
	}

	// 6. 更新 traders 表的列名
	_, err = d.db.Exec(`ALTER TABLE traders RENAME COLUMN ai_model_id TO ai_model_id_old`)
	if err != nil {
		return fmt.Errorf("重命名旧列失败: %w", err)
	}
	_, err = d.db.Exec(`ALTER TABLE traders RENAME COLUMN ai_model_id_new TO ai_model_id`)
	if err != nil {
		return fmt.Errorf("重命名新列失败: %w", err)
	}

	// 7. 重新创建触发器
	_, err = d.db.Exec(`
		CREATE TRIGGER IF NOT EXISTS update_ai_models_updated_at
			AFTER UPDATE ON ai_models
			BEGIN
				UPDATE ai_models SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
			END
	`)
	if err != nil {
		return fmt.Errorf("创建触发器失败: %w", err)
	}

	log.Printf("  ✅ ai_models 表迁移完成，共迁移 %d 条记录", len(oldToNewID))
	return nil
}

// migrateExchangesTableToAutoIncrement 迁移 exchanges 表到自增ID结构
func (d *Database) migrateExchangesTableToAutoIncrement() error {
	log.Printf("  🔄 迁移 exchanges 表到自增ID...")

	// 1. 创建新表
	_, err := d.db.Exec(`
		CREATE TABLE exchanges_new2 (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			exchange_id TEXT NOT NULL,
			user_id TEXT NOT NULL DEFAULT 'default',
			display_name TEXT DEFAULT '',
			name TEXT NOT NULL,
			type TEXT NOT NULL,
			enabled BOOLEAN DEFAULT 0,
			api_key TEXT DEFAULT '',
			secret_key TEXT DEFAULT '',
			testnet BOOLEAN DEFAULT 0,
			hyperliquid_wallet_addr TEXT DEFAULT '',
			aster_user TEXT DEFAULT '',
			aster_signer TEXT DEFAULT '',
			aster_private_key TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return fmt.Errorf("创建新表失败: %w", err)
	}

	// 2. 迁移数据
	rows, err := d.db.Query(`SELECT id, user_id, name, type, enabled, api_key, secret_key, testnet, hyperliquid_wallet_addr, aster_user, aster_signer, aster_private_key, created_at, updated_at FROM exchanges`)
	if err != nil {
		return fmt.Errorf("查询旧数据失败: %w", err)
	}
	defer rows.Close()

	// 创建映射：(旧exchange_id, user_id) -> 新ID
	type OldKey struct {
		ExchangeID string
		UserID     string
	}
	oldToNewID := make(map[OldKey]int)

	for rows.Next() {
		var exchangeID, userID, name, typeStr, apiKey, secretKey, hyperliquidAddr, asterUser, asterSigner, asterKey string
		var enabled, testnet bool
		var createdAt, updatedAt time.Time

		if err := rows.Scan(&exchangeID, &userID, &name, &typeStr, &enabled, &apiKey, &secretKey, &testnet, &hyperliquidAddr, &asterUser, &asterSigner, &asterKey, &createdAt, &updatedAt); err != nil {
			return fmt.Errorf("读取数据失败: %w", err)
		}

		// 插入新表
		result, err := d.db.Exec(`
			INSERT INTO exchanges_new2 (exchange_id, user_id, name, type, enabled, api_key, secret_key, testnet, hyperliquid_wallet_addr, aster_user, aster_signer, aster_private_key, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, exchangeID, userID, name, typeStr, enabled, apiKey, secretKey, testnet, hyperliquidAddr, asterUser, asterSigner, asterKey, createdAt, updatedAt)
		if err != nil {
			return fmt.Errorf("插入数据失败: %w", err)
		}

		// 获取新ID
		newID, _ := result.LastInsertId()
		oldToNewID[OldKey{exchangeID, userID}] = int(newID)
	}

	// 3. 更新 traders 表中的 exchange_id
	_, err = d.db.Exec(`ALTER TABLE traders ADD COLUMN exchange_id_new INTEGER`)
	if err != nil {
		return fmt.Errorf("添加临时列失败: %w", err)
	}

	// 更新外键引用（需要同时匹配 exchange_id 和 user_id）
	for key, newID := range oldToNewID {
		_, err = d.db.Exec(`UPDATE traders SET exchange_id_new = ? WHERE exchange_id = ? AND user_id = ?`, newID, key.ExchangeID, key.UserID)
		if err != nil {
			return fmt.Errorf("更新 traders 外键失败: %w", err)
		}
	}

	// 4. 删除旧表
	_, err = d.db.Exec(`DROP TABLE exchanges`)
	if err != nil {
		return fmt.Errorf("删除旧表失败: %w", err)
	}

	// 5. 重命名新表
	_, err = d.db.Exec(`ALTER TABLE exchanges_new2 RENAME TO exchanges`)
	if err != nil {
		return fmt.Errorf("重命名表失败: %w", err)
	}

	// 6. 更新 traders 表的列名
	_, err = d.db.Exec(`ALTER TABLE traders RENAME COLUMN exchange_id TO exchange_id_old`)
	if err != nil {
		return fmt.Errorf("重命名旧列失败: %w", err)
	}
	_, err = d.db.Exec(`ALTER TABLE traders RENAME COLUMN exchange_id_new TO exchange_id`)
	if err != nil {
		return fmt.Errorf("重命名新列失败: %w", err)
	}

	// 7. 重新创建触发器
	_, err = d.db.Exec(`
		CREATE TRIGGER IF NOT EXISTS update_exchanges_updated_at
			AFTER UPDATE ON exchanges
			BEGIN
				UPDATE exchanges SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
			END
	`)
	if err != nil {
		return fmt.Errorf("创建触发器失败: %w", err)
	}

	log.Printf("  ✅ exchanges 表迁移完成，共迁移 %d 条记录", len(oldToNewID))
	return nil
}

// User 用户配置
type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"` // 不返回到前端
	OTPSecret    string    `json:"-"` // 不返回到前端
	OTPVerified  bool      `json:"otp_verified"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// AIModelConfig AI模型配置
type AIModelConfig struct {
	ID              int       `json:"id"`       // 自增ID（主键）
	ModelID         string    `json:"model_id"` // 模型类型ID（例如 "deepseek"）
	UserID          string    `json:"user_id"`
	DisplayName     string    `json:"display_name"` // 用户自定义显示名称
	Name            string    `json:"name"`
	Provider        string    `json:"provider"`
	Enabled         bool      `json:"enabled"`
	APIKey          string    `json:"apiKey"`
	CustomAPIURL    string    `json:"customApiUrl"`
	CustomModelName string    `json:"customModelName"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// ExchangeConfig 交易所配置
type ExchangeConfig struct {
	ID          int    `json:"id"`          // 自增ID（主键）
	ExchangeID  string `json:"exchange_id"` // 交易所类型ID（例如 "binance"）
	UserID      string `json:"user_id"`
	DisplayName string `json:"display_name"` // 用户自定义显示名称
	Name        string `json:"name"`
	Type        string `json:"type"`
	Enabled     bool   `json:"enabled"`
	APIKey      string `json:"apiKey"`    // For Binance: API Key; For Hyperliquid: Agent Private Key (should have ~0 balance)
	SecretKey   string `json:"secretKey"` // For Binance: Secret Key; Not used for Hyperliquid
	Testnet     bool   `json:"testnet"`
	// Hyperliquid Agent Wallet configuration (following official best practices)
	// Reference: https://hyperliquid.gitbook.io/hyperliquid-docs/for-developers/api/nonces-and-api-wallets
	HyperliquidWalletAddr string `json:"hyperliquidWalletAddr"` // Main Wallet Address (holds funds, never expose private key)
	// Aster 特定字段
	AsterUser       string    `json:"asterUser"`
	AsterSigner     string    `json:"asterSigner"`
	AsterPrivateKey string    `json:"asterPrivateKey"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// TraderRecord 交易员配置（数据库实体）
type TraderRecord struct {
	ID                   string    `json:"id"`
	UserID               string    `json:"user_id"`
	Name                 string    `json:"name"`
	AIModelID            int       `json:"ai_model_id"` // 外键：指向 ai_models.id
	ExchangeID           int       `json:"exchange_id"` // 外键：指向 exchanges.id
	InitialBalance       float64   `json:"initial_balance"`
	ScanIntervalMinutes  int       `json:"scan_interval_minutes"`
	IsRunning            bool      `json:"is_running"`
	BTCETHLeverage       int       `json:"btc_eth_leverage"`       // BTC/ETH杠杆倍数
	AltcoinLeverage      int       `json:"altcoin_leverage"`       // 山寨币杠杆倍数
	TradingSymbols       string    `json:"trading_symbols"`        // 交易币种，逗号分隔
	UseCoinPool          bool      `json:"use_coin_pool"`          // 是否使用COIN POOL信号源
	UseOITop             bool      `json:"use_oi_top"`             // 是否使用OI TOP信号源
	CustomPrompt         string    `json:"custom_prompt"`          // 自定义交易策略prompt
	OverrideBasePrompt   bool      `json:"override_base_prompt"`   // 是否覆盖基础prompt
	SystemPromptTemplate string    `json:"system_prompt_template"` // 系统提示词模板名称
	IsCrossMargin        bool      `json:"is_cross_margin"`        // 是否为全仓模式（true=全仓，false=逐仓）
	TakerFeeRate         float64   `json:"taker_fee_rate"`         // Taker fee rate, default 0.0004
	MakerFeeRate         float64   `json:"maker_fee_rate"`         // Maker fee rate, default 0.0002
	OrderStrategy        string    `json:"order_strategy"`         // Order strategy: "market_only", "conservative_hybrid", "limit_only"
	LimitPriceOffset     float64   `json:"limit_price_offset"`     // Limit order price offset percentage (e.g., -0.03 for -0.03%)
	LimitTimeoutSeconds  int       `json:"limit_timeout_seconds"`  // Timeout in seconds before converting to market order (default: 60)
	Timeframes           string    `json:"timeframes"`             // 时间线选择 (逗号分隔，例如: "1m,4h,1d")
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

// UserSignalSource 用户信号源配置
type UserSignalSource struct {
	ID          int       `json:"id"`
	UserID      string    `json:"user_id"`
	CoinPoolURL string    `json:"coin_pool_url"`
	OITopURL    string    `json:"oi_top_url"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// GenerateOTPSecret 生成OTP密钥
func GenerateOTPSecret() (string, error) {
	secret := make([]byte, 20)
	_, err := rand.Read(secret)
	if err != nil {
		return "", err
	}
	return base32.StdEncoding.EncodeToString(secret), nil
}

// CreateUser 创建用户
func (d *Database) CreateUser(user *User) error {
	_, err := d.db.Exec(`
		INSERT INTO users (id, email, password_hash, otp_secret, otp_verified)
		VALUES (?, ?, ?, ?, ?)
	`, user.ID, user.Email, user.PasswordHash, user.OTPSecret, user.OTPVerified)
	return err
}

// EnsureAdminUser 确保admin用户存在（用于管理员模式）
func (d *Database) EnsureAdminUser() error {
	// 检查admin用户是否已存在
	var count int
	err := d.db.QueryRow(`SELECT COUNT(*) FROM users WHERE id = 'admin'`).Scan(&count)
	if err != nil {
		return err
	}

	// 如果已存在，直接返回
	if count > 0 {
		return nil
	}

	// 创建admin用户（密码为空，因为管理员模式下不需要密码）
	adminUser := &User{
		ID:           "admin",
		Email:        "admin@localhost",
		PasswordHash: "", // 管理员模式下不使用密码
		OTPSecret:    "",
		OTPVerified:  true,
	}

	return d.CreateUser(adminUser)
}

// GetUserByEmail 通过邮箱获取用户
func (d *Database) GetUserByEmail(email string) (*User, error) {
	var user User
	err := d.db.QueryRow(`
		SELECT id, email, password_hash, otp_secret, otp_verified, created_at, updated_at
		FROM users WHERE email = ?
	`, email).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.OTPSecret,
		&user.OTPVerified, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetUserByID 通过ID获取用户
func (d *Database) GetUserByID(userID string) (*User, error) {
	var user User
	err := d.db.QueryRow(`
		SELECT id, email, password_hash, otp_secret, otp_verified, created_at, updated_at
		FROM users WHERE id = ?
	`, userID).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.OTPSecret,
		&user.OTPVerified, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetAllUsers 获取所有用户ID列表
func (d *Database) GetAllUsers() ([]string, error) {
	rows, err := d.db.Query(`SELECT id FROM users ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var userIDs []string
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			return nil, err
		}
		userIDs = append(userIDs, userID)
	}
	return userIDs, nil
}

// UpdateUserOTPVerified 更新用户OTP验证状态
func (d *Database) UpdateUserOTPVerified(userID string, verified bool) error {
	_, err := d.db.Exec(`UPDATE users SET otp_verified = ? WHERE id = ?`, verified, userID)
	return err
}

// UpdateUserPassword 更新用户密码
func (d *Database) UpdateUserPassword(userID, passwordHash string) error {
	_, err := d.db.Exec(`
		UPDATE users
		SET password_hash = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, passwordHash, userID)
	return err
}

// GetAIModels 获取用户的AI模型配置
func (d *Database) GetAIModels(userID string) ([]*AIModelConfig, error) {
	rows, err := d.db.Query(`
		SELECT id, model_id, user_id, name, provider, enabled, api_key,
		       COALESCE(custom_api_url, '') as custom_api_url,
		       COALESCE(custom_model_name, '') as custom_model_name,
		       created_at, updated_at
		FROM ai_models WHERE user_id = ? ORDER BY id
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// 初始化为空切片而不是nil，确保JSON序列化为[]而不是null
	models := make([]*AIModelConfig, 0)
	for rows.Next() {
		var model AIModelConfig
		err := rows.Scan(
			&model.ID, &model.ModelID, &model.UserID, &model.Name, &model.Provider,
			&model.Enabled, &model.APIKey, &model.CustomAPIURL, &model.CustomModelName,
			&model.CreatedAt, &model.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		// 解密API Key
		model.APIKey = d.decryptSensitiveData(model.APIKey)
		models = append(models, &model)
	}

	return models, nil
}

// UpdateAIModel 更新AI模型配置，如果不存在则创建用户特定配置
func (d *Database) UpdateAIModel(userID, id string, enabled bool, apiKey, customAPIURL, customModelName string) error {
	// 先尝试精确匹配 model_id（新版逻辑，支持多个相同 provider 的模型）
	var existingModelID string
	err := d.db.QueryRow(`
		SELECT model_id FROM ai_models WHERE user_id = ? AND model_id = ? LIMIT 1
	`, userID, id).Scan(&existingModelID)

	if err == nil {
		// 找到了现有配置（精确匹配 model_id），更新它
		encryptedAPIKey := d.encryptSensitiveData(apiKey)
		_, err = d.db.Exec(`
			UPDATE ai_models SET enabled = ?, api_key = ?, custom_api_url = ?, custom_model_name = ?, updated_at = datetime('now')
			WHERE model_id = ? AND user_id = ?
		`, enabled, encryptedAPIKey, customAPIURL, customModelName, existingModelID, userID)
		return err
	}

	// model_id 不存在，尝试兼容旧逻辑：将 id 作为 provider 查找
	provider := id
	err = d.db.QueryRow(`
		SELECT model_id FROM ai_models WHERE user_id = ? AND provider = ? LIMIT 1
	`, userID, provider).Scan(&existingModelID)

	if err == nil {
		// 找到了现有配置（通过 provider 匹配，兼容旧版），更新它
		log.Printf("⚠️  使用旧版 provider 匹配更新模型: %s -> %s", provider, existingModelID)
		encryptedAPIKey := d.encryptSensitiveData(apiKey)
		_, err = d.db.Exec(`
			UPDATE ai_models SET enabled = ?, api_key = ?, custom_api_url = ?, custom_model_name = ?, updated_at = datetime('now')
			WHERE model_id = ? AND user_id = ?
		`, enabled, encryptedAPIKey, customAPIURL, customModelName, existingModelID, userID)
		return err
	}

	// 没有找到任何现有配置，创建新的
	// 推断 provider（从 id 中提取，或者直接使用 id）
	if provider == id && (provider == "deepseek" || provider == "qwen") {
		// id 本身就是 provider
		provider = id
	} else {
		// 从 id 中提取 provider（假设格式是 userID_provider 或 timestamp_userID_provider）
		parts := strings.Split(id, "_")
		if len(parts) >= 2 {
			provider = parts[len(parts)-1] // 取最后一部分作为 provider
		} else {
			provider = id
		}
	}

	// 获取模型的基本信息
	var name string
	err = d.db.QueryRow(`
		SELECT name FROM ai_models WHERE provider = ? LIMIT 1
	`, provider).Scan(&name)
	if err != nil {
		// 如果找不到基本信息，使用默认值
		if provider == "deepseek" {
			name = "DeepSeek AI"
		} else if provider == "qwen" {
			name = "Qwen AI"
		} else {
			name = provider + " AI"
		}
	}

	// 如果传入的 ID 已经是完整格式（如 "admin_deepseek_custom1"），直接使用
	// 否则生成新的 ID
	newModelID := id
	if id == provider {
		// id 就是 provider，生成新的用户特定 ID
		newModelID = fmt.Sprintf("%s_%s", userID, provider)
	}

	log.Printf("✓ 创建新的 AI 模型配置: ID=%s, Provider=%s, Name=%s", newModelID, provider, name)
	encryptedAPIKey := d.encryptSensitiveData(apiKey)
	_, err = d.db.Exec(`
		INSERT INTO ai_models (model_id, user_id, name, provider, enabled, api_key, custom_api_url, custom_model_name, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))
	`, newModelID, userID, name, provider, enabled, encryptedAPIKey, customAPIURL, customModelName)

	return err
}

// GetExchanges 获取用户的交易所配置
func (d *Database) GetExchanges(userID string) ([]*ExchangeConfig, error) {
	rows, err := d.db.Query(`
		SELECT id, exchange_id, user_id, name, type, enabled, api_key, secret_key, testnet,
		       COALESCE(hyperliquid_wallet_addr, '') as hyperliquid_wallet_addr,
		       COALESCE(aster_user, '') as aster_user,
		       COALESCE(aster_signer, '') as aster_signer,
		       COALESCE(aster_private_key, '') as aster_private_key,
		       created_at, updated_at
		FROM exchanges WHERE user_id = ? ORDER BY id
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// 初始化为空切片而不是nil，确保JSON序列化为[]而不是null
	exchanges := make([]*ExchangeConfig, 0)
	for rows.Next() {
		var exchange ExchangeConfig
		err := rows.Scan(
			&exchange.ID, &exchange.ExchangeID, &exchange.UserID, &exchange.Name, &exchange.Type,
			&exchange.Enabled, &exchange.APIKey, &exchange.SecretKey, &exchange.Testnet,
			&exchange.HyperliquidWalletAddr, &exchange.AsterUser,
			&exchange.AsterSigner, &exchange.AsterPrivateKey,
			&exchange.CreatedAt, &exchange.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		// 解密敏感字段
		exchange.APIKey = d.decryptSensitiveData(exchange.APIKey)
		exchange.SecretKey = d.decryptSensitiveData(exchange.SecretKey)
		exchange.AsterPrivateKey = d.decryptSensitiveData(exchange.AsterPrivateKey)

		exchanges = append(exchanges, &exchange)
	}

	return exchanges, nil
}

// UpdateExchange 更新交易所配置，如果不存在则创建用户特定配置
// 🔒 安全特性：空值不会覆盖现有的敏感字段（api_key, secret_key, aster_private_key）
func (d *Database) UpdateExchange(userID, id string, enabled bool, apiKey, secretKey string, testnet bool, hyperliquidWalletAddr, asterUser, asterSigner, asterPrivateKey string) error {
	log.Printf("🔧 UpdateExchange: userID=%s, id=%s, enabled=%v", userID, id, enabled)

	// 构建动态 UPDATE SET 子句
	// 基础字段：总是更新
	setClauses := []string{
		"enabled = ?",
		"testnet = ?",
		"hyperliquid_wallet_addr = ?",
		"aster_user = ?",
		"aster_signer = ?",
		"updated_at = datetime('now')",
	}
	args := []interface{}{enabled, testnet, hyperliquidWalletAddr, asterUser, asterSigner}

	// 🔒 敏感字段：只在非空时更新（保护现有数据）
	if apiKey != "" {
		encryptedAPIKey := d.encryptSensitiveData(apiKey)
		setClauses = append(setClauses, "api_key = ?")
		args = append(args, encryptedAPIKey)
	}

	if secretKey != "" {
		encryptedSecretKey := d.encryptSensitiveData(secretKey)
		setClauses = append(setClauses, "secret_key = ?")
		args = append(args, encryptedSecretKey)
	}

	if asterPrivateKey != "" {
		encryptedAsterPrivateKey := d.encryptSensitiveData(asterPrivateKey)
		setClauses = append(setClauses, "aster_private_key = ?")
		args = append(args, encryptedAsterPrivateKey)
	}

	// WHERE 条件
	args = append(args, id, userID)

	// 构建完整的 UPDATE 语句
	query := fmt.Sprintf(`
		UPDATE exchanges SET %s
		WHERE exchange_id = ? AND user_id = ?
	`, strings.Join(setClauses, ", "))

	// 执行更新
	result, err := d.db.Exec(query, args...)
	if err != nil {
		log.Printf("❌ UpdateExchange: 更新失败: %v", err)
		return err
	}

	// 检查是否有行被更新
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("❌ UpdateExchange: 获取影响行数失败: %v", err)
		return err
	}

	log.Printf("📊 UpdateExchange: 影响行数 = %d", rowsAffected)

	// 如果没有行被更新，说明用户没有这个交易所的配置，需要创建
	if rowsAffected == 0 {
		log.Printf("💡 UpdateExchange: 没有现有记录，创建新记录")

		// 根据交易所ID确定基本信息
		var name, typ string
		if id == "binance" {
			name = "Binance Futures"
			typ = "cex"
		} else if id == "hyperliquid" {
			name = "Hyperliquid"
			typ = "dex"
		} else if id == "aster" {
			name = "Aster DEX"
			typ = "dex"
		} else {
			name = id + " Exchange"
			typ = "cex"
		}

		log.Printf("🆕 UpdateExchange: 创建新记录 ID=%s, name=%s, type=%s", id, name, typ)

		// 创建用户特定的配置，使用原始的交易所ID
		// 加密敏感字段
		encryptedAPIKey := d.encryptSensitiveData(apiKey)
		encryptedSecretKey := d.encryptSensitiveData(secretKey)
		encryptedAsterPrivateKey := d.encryptSensitiveData(asterPrivateKey)

		_, err = d.db.Exec(`
			INSERT INTO exchanges (exchange_id, user_id, name, type, enabled, api_key, secret_key, testnet,
			                       hyperliquid_wallet_addr, aster_user, aster_signer, aster_private_key, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))
		`, id, userID, name, typ, enabled, encryptedAPIKey, encryptedSecretKey, testnet, hyperliquidWalletAddr, asterUser, asterSigner, encryptedAsterPrivateKey)

		if err != nil {
			log.Printf("❌ UpdateExchange: 创建记录失败: %v", err)
		} else {
			log.Printf("✅ UpdateExchange: 创建记录成功")
		}
		return err
	}

	log.Printf("✅ UpdateExchange: 更新现有记录成功")
	return nil
}

// CreateAIModel 创建AI模型配置
func (d *Database) CreateAIModel(userID, id, name, provider string, enabled bool, apiKey, customAPIURL string) error {
	_, err := d.db.Exec(`
		INSERT OR IGNORE INTO ai_models (model_id, user_id, name, provider, enabled, api_key, custom_api_url)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, id, userID, name, provider, enabled, apiKey, customAPIURL)
	return err
}

// CreateExchange 创建交易所配置
func (d *Database) CreateExchange(userID, id, name, typ string, enabled bool, apiKey, secretKey string, testnet bool, hyperliquidWalletAddr, asterUser, asterSigner, asterPrivateKey string) error {
	// 加密敏感字段
	encryptedAPIKey := d.encryptSensitiveData(apiKey)
	encryptedSecretKey := d.encryptSensitiveData(secretKey)
	encryptedAsterPrivateKey := d.encryptSensitiveData(asterPrivateKey)

	_, err := d.db.Exec(`
		INSERT OR IGNORE INTO exchanges (exchange_id, user_id, name, type, enabled, api_key, secret_key, testnet, hyperliquid_wallet_addr, aster_user, aster_signer, aster_private_key)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, id, userID, name, typ, enabled, encryptedAPIKey, encryptedSecretKey, testnet, hyperliquidWalletAddr, asterUser, asterSigner, encryptedAsterPrivateKey)
	return err
}

// CreateTrader 创建交易员
func (d *Database) CreateTrader(trader *TraderRecord) error {
	_, err := d.db.Exec(`
		INSERT INTO traders (id, user_id, name, ai_model_id, exchange_id, initial_balance, scan_interval_minutes, is_running, btc_eth_leverage, altcoin_leverage, trading_symbols, use_coin_pool, use_oi_top, custom_prompt, override_base_prompt, system_prompt_template, is_cross_margin, taker_fee_rate, maker_fee_rate, order_strategy, limit_price_offset, limit_timeout_seconds, timeframes)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, trader.ID, trader.UserID, trader.Name, trader.AIModelID, trader.ExchangeID, trader.InitialBalance, trader.ScanIntervalMinutes, trader.IsRunning, trader.BTCETHLeverage, trader.AltcoinLeverage, trader.TradingSymbols, trader.UseCoinPool, trader.UseOITop, trader.CustomPrompt, trader.OverrideBasePrompt, trader.SystemPromptTemplate, trader.IsCrossMargin, trader.TakerFeeRate, trader.MakerFeeRate, trader.OrderStrategy, trader.LimitPriceOffset, trader.LimitTimeoutSeconds, trader.Timeframes)
	return err
}

// GetTraders 获取用户的交易员
func (d *Database) GetTraders(userID string) ([]*TraderRecord, error) {
	rows, err := d.db.Query(`
		SELECT id, user_id, name, ai_model_id, exchange_id, initial_balance, scan_interval_minutes, is_running,
		       COALESCE(btc_eth_leverage, 5) as btc_eth_leverage, COALESCE(altcoin_leverage, 5) as altcoin_leverage,
		       COALESCE(trading_symbols, '') as trading_symbols,
		       COALESCE(use_coin_pool, 0) as use_coin_pool, COALESCE(use_oi_top, 0) as use_oi_top,
		       COALESCE(custom_prompt, '') as custom_prompt, COALESCE(override_base_prompt, 0) as override_base_prompt,
		       COALESCE(system_prompt_template, 'default') as system_prompt_template,
		       COALESCE(is_cross_margin, 1) as is_cross_margin,
		       COALESCE(taker_fee_rate, 0.0004) as taker_fee_rate, COALESCE(maker_fee_rate, 0.0002) as maker_fee_rate,
		       COALESCE(order_strategy, 'conservative_hybrid') as order_strategy,
		       COALESCE(limit_price_offset, -0.03) as limit_price_offset,
		       COALESCE(limit_timeout_seconds, 60) as limit_timeout_seconds,
		       COALESCE(timeframes, '4h') as timeframes,
		       created_at, updated_at
		FROM traders WHERE user_id = ? ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var traders []*TraderRecord
	for rows.Next() {
		var trader TraderRecord
		err := rows.Scan(
			&trader.ID, &trader.UserID, &trader.Name, &trader.AIModelID, &trader.ExchangeID,
			&trader.InitialBalance, &trader.ScanIntervalMinutes, &trader.IsRunning,
			&trader.BTCETHLeverage, &trader.AltcoinLeverage, &trader.TradingSymbols,
			&trader.UseCoinPool, &trader.UseOITop,
			&trader.CustomPrompt, &trader.OverrideBasePrompt, &trader.SystemPromptTemplate,
			&trader.IsCrossMargin,
			&trader.TakerFeeRate, &trader.MakerFeeRate,
			&trader.OrderStrategy, &trader.LimitPriceOffset, &trader.LimitTimeoutSeconds,
			&trader.Timeframes,
			&trader.CreatedAt, &trader.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		traders = append(traders, &trader)
	}

	return traders, nil
}

// UpdateTraderStatus 更新交易员状态
func (d *Database) UpdateTraderStatus(userID, id string, isRunning bool) error {
	_, err := d.db.Exec(`UPDATE traders SET is_running = ? WHERE id = ? AND user_id = ?`, isRunning, id, userID)
	return err
}

// UpdateTrader 更新交易员配置
func (d *Database) UpdateTrader(trader *TraderRecord) error {
	_, err := d.db.Exec(`
		UPDATE traders SET
			name = ?, ai_model_id = ?, exchange_id = ?,
			scan_interval_minutes = ?, btc_eth_leverage = ?, altcoin_leverage = ?,
			trading_symbols = ?, custom_prompt = ?, override_base_prompt = ?,
			system_prompt_template = ?, is_cross_margin = ?, taker_fee_rate = ?, maker_fee_rate = ?,
			order_strategy = ?, limit_price_offset = ?, limit_timeout_seconds = ?, timeframes = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND user_id = ?
	`, trader.Name, trader.AIModelID, trader.ExchangeID,
		trader.ScanIntervalMinutes, trader.BTCETHLeverage, trader.AltcoinLeverage,
		trader.TradingSymbols, trader.CustomPrompt, trader.OverrideBasePrompt,
		trader.SystemPromptTemplate, trader.IsCrossMargin, trader.TakerFeeRate, trader.MakerFeeRate,
		trader.OrderStrategy, trader.LimitPriceOffset, trader.LimitTimeoutSeconds, trader.Timeframes,
		trader.ID, trader.UserID)
	return err
}

// UpdateTraderCustomPrompt 更新交易员自定义Prompt
func (d *Database) UpdateTraderCustomPrompt(userID, id string, customPrompt string, overrideBase bool) error {
	_, err := d.db.Exec(`UPDATE traders SET custom_prompt = ?, override_base_prompt = ? WHERE id = ? AND user_id = ?`, customPrompt, overrideBase, id, userID)
	return err
}

// UpdateTraderInitialBalance 更新交易员初始余额（仅支持手动更新）
// ⚠️ 注意：系统不会自动调用此方法，仅供用户在充值/提现后手动同步使用
func (d *Database) UpdateTraderInitialBalance(userID, id string, newBalance float64) error {
	_, err := d.db.Exec(`UPDATE traders SET initial_balance = ? WHERE id = ? AND user_id = ?`, newBalance, id, userID)
	return err
}

// DeleteTrader 删除交易员
func (d *Database) DeleteTrader(userID, id string) error {
	_, err := d.db.Exec(`DELETE FROM traders WHERE id = ? AND user_id = ?`, id, userID)
	return err
}

// GetTraderConfig 获取交易员完整配置（包含AI模型和交易所信息）
func (d *Database) GetTraderConfig(userID, traderID string) (*TraderRecord, *AIModelConfig, *ExchangeConfig, error) {
	var trader TraderRecord
	var aiModel AIModelConfig
	var exchange ExchangeConfig

	err := d.db.QueryRow(`
		SELECT
			t.id, t.user_id, t.name, t.ai_model_id, t.exchange_id, t.initial_balance, t.scan_interval_minutes, t.is_running,
			COALESCE(t.btc_eth_leverage, 5) as btc_eth_leverage,
			COALESCE(t.altcoin_leverage, 5) as altcoin_leverage,
			COALESCE(t.trading_symbols, '') as trading_symbols,
			COALESCE(t.use_coin_pool, 0) as use_coin_pool,
			COALESCE(t.use_oi_top, 0) as use_oi_top,
			COALESCE(t.custom_prompt, '') as custom_prompt,
			COALESCE(t.override_base_prompt, 0) as override_base_prompt,
			COALESCE(t.system_prompt_template, 'default') as system_prompt_template,
			COALESCE(t.is_cross_margin, 1) as is_cross_margin,
			COALESCE(t.taker_fee_rate, 0.0004) as taker_fee_rate,
			COALESCE(t.maker_fee_rate, 0.0002) as maker_fee_rate,
			COALESCE(t.order_strategy, 'conservative_hybrid') as order_strategy,
			COALESCE(t.limit_price_offset, -0.03) as limit_price_offset,
			COALESCE(t.limit_timeout_seconds, 60) as limit_timeout_seconds,
			COALESCE(t.timeframes, '4h') as timeframes,
			t.created_at, t.updated_at,
			a.id, a.model_id, a.user_id, a.name, a.provider, a.enabled, a.api_key,
			COALESCE(a.custom_api_url, '') as custom_api_url,
			COALESCE(a.custom_model_name, '') as custom_model_name,
			a.created_at, a.updated_at,
			e.id, e.exchange_id, e.user_id, e.name, e.type, e.enabled, e.api_key, e.secret_key, e.testnet,
			COALESCE(e.hyperliquid_wallet_addr, '') as hyperliquid_wallet_addr,
			COALESCE(e.aster_user, '') as aster_user,
			COALESCE(e.aster_signer, '') as aster_signer,
			COALESCE(e.aster_private_key, '') as aster_private_key,
			e.created_at, e.updated_at
		FROM traders t
		JOIN ai_models a ON t.ai_model_id = a.id
		JOIN exchanges e ON t.exchange_id = e.id
		WHERE t.id = ? AND t.user_id = ?
	`, traderID, userID).Scan(
		&trader.ID, &trader.UserID, &trader.Name, &trader.AIModelID, &trader.ExchangeID,
		&trader.InitialBalance, &trader.ScanIntervalMinutes, &trader.IsRunning,
		&trader.BTCETHLeverage, &trader.AltcoinLeverage, &trader.TradingSymbols,
		&trader.UseCoinPool, &trader.UseOITop,
		&trader.CustomPrompt, &trader.OverrideBasePrompt, &trader.SystemPromptTemplate,
		&trader.IsCrossMargin,
		&trader.TakerFeeRate, &trader.MakerFeeRate,
		&trader.OrderStrategy, &trader.LimitPriceOffset, &trader.LimitTimeoutSeconds,
		&trader.Timeframes,
		&trader.CreatedAt, &trader.UpdatedAt,
		&aiModel.ID, &aiModel.ModelID, &aiModel.UserID, &aiModel.Name, &aiModel.Provider, &aiModel.Enabled, &aiModel.APIKey,
		&aiModel.CustomAPIURL, &aiModel.CustomModelName,
		&aiModel.CreatedAt, &aiModel.UpdatedAt,
		&exchange.ID, &exchange.ExchangeID, &exchange.UserID, &exchange.Name, &exchange.Type, &exchange.Enabled,
		&exchange.APIKey, &exchange.SecretKey, &exchange.Testnet,
		&exchange.HyperliquidWalletAddr, &exchange.AsterUser, &exchange.AsterSigner, &exchange.AsterPrivateKey,
		&exchange.CreatedAt, &exchange.UpdatedAt,
	)

	if err != nil {
		return nil, nil, nil, err
	}

	// 解密敏感数据
	aiModel.APIKey = d.decryptSensitiveData(aiModel.APIKey)
	exchange.APIKey = d.decryptSensitiveData(exchange.APIKey)
	exchange.SecretKey = d.decryptSensitiveData(exchange.SecretKey)
	exchange.AsterPrivateKey = d.decryptSensitiveData(exchange.AsterPrivateKey)

	return &trader, &aiModel, &exchange, nil
}

// GetSystemConfig 获取系统配置
func (d *Database) GetSystemConfig(key string) (string, error) {
	var value string
	err := d.db.QueryRow(`SELECT value FROM system_config WHERE key = ?`, key).Scan(&value)
	return value, err
}

// SetSystemConfig 设置系统配置
func (d *Database) SetSystemConfig(key, value string) error {
	_, err := d.db.Exec(`
		INSERT OR REPLACE INTO system_config (key, value) VALUES (?, ?)
	`, key, value)
	return err
}

// CreateUserSignalSource 创建用户信号源配置
func (d *Database) CreateUserSignalSource(userID, coinPoolURL, oiTopURL string) error {
	_, err := d.db.Exec(`
		INSERT OR REPLACE INTO user_signal_sources (user_id, coin_pool_url, oi_top_url, updated_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)
	`, userID, coinPoolURL, oiTopURL)
	return err
}

// GetUserSignalSource 获取用户信号源配置
func (d *Database) GetUserSignalSource(userID string) (*UserSignalSource, error) {
	var source UserSignalSource
	err := d.db.QueryRow(`
		SELECT id, user_id, coin_pool_url, oi_top_url, created_at, updated_at
		FROM user_signal_sources WHERE user_id = ?
	`, userID).Scan(
		&source.ID, &source.UserID, &source.CoinPoolURL, &source.OITopURL,
		&source.CreatedAt, &source.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &source, nil
}

// UpdateUserSignalSource 更新用户信号源配置
func (d *Database) UpdateUserSignalSource(userID, coinPoolURL, oiTopURL string) error {
	_, err := d.db.Exec(`
		UPDATE user_signal_sources SET coin_pool_url = ?, oi_top_url = ?, updated_at = CURRENT_TIMESTAMP
		WHERE user_id = ?
	`, coinPoolURL, oiTopURL, userID)
	return err
}

// GetCustomCoins 获取所有交易员自定义币种 / Get all trader-customized currencies
func (d *Database) GetCustomCoins() []string {
	var symbol string
	var symbols []string
	_ = d.db.QueryRow(`
		SELECT GROUP_CONCAT(custom_coins , ',') as symbol
		FROM main.traders where custom_coins != ''
	`).Scan(&symbol)
	// 检测用户是否未配置币种 - 兼容性
	if symbol == "" {
		symbolJSON, _ := d.GetSystemConfig("default_coins")
		if err := json.Unmarshal([]byte(symbolJSON), &symbols); err != nil {
			log.Printf("⚠️  解析default_coins配置失败: %v，使用硬编码默认值", err)
			symbols = []string{"BTCUSDT", "ETHUSDT", "SOLUSDT", "BNBUSDT"}
		}
	}
	// filter Symbol
	for _, s := range strings.Split(symbol, ",") {
		if s == "" {
			continue
		}
		coin := market.Normalize(s)
		if !slices.Contains(symbols, coin) {
			symbols = append(symbols, coin)
		}
	}
	return symbols
}

// GetAllTimeframes 获取所有交易员配置的时间线并集 / Get union of all trader timeframes
func (d *Database) GetAllTimeframes() []string {
	rows, err := d.db.Query(`
		SELECT DISTINCT timeframes
		FROM traders
		WHERE timeframes != '' AND is_running = 1
	`)
	if err != nil {
		log.Printf("查询 trader timeframes 失败: %v", err)
		return []string{"4h"} // 默认返回 4h
	}
	defer rows.Close()

	timeframeSet := make(map[string]bool)
	for rows.Next() {
		var timeframes string
		if err := rows.Scan(&timeframes); err != nil {
			continue
		}
		// 解析逗号分隔的时间线
		for _, tf := range strings.Split(timeframes, ",") {
			tf = strings.TrimSpace(tf)
			if tf != "" {
				timeframeSet[tf] = true
			}
		}
	}

	// 转换为切片
	result := make([]string, 0, len(timeframeSet))
	for tf := range timeframeSet {
		result = append(result, tf)
	}

	// 如果没有配置，返回默认值
	if len(result) == 0 {
		return []string{"15m", "1h", "4h"}
	}

	log.Printf("📊 从数据库加载所有活跃 trader 的时间线: %v", result)
	return result
}

// Close 关闭数据库连接
func (d *Database) Close() error {
	return d.db.Close()
}

// LoadBetaCodesFromFile 从文件加载内测码到数据库
func (d *Database) LoadBetaCodesFromFile(filePath string) error {
	// 读取文件内容
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("读取内测码文件失败: %w", err)
	}

	// 按行分割内测码
	lines := strings.Split(string(content), "\n")
	var codes []string
	for _, line := range lines {
		code := strings.TrimSpace(line)
		if code != "" && !strings.HasPrefix(code, "#") {
			codes = append(codes, code)
		}
	}

	// 批量插入内测码
	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("开始事务失败: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT OR IGNORE INTO beta_codes (code) VALUES (?)`)
	if err != nil {
		return fmt.Errorf("准备语句失败: %w", err)
	}
	defer stmt.Close()

	insertedCount := 0
	for _, code := range codes {
		result, err := stmt.Exec(code)
		if err != nil {
			log.Printf("插入内测码 %s 失败: %v", code, err)
			continue
		}

		if rowsAffected, _ := result.RowsAffected(); rowsAffected > 0 {
			insertedCount++
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交事务失败: %w", err)
	}

	log.Printf("✅ 成功加载 %d 个内测码到数据库 (总计 %d 个)", insertedCount, len(codes))
	return nil
}

// ValidateBetaCode 验证内测码是否有效且未使用
func (d *Database) ValidateBetaCode(code string) (bool, error) {
	var used bool
	err := d.db.QueryRow(`SELECT used FROM beta_codes WHERE code = ?`, code).Scan(&used)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil // 内测码不存在
		}
		return false, err
	}
	return !used, nil // 内测码存在且未使用
}

// UseBetaCode 使用内测码（标记为已使用）
func (d *Database) UseBetaCode(code, userEmail string) error {
	result, err := d.db.Exec(`
		UPDATE beta_codes SET used = 1, used_by = ?, used_at = CURRENT_TIMESTAMP 
		WHERE code = ? AND used = 0
	`, userEmail, code)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("内测码无效或已被使用")
	}

	return nil
}

// GetBetaCodeStats 获取内测码统计信息
func (d *Database) GetBetaCodeStats() (total, used int, err error) {
	err = d.db.QueryRow(`SELECT COUNT(*) FROM beta_codes`).Scan(&total)
	if err != nil {
		return 0, 0, err
	}

	err = d.db.QueryRow(`SELECT COUNT(*) FROM beta_codes WHERE used = 1`).Scan(&used)
	if err != nil {
		return 0, 0, err
	}

	return total, used, nil
}

// SetCryptoService 设置加密服务
func (d *Database) SetCryptoService(cs *crypto.CryptoService) {
	d.cryptoService = cs
}

// encryptSensitiveData 加密敏感数据用于存储
func (d *Database) encryptSensitiveData(plaintext string) string {
	if d.cryptoService == nil || plaintext == "" {
		return plaintext
	}

	encrypted, err := d.cryptoService.EncryptForStorage(plaintext)
	if err != nil {
		log.Printf("⚠️ 加密失败: %v", err)
		return plaintext // 返回明文作为降级处理
	}

	return encrypted
}

// decryptSensitiveData 解密敏感数据
func (d *Database) decryptSensitiveData(encrypted string) string {
	if d.cryptoService == nil || encrypted == "" {
		return encrypted
	}

	// 如果不是加密格式，直接返回
	if !d.cryptoService.IsEncryptedStorageValue(encrypted) {
		return encrypted
	}

	decrypted, err := d.cryptoService.DecryptFromStorage(encrypted)
	if err != nil {
		log.Printf("⚠️ 解密失败: %v", err)
		return encrypted // 返回加密文本作为降级处理
	}

	return decrypted
}

// cleanupLegacyColumns removes legacy _old columns from database (automatic migration)
// This function automatically executes during database initialization to ensure
// existing users can upgrade smoothly without manual intervention
func (d *Database) cleanupLegacyColumns() error {
	// Check if traders table has legacy _old columns
	var hasOldColumns bool
	rows, err := d.db.Query("PRAGMA table_info(traders)")
	if err != nil {
		return fmt.Errorf("failed to check table structure: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, dfltValue, pk interface{}
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dfltValue, &pk); err != nil {
			return fmt.Errorf("failed to read column info: %w", err)
		}
		if name == "ai_model_id_old" || name == "exchange_id_old" {
			hasOldColumns = true
			break
		}
	}

	// If no _old columns exist, skip cleanup
	if !hasOldColumns {
		return nil
	}

	log.Printf("🔄 Detected legacy _old columns, starting automatic cleanup...")

	// Begin transaction
	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Create new traders table without _old columns but WITH all feature columns
	_, err = tx.Exec(`
		CREATE TABLE traders_new (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL DEFAULT 'default',
			name TEXT NOT NULL,
			ai_model_id TEXT NOT NULL,
			exchange_id TEXT NOT NULL,
			initial_balance REAL NOT NULL,
			scan_interval_minutes INTEGER DEFAULT 3,
			is_running BOOLEAN DEFAULT 0,
			btc_eth_leverage INTEGER DEFAULT 5,
			altcoin_leverage INTEGER DEFAULT 5,
			trading_symbols TEXT DEFAULT '',
			use_coin_pool BOOLEAN DEFAULT 0,
			use_oi_top BOOLEAN DEFAULT 0,
			custom_prompt TEXT DEFAULT '',
			override_base_prompt BOOLEAN DEFAULT 0,
			system_prompt_template TEXT DEFAULT 'default',
			is_cross_margin BOOLEAN DEFAULT 1,
			use_default_coins BOOLEAN DEFAULT 1,
			custom_coins TEXT DEFAULT '',
			taker_fee_rate REAL DEFAULT 0.0004,
			maker_fee_rate REAL DEFAULT 0.0002,
			order_strategy TEXT DEFAULT 'conservative_hybrid',
			limit_price_offset REAL DEFAULT -0.03,
			limit_timeout_seconds INTEGER DEFAULT 60,
			timeframes TEXT DEFAULT '4h',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY (ai_model_id) REFERENCES ai_models(id),
			FOREIGN KEY (exchange_id) REFERENCES exchanges(id)
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create new table: %w", err)
	}

	// Migrate data (copy all columns, use COALESCE for nullable fields)
	_, err = tx.Exec(`
		INSERT INTO traders_new (
			id, user_id, name, ai_model_id, exchange_id,
			initial_balance, scan_interval_minutes, is_running,
			btc_eth_leverage, altcoin_leverage, trading_symbols,
			use_coin_pool, use_oi_top,
			custom_prompt, override_base_prompt, system_prompt_template,
			is_cross_margin, use_default_coins, custom_coins,
			taker_fee_rate, maker_fee_rate, order_strategy,
			limit_price_offset, limit_timeout_seconds, timeframes,
			created_at, updated_at
		)
		SELECT
			id, user_id, name, ai_model_id, exchange_id,
			initial_balance, scan_interval_minutes, is_running,
			btc_eth_leverage, altcoin_leverage, trading_symbols,
			use_coin_pool, use_oi_top,
			COALESCE(custom_prompt, ''), COALESCE(override_base_prompt, 0), COALESCE(system_prompt_template, 'default'),
			COALESCE(is_cross_margin, 1), COALESCE(use_default_coins, 1), COALESCE(custom_coins, ''),
			COALESCE(taker_fee_rate, 0.0004), COALESCE(maker_fee_rate, 0.0002), COALESCE(order_strategy, 'conservative_hybrid'),
			COALESCE(limit_price_offset, -0.03), COALESCE(limit_timeout_seconds, 60), COALESCE(timeframes, '4h'),
			created_at, updated_at
		FROM traders
	`)
	if err != nil {
		return fmt.Errorf("failed to migrate data: %w", err)
	}

	// Drop old table
	_, err = tx.Exec("DROP TABLE traders")
	if err != nil {
		return fmt.Errorf("failed to drop old table: %w", err)
	}

	// Rename new table
	_, err = tx.Exec("ALTER TABLE traders_new RENAME TO traders")
	if err != nil {
		return fmt.Errorf("failed to rename table: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("✅ Successfully cleaned up legacy _old columns")
	return nil
}
