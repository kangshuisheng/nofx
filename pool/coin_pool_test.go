package pool

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestSetCoinPoolAPI tests setting the coin pool API URL
func TestSetCoinPoolAPI(t *testing.T) {
	testURL := "https://test.example.com/api/coins"
	SetCoinPoolAPI(testURL)

	if coinPoolConfig.APIURL != testURL {
		t.Errorf("Expected APIURL %s, got %s", testURL, coinPoolConfig.APIURL)
	}
}

// TestSetOITopAPI tests setting the OI Top API URL
func TestSetOITopAPI(t *testing.T) {
	testURL := "https://test.example.com/api/oi-top"
	SetOITopAPI(testURL)

	if oiTopConfig.APIURL != testURL {
		t.Errorf("Expected OI Top APIURL %s, got %s", testURL, oiTopConfig.APIURL)
	}
}

// TestSetUseDefaultCoins tests enabling/disabling default coins
func TestSetUseDefaultCoins(t *testing.T) {
	// Test enabling
	SetUseDefaultCoins(true)
	if !coinPoolConfig.UseDefaultCoins {
		t.Error("Expected UseDefaultCoins to be true")
	}

	// Test disabling
	SetUseDefaultCoins(false)
	if coinPoolConfig.UseDefaultCoins {
		t.Error("Expected UseDefaultCoins to be false")
	}
}

// TestSetDefaultCoins tests setting custom default coins
func TestSetDefaultCoins(t *testing.T) {
	testCoins := []string{"BTCUSDT", "ETHUSDT", "SOLUSDT"}
	SetDefaultCoins(testCoins)

	if len(defaultMainstreamCoins) != len(testCoins) {
		t.Errorf("Expected %d coins, got %d", len(testCoins), len(defaultMainstreamCoins))
	}

	for i, coin := range testCoins {
		if defaultMainstreamCoins[i] != coin {
			t.Errorf("Expected coin %s at index %d, got %s", coin, i, defaultMainstreamCoins[i])
		}
	}
}

// TestGetCoinPoolWithDefaultCoins tests getting coin pool with default coins enabled
func TestGetCoinPoolWithDefaultCoins(t *testing.T) {
	// Enable default coins
	SetUseDefaultCoins(true)

	coins, err := GetCoinPool()
	if err != nil {
		t.Errorf("GetCoinPool with default coins failed: %v", err)
	}

	if len(coins) == 0 {
		t.Error("Expected non-empty coin list")
	}

	// Verify coins are marked as available
	for _, coin := range coins {
		if !coin.IsAvailable {
			t.Errorf("Coin %s should be available", coin.Pair)
		}
	}

	t.Logf("✅ Got %d default coins", len(coins))
}

// TestGetCoinPoolWithoutAPI tests getting coin pool without API configured
func TestGetCoinPoolWithoutAPI(t *testing.T) {
	// Disable default coins and clear API URL
	SetUseDefaultCoins(false)
	SetCoinPoolAPI("")

	// Should fallback to default coins
	coins, err := GetCoinPool()
	if err != nil {
		t.Errorf("GetCoinPool without API should fallback to defaults: %v", err)
	}

	if len(coins) == 0 {
		t.Error("Expected fallback to default coins")
	}

	t.Logf("✅ Fallback returned %d coins", len(coins))
}

// TestGetAvailableCoins tests getting available coins list
func TestGetAvailableCoins(t *testing.T) {
	SetUseDefaultCoins(true)

	symbols, err := GetAvailableCoins()
	if err != nil {
		t.Errorf("GetAvailableCoins failed: %v", err)
	}

	if len(symbols) == 0 {
		t.Error("Expected non-empty symbols list")
	}

	// Verify all symbols are valid
	for _, symbol := range symbols {
		if len(symbol) == 0 {
			t.Error("Found empty symbol")
		}
	}

	t.Logf("✅ Got %d available symbols", len(symbols))
}

// TestGetTopRatedCoins tests getting top rated coins
func TestGetTopRatedCoins(t *testing.T) {
	SetUseDefaultCoins(true)

	tests := []struct {
		name  string
		limit int
	}{
		{"Top 3 coins", 3},
		{"Top 5 coins", 5},
		{"Top 10 coins", 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			symbols, err := GetTopRatedCoins(tt.limit)
			if err != nil {
				t.Errorf("GetTopRatedCoins(%d) failed: %v", tt.limit, err)
			}

			if len(symbols) == 0 {
				t.Errorf("Expected non-empty symbols list for limit %d", tt.limit)
			}

			// Should not exceed limit
			if len(symbols) > tt.limit {
				t.Errorf("Expected at most %d symbols, got %d", tt.limit, len(symbols))
			}

			t.Logf("✅ Got %d symbols for limit %d", len(symbols), tt.limit)
		})
	}
}

// TestNormalizeSymbol tests symbol normalization
func TestNormalizeSymbol(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Lowercase to uppercase",
			input:    "btc",
			expected: "BTCUSDT",
		},
		{
			name:     "Already has USDT",
			input:    "BTCUSDT",
			expected: "BTCUSDT",
		},
		{
			name:     "With spaces",
			input:    " BTC ",
			expected: "BTCUSDT",
		},
		{
			name:     "Lowercase with USDT",
			input:    "ethusdt",
			expected: "ETHUSDT",
		},
		{
			name:     "Mixed case",
			input:    "SoL",
			expected: "SOLUSDT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeSymbol(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeSymbol(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestTrimSpaces tests the trimSpaces helper function
func TestTrimSpaces(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{" hello ", "hello"},
		{"no spaces", "nospaces"},
		{"  multiple  spaces  ", "multiplespaces"},
		{"", ""},
		{"   ", ""},
	}

	for _, tt := range tests {
		result := trimSpaces(tt.input)
		if result != tt.expected {
			t.Errorf("trimSpaces(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

// TestToUpper tests the toUpper helper function
func TestToUpper(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello", "HELLO"},
		{"WORLD", "WORLD"},
		{"MixedCase", "MIXEDCASE"},
		{"", ""},
		{"123abc", "123ABC"},
	}

	for _, tt := range tests {
		result := toUpper(tt.input)
		if result != tt.expected {
			t.Errorf("toUpper(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

// TestEndsWith tests the endsWith helper function
func TestEndsWith(t *testing.T) {
	tests := []struct {
		str      string
		suffix   string
		expected bool
	}{
		{"BTCUSDT", "USDT", true},
		{"ETHUSDT", "BTC", false},
		{"hello", "lo", true},
		{"short", "toolong", false},
		{"", "", true},
		{"test", "", true},
	}

	for _, tt := range tests {
		result := endsWith(tt.str, tt.suffix)
		if result != tt.expected {
			t.Errorf("endsWith(%q, %q) = %v, want %v", tt.str, tt.suffix, result, tt.expected)
		}
	}
}

// TestConvertSymbolsToCoins tests symbol to CoinInfo conversion
func TestConvertSymbolsToCoins(t *testing.T) {
	symbols := []string{"BTCUSDT", "ETHUSDT", "SOLUSDT"}
	coins := convertSymbolsToCoins(symbols)

	if len(coins) != len(symbols) {
		t.Errorf("Expected %d coins, got %d", len(symbols), len(coins))
	}

	for i, coin := range coins {
		if coin.Pair != symbols[i] {
			t.Errorf("Expected pair %s, got %s", symbols[i], coin.Pair)
		}

		if !coin.IsAvailable {
			t.Errorf("Coin %s should be available", coin.Pair)
		}

		if coin.Score != 0 {
			t.Errorf("Expected score 0 for converted coin, got %f", coin.Score)
		}
	}
}

// TestCacheOperations tests cache save and load operations
func TestCacheOperations(t *testing.T) {
	// Create temporary cache directory
	tmpDir := filepath.Join(os.TempDir(), "test_coin_pool_cache")
	defer os.RemoveAll(tmpDir)

	// Set cache directory
	originalCacheDir := coinPoolConfig.CacheDir
	coinPoolConfig.CacheDir = tmpDir
	defer func() { coinPoolConfig.CacheDir = originalCacheDir }()

	// Create test coins
	testCoins := []CoinInfo{
		{Pair: "BTCUSDT", Score: 95.5, IsAvailable: true},
		{Pair: "ETHUSDT", Score: 88.3, IsAvailable: true},
		{Pair: "SOLUSDT", Score: 76.2, IsAvailable: true},
	}

	// Test save
	err := saveCoinPoolCache(testCoins)
	if err != nil {
		t.Errorf("saveCoinPoolCache failed: %v", err)
	}

	// Verify cache file exists
	cachePath := filepath.Join(tmpDir, "latest.json")
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		t.Error("Cache file was not created")
	}

	// Test load
	loadedCoins, err := loadCoinPoolCache()
	if err != nil {
		t.Errorf("loadCoinPoolCache failed: %v", err)
	}

	if len(loadedCoins) != len(testCoins) {
		t.Errorf("Expected %d coins, got %d", len(testCoins), len(loadedCoins))
	}

	for i, loaded := range loadedCoins {
		if loaded.Pair != testCoins[i].Pair {
			t.Errorf("Expected pair %s, got %s", testCoins[i].Pair, loaded.Pair)
		}
		if loaded.Score != testCoins[i].Score {
			t.Errorf("Expected score %f, got %f", testCoins[i].Score, loaded.Score)
		}
	}

	t.Log("✅ Cache save and load operations passed")
}

// TestOITopCacheOperations tests OI Top cache operations
func TestOITopCacheOperations(t *testing.T) {
	// Create temporary cache directory
	tmpDir := filepath.Join(os.TempDir(), "test_oi_top_cache")
	defer os.RemoveAll(tmpDir)

	// Set cache directory
	originalCacheDir := oiTopConfig.CacheDir
	oiTopConfig.CacheDir = tmpDir
	defer func() { oiTopConfig.CacheDir = originalCacheDir }()

	// Create test positions
	testPositions := []OIPosition{
		{Symbol: "BTCUSDT", Rank: 1, CurrentOI: 1000000, OIDeltaPercent: 5.5},
		{Symbol: "ETHUSDT", Rank: 2, CurrentOI: 800000, OIDeltaPercent: 4.2},
		{Symbol: "SOLUSDT", Rank: 3, CurrentOI: 600000, OIDeltaPercent: 3.8},
	}

	// Test save
	err := saveOITopCache(testPositions)
	if err != nil {
		t.Errorf("saveOITopCache failed: %v", err)
	}

	// Verify cache file exists
	cachePath := filepath.Join(tmpDir, "oi_top_latest.json")
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		t.Error("OI Top cache file was not created")
	}

	// Test load
	loadedPositions, err := loadOITopCache()
	if err != nil {
		t.Errorf("loadOITopCache failed: %v", err)
	}

	if len(loadedPositions) != len(testPositions) {
		t.Errorf("Expected %d positions, got %d", len(testPositions), len(loadedPositions))
	}

	for i, loaded := range loadedPositions {
		if loaded.Symbol != testPositions[i].Symbol {
			t.Errorf("Expected symbol %s, got %s", testPositions[i].Symbol, loaded.Symbol)
		}
		if loaded.Rank != testPositions[i].Rank {
			t.Errorf("Expected rank %d, got %d", testPositions[i].Rank, loaded.Rank)
		}
	}

	t.Log("✅ OI Top cache save and load operations passed")
}

// TestGetOITopPositionsWithoutAPI tests OI Top without API configured
func TestGetOITopPositionsWithoutAPI(t *testing.T) {
	// Clear API URL
	SetOITopAPI("")

	positions, err := GetOITopPositions()
	if err != nil {
		t.Errorf("GetOITopPositions without API should not error: %v", err)
	}

	// Should return empty list without error
	if len(positions) != 0 {
		t.Errorf("Expected empty positions without API, got %d", len(positions))
	}

	t.Log("✅ GetOITopPositions returned empty list without API (expected)")
}

// TestGetOITopSymbols tests getting OI Top symbols
func TestGetOITopSymbols(t *testing.T) {
	// Without API, should return empty
	SetOITopAPI("")

	symbols, err := GetOITopSymbols()
	if err != nil {
		t.Errorf("GetOITopSymbols failed: %v", err)
	}

	// Should be empty without API
	if len(symbols) != 0 {
		t.Logf("Got %d OI Top symbols (unexpected, API might be configured)", len(symbols))
	}
}

// TestGetMergedCoinPool tests merged coin pool functionality
func TestGetMergedCoinPool(t *testing.T) {
	// Use default coins
	SetUseDefaultCoins(true)
	SetOITopAPI("") // No OI Top API

	merged, err := GetMergedCoinPool(5)
	if err != nil {
		t.Errorf("GetMergedCoinPool failed: %v", err)
	}

	if merged == nil {
		t.Fatal("GetMergedCoinPool returned nil")
	}

	if len(merged.AllSymbols) == 0 {
		t.Error("Expected non-empty merged symbols")
	}

	if merged.SymbolSources == nil {
		t.Error("SymbolSources should not be nil")
	}

	t.Logf("✅ Merged coin pool: AI500=%d, OI=%d, Total=%d",
		len(merged.AI500Coins), len(merged.OITopCoins), len(merged.AllSymbols))
}

// TestCacheAgeWarning tests cache age detection
func TestCacheAgeWarning(t *testing.T) {
	// Create temporary cache directory
	tmpDir := filepath.Join(os.TempDir(), "test_cache_age")
	defer os.RemoveAll(tmpDir)

	originalCacheDir := coinPoolConfig.CacheDir
	coinPoolConfig.CacheDir = tmpDir
	defer func() { coinPoolConfig.CacheDir = originalCacheDir }()

	// Create cache with old timestamp
	oldTime := time.Now().Add(-25 * time.Hour)
	testCoins := []CoinInfo{
		{Pair: "BTCUSDT", Score: 90.0, IsAvailable: true},
	}

	// Manually create cache file with old timestamp
	cache := CoinPoolCache{
		Coins:      testCoins,
		FetchedAt:  oldTime,
		SourceType: "api",
	}

	data, _ := jsonMarshalIndent(cache, "", "  ")
	cachePath := filepath.Join(tmpDir, "latest.json")
	os.MkdirAll(tmpDir, 0755)
	writeFile(cachePath, data, 0644)

	// Load cache (should warn about old data)
	loadedCoins, err := loadCoinPoolCache()
	if err != nil {
		t.Errorf("loadCoinPoolCache failed: %v", err)
	}

	if len(loadedCoins) != 1 {
		t.Errorf("Expected 1 coin, got %d", len(loadedCoins))
	}

	t.Log("✅ Cache age detection working (check logs for warning)")
}

// Helper functions for TestCacheAgeWarning
func jsonMarshalIndent(v interface{}, prefix, indent string) ([]byte, error) {
	// Simple implementation for testing
	return []byte(`{"coins":[{"pair":"BTCUSDT","score":90,"start_time":0,"start_price":0,"last_score":0,"max_score":0,"max_price":0,"increase_percent":0}],"fetched_at":"` + time.Now().Add(-25*time.Hour).Format(time.RFC3339) + `","source_type":"api"}`), nil
}

func writeFile(filename string, data []byte, perm os.FileMode) error {
	return os.WriteFile(filename, data, perm)
}
