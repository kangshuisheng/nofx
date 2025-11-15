package config

import "testing"

func TestGetCustomCoinsFromRunningTraders(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	userID := "test-user-001"
	aiID := ensureTestAIModel(t, db, userID, "model-coins-1")
	exID := ensureTestExchange(t, db, userID, "binance-coins-1")

	createTrader := func(id, symbols string, running bool) {
		tr := &TraderRecord{
			ID:                   id,
			UserID:               userID,
			Name:                 id,
			AIModelID:            aiID,
			ExchangeID:           exID,
			InitialBalance:       1000,
			ScanIntervalMinutes:  5,
			IsRunning:            running,
			TradingSymbols:       symbols,
			SystemPromptTemplate: "default",
		}
		if err := db.CreateTrader(tr); err != nil {
			t.Fatalf("CreateTrader failed: %v", err)
		}
	}

	createTrader("tr-1", "btcUSDT ,  sol", true)
	createTrader("tr-2", "ethusdt, SOLUSDT", true)
	createTrader("tr-3", "dogeusdt", false)

	coins := db.GetCustomCoins()

	expected := map[string]bool{
		"BTCUSDT": true,
		"ETHUSDT": true,
		"SOLUSDT": true,
	}

	if len(coins) != len(expected) {
		t.Fatalf("expected %d coins, got %d: %v", len(expected), len(coins), coins)
	}

	for _, coin := range coins {
		if !expected[coin] {
			t.Fatalf("unexpected coin %s in result %v", coin, coins)
		}
	}
}

func TestGetCustomCoinsFallbackToDefault(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	if err := db.SetSystemConfig("default_coins", `["XRPUSDT","DOGEUSDT"]`); err != nil {
		t.Fatalf("failed to set default coins: %v", err)
	}

	coins := db.GetCustomCoins()
	expected := map[string]bool{
		"XRPUSDT":  true,
		"DOGEUSDT": true,
	}
	if len(coins) != len(expected) {
		t.Fatalf("expected %d default coins, got %d (%v)", len(expected), len(coins), coins)
	}
	for _, coin := range coins {
		if !expected[coin] {
			t.Fatalf("unexpected default coin %s in %v", coin, coins)
		}
	}
}

func ensureTestAIModel(t *testing.T, db *Database, userID, modelID string) int {
	t.Helper()
	if err := db.CreateAIModel(userID, modelID, "Test Model", "deepseek", true, "", ""); err != nil {
		t.Fatalf("CreateAIModel failed: %v", err)
	}
	models, err := db.GetAIModels(userID)
	if err != nil {
		t.Fatalf("GetAIModels failed: %v", err)
	}
	for _, m := range models {
		if m.ModelID == modelID {
			return m.ID
		}
	}
	t.Fatalf("model %s not found", modelID)
	return 0
}

func ensureTestExchange(t *testing.T, db *Database, userID, exchangeID string) int {
	t.Helper()
	if err := db.CreateExchange(userID, exchangeID, "Binance", "cex", true, "key", "secret", false, "", "", "", ""); err != nil {
		t.Fatalf("CreateExchange failed: %v", err)
	}
	exchanges, err := db.GetExchanges(userID)
	if err != nil {
		t.Fatalf("GetExchanges failed: %v", err)
	}
	for _, ex := range exchanges {
		if ex.ExchangeID == exchangeID {
			return ex.ID
		}
	}
	t.Fatalf("exchange %s not found", exchangeID)
	return 0
}
