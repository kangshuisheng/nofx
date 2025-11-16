package market

import (
	"fmt"
	"testing"
	"time"
)

// MockDataSource is a mock implementation of DataSource for testing
type MockDataSource struct {
	name          string
	healthy       bool
	latency       time.Duration
	failKlines    bool
	failTicker    bool
	klinesData    []Kline
	tickerData    *Ticker
	healthCheckFn func() error
}

func (m *MockDataSource) GetName() string {
	return m.name
}

func (m *MockDataSource) GetKlines(symbol, interval string, limit int) ([]Kline, error) {
	if m.failKlines {
		return nil, fmt.Errorf("mock klines error")
	}
	return m.klinesData, nil
}

func (m *MockDataSource) GetTicker(symbol string) (*Ticker, error) {
	if m.failTicker {
		return nil, fmt.Errorf("mock ticker error")
	}
	return m.tickerData, nil
}

func (m *MockDataSource) HealthCheck() error {
	if m.healthCheckFn != nil {
		return m.healthCheckFn()
	}
	if !m.healthy {
		return fmt.Errorf("mock health check failed")
	}
	return nil
}

func (m *MockDataSource) GetLatency() time.Duration {
	return m.latency
}

// TestNewDataSourceManager tests creating a new DataSourceManager
func TestNewDataSourceManager(t *testing.T) {
	dsm := NewDataSourceManager(10 * time.Second)

	if dsm == nil {
		t.Fatal("NewDataSourceManager returned nil")
	}

	if dsm.checkInterval != 10*time.Second {
		t.Errorf("Expected check interval 10s, got %v", dsm.checkInterval)
	}

	if len(dsm.sources) != 0 {
		t.Errorf("Expected 0 sources initially, got %d", len(dsm.sources))
	}

	t.Logf("✅ NewDataSourceManager created successfully")
}

// TestNewDataSourceManager_DefaultInterval tests default interval
func TestNewDataSourceManager_DefaultInterval(t *testing.T) {
	dsm := NewDataSourceManager(0) // Invalid interval

	if dsm.checkInterval != 30*time.Second {
		t.Errorf("Expected default interval 30s, got %v", dsm.checkInterval)
	}

	t.Logf("✅ Default interval applied correctly")
}

// TestAddSource tests adding data sources
func TestAddSource(t *testing.T) {
	dsm := NewDataSourceManager(10 * time.Second)

	mock1 := &MockDataSource{name: "source1", healthy: true}
	mock2 := &MockDataSource{name: "source2", healthy: true}

	dsm.AddSource(mock1)
	dsm.AddSource(mock2)

	if len(dsm.sources) != 2 {
		t.Errorf("Expected 2 sources, got %d", len(dsm.sources))
	}

	if len(dsm.statuses) != 2 {
		t.Errorf("Expected 2 statuses, got %d", len(dsm.statuses))
	}

	// Verify statuses are initialized
	status1 := dsm.statuses["source1"]
	if status1 == nil {
		t.Fatal("source1 status is nil")
	}
	if status1.Name != "source1" {
		t.Errorf("Expected name 'source1', got '%s'", status1.Name)
	}
	if !status1.Healthy {
		t.Error("source1 should be healthy initially")
	}

	t.Logf("✅ AddSource works correctly")
}

// TestGetHealthySource tests getting a healthy source with round-robin
func TestGetHealthySource(t *testing.T) {
	dsm := NewDataSourceManager(10 * time.Second)

	mock1 := &MockDataSource{name: "source1", healthy: true}
	mock2 := &MockDataSource{name: "source2", healthy: true}
	mock3 := &MockDataSource{name: "source3", healthy: true}

	dsm.AddSource(mock1)
	dsm.AddSource(mock2)
	dsm.AddSource(mock3)

	// Test round-robin behavior
	source1, err := dsm.GetHealthySource()
	if err != nil {
		t.Fatalf("GetHealthySource failed: %v", err)
	}
	if source1.GetName() != "source1" {
		t.Errorf("Expected source1, got %s", source1.GetName())
	}

	source2, err := dsm.GetHealthySource()
	if err != nil {
		t.Fatalf("GetHealthySource failed: %v", err)
	}
	if source2.GetName() != "source2" {
		t.Errorf("Expected source2, got %s", source2.GetName())
	}

	source3, err := dsm.GetHealthySource()
	if err != nil {
		t.Fatalf("GetHealthySource failed: %v", err)
	}
	if source3.GetName() != "source3" {
		t.Errorf("Expected source3, got %s", source3.GetName())
	}

	// Should wrap around to source1
	source4, err := dsm.GetHealthySource()
	if err != nil {
		t.Fatalf("GetHealthySource failed: %v", err)
	}
	if source4.GetName() != "source1" {
		t.Errorf("Expected source1 (wrap around), got %s", source4.GetName())
	}

	t.Logf("✅ GetHealthySource round-robin works correctly")
}

// TestGetHealthySource_SkipUnhealthy tests skipping unhealthy sources
func TestGetHealthySource_SkipUnhealthy(t *testing.T) {
	dsm := NewDataSourceManager(10 * time.Second)

	mock1 := &MockDataSource{name: "source1", healthy: false}
	mock2 := &MockDataSource{name: "source2", healthy: true}
	mock3 := &MockDataSource{name: "source3", healthy: false}

	dsm.AddSource(mock1)
	dsm.AddSource(mock2)
	dsm.AddSource(mock3)

	// Mark source1 and source3 as unhealthy
	dsm.statuses["source1"].Healthy = false
	dsm.statuses["source3"].Healthy = false

	// Should skip unhealthy sources and return source2
	source, err := dsm.GetHealthySource()
	if err != nil {
		t.Fatalf("GetHealthySource failed: %v", err)
	}
	if source.GetName() != "source2" {
		t.Errorf("Expected source2 (only healthy), got %s", source.GetName())
	}

	t.Logf("✅ GetHealthySource correctly skips unhealthy sources")
}

// TestGetHealthySource_AllUnhealthy tests fallback when all sources are unhealthy
func TestGetHealthySource_AllUnhealthy(t *testing.T) {
	dsm := NewDataSourceManager(10 * time.Second)

	mock1 := &MockDataSource{name: "source1", healthy: false}
	mock2 := &MockDataSource{name: "source2", healthy: false}

	dsm.AddSource(mock1)
	dsm.AddSource(mock2)

	dsm.statuses["source1"].Healthy = false
	dsm.statuses["source2"].Healthy = false

	// Should return first source as fallback
	source, err := dsm.GetHealthySource()
	if err != nil {
		t.Fatalf("GetHealthySource failed: %v", err)
	}
	if source.GetName() != "source1" {
		t.Errorf("Expected source1 (fallback), got %s", source.GetName())
	}

	t.Logf("✅ GetHealthySource fallback works when all sources unhealthy")
}

// TestGetHealthySource_NoSources tests error when no sources available
func TestGetHealthySource_NoSources(t *testing.T) {
	dsm := NewDataSourceManager(10 * time.Second)

	_, err := dsm.GetHealthySource()
	if err == nil {
		t.Error("Expected error when no sources available, got nil")
	}

	t.Logf("✅ GetHealthySource correctly returns error with no sources")
}

// TestGetKlinesWithFallback tests K-line fetching with fallback
func TestGetKlinesWithFallback(t *testing.T) {
	dsm := NewDataSourceManager(10 * time.Second)

	mockKlines := []Kline{
		{OpenTime: 1000, Close: 50000},
		{OpenTime: 2000, Close: 51000},
	}

	// First source fails, second succeeds
	mock1 := &MockDataSource{name: "source1", healthy: true, failKlines: true}
	mock2 := &MockDataSource{name: "source2", healthy: true, klinesData: mockKlines}

	dsm.AddSource(mock1)
	dsm.AddSource(mock2)

	klines, err := dsm.GetKlinesWithFallback("BTCUSDT", "1m", 2)
	if err != nil {
		t.Fatalf("GetKlinesWithFallback failed: %v", err)
	}

	if len(klines) != 2 {
		t.Errorf("Expected 2 klines, got %d", len(klines))
	}

	// Verify request count incremented
	if dsm.statuses["source1"].TotalRequests != 1 {
		t.Errorf("Expected source1 requests=1, got %d", dsm.statuses["source1"].TotalRequests)
	}
	if dsm.statuses["source2"].TotalRequests != 1 {
		t.Errorf("Expected source2 requests=1, got %d", dsm.statuses["source2"].TotalRequests)
	}

	t.Logf("✅ GetKlinesWithFallback successfully fell back to second source")
}

// TestGetKlinesWithFallback_AllFail tests when all sources fail
func TestGetKlinesWithFallback_AllFail(t *testing.T) {
	dsm := NewDataSourceManager(10 * time.Second)

	mock1 := &MockDataSource{name: "source1", healthy: true, failKlines: true}
	mock2 := &MockDataSource{name: "source2", healthy: true, failKlines: true}

	dsm.AddSource(mock1)
	dsm.AddSource(mock2)

	_, err := dsm.GetKlinesWithFallback("BTCUSDT", "1m", 2)
	if err == nil {
		t.Error("Expected error when all sources fail, got nil")
	}

	t.Logf("✅ GetKlinesWithFallback correctly returns error when all fail")
}

// TestGetTickerWithFallback tests ticker fetching with fallback
func TestGetTickerWithFallback(t *testing.T) {
	dsm := NewDataSourceManager(10 * time.Second)

	mockTicker := &Ticker{
		Symbol:    "BTCUSDT",
		LastPrice: 50000.0,
		Volume:    1000000.0,
	}

	// First source fails, second succeeds
	mock1 := &MockDataSource{name: "source1", healthy: true, failTicker: true}
	mock2 := &MockDataSource{name: "source2", healthy: true, tickerData: mockTicker}

	dsm.AddSource(mock1)
	dsm.AddSource(mock2)

	ticker, err := dsm.GetTickerWithFallback("BTCUSDT")
	if err != nil {
		t.Fatalf("GetTickerWithFallback failed: %v", err)
	}

	if ticker.LastPrice != 50000.0 {
		t.Errorf("Expected price 50000.0, got %f", ticker.LastPrice)
	}

	t.Logf("✅ GetTickerWithFallback successfully fell back to second source")
}

// TestGetStatus tests getting all source statuses
func TestGetStatus(t *testing.T) {
	dsm := NewDataSourceManager(10 * time.Second)

	mock1 := &MockDataSource{name: "source1", healthy: true}
	mock2 := &MockDataSource{name: "source2", healthy: false}

	dsm.AddSource(mock1)
	dsm.AddSource(mock2)

	dsm.statuses["source2"].Healthy = false
	dsm.statuses["source2"].FailureCount = 3

	statuses := dsm.GetStatus()

	if len(statuses) != 2 {
		t.Errorf("Expected 2 statuses, got %d", len(statuses))
	}

	status1 := statuses["source1"]
	if status1 == nil {
		t.Fatal("source1 status is nil")
	}
	if !status1.Healthy {
		t.Error("source1 should be healthy")
	}

	status2 := statuses["source2"]
	if status2 == nil {
		t.Fatal("source2 status is nil")
	}
	if status2.Healthy {
		t.Error("source2 should be unhealthy")
	}
	if status2.FailureCount != 3 {
		t.Errorf("Expected failure count 3, got %d", status2.FailureCount)
	}

	t.Logf("✅ GetStatus returns correct status information")
}

// TestVerifyPriceConsistency tests price consistency verification
func TestVerifyPriceConsistency(t *testing.T) {
	dsm := NewDataSourceManager(10 * time.Second)

	// Create 3 sources with similar prices (consistent)
	mock1 := &MockDataSource{
		name:    "source1",
		healthy: true,
		tickerData: &Ticker{
			Symbol:    "BTCUSDT",
			LastPrice: 50000.0,
		},
	}
	mock2 := &MockDataSource{
		name:    "source2",
		healthy: true,
		tickerData: &Ticker{
			Symbol:    "BTCUSDT",
			LastPrice: 50100.0,
		},
	}
	mock3 := &MockDataSource{
		name:    "source3",
		healthy: true,
		tickerData: &Ticker{
			Symbol:    "BTCUSDT",
			LastPrice: 49900.0,
		},
	}

	dsm.AddSource(mock1)
	dsm.AddSource(mock2)
	dsm.AddSource(mock3)

	// Test with 1% max deviation (should pass)
	consistent, prices, err := dsm.VerifyPriceConsistency("BTCUSDT", 0.01)
	if err != nil {
		t.Errorf("VerifyPriceConsistency failed: %v", err)
	}

	if !consistent {
		t.Error("Prices should be consistent within 1% deviation")
	}

	if len(prices) != 3 {
		t.Errorf("Expected 3 prices, got %d", len(prices))
	}

	t.Logf("✅ VerifyPriceConsistency correctly validates consistent prices")
}

// TestVerifyPriceConsistency_Inconsistent tests detecting price inconsistency
func TestVerifyPriceConsistency_Inconsistent(t *testing.T) {
	dsm := NewDataSourceManager(10 * time.Second)

	// Create sources with large price difference (inconsistent)
	mock1 := &MockDataSource{
		name:    "source1",
		healthy: true,
		tickerData: &Ticker{
			Symbol:    "BTCUSDT",
			LastPrice: 50000.0,
		},
	}
	mock2 := &MockDataSource{
		name:    "source2",
		healthy: true,
		tickerData: &Ticker{
			Symbol:    "BTCUSDT",
			LastPrice: 60000.0, // 20% higher
		},
	}

	dsm.AddSource(mock1)
	dsm.AddSource(mock2)

	// Test with 1% max deviation (should fail)
	consistent, prices, err := dsm.VerifyPriceConsistency("BTCUSDT", 0.01)
	if err != nil {
		t.Errorf("VerifyPriceConsistency failed: %v", err)
	}

	if consistent {
		t.Error("Prices should NOT be consistent with 20% difference")
	}

	if len(prices) != 2 {
		t.Errorf("Expected 2 prices, got %d", len(prices))
	}

	t.Logf("✅ VerifyPriceConsistency correctly detects inconsistent prices")
}

// TestVerifyPriceConsistency_InsufficientSources tests with insufficient sources
func TestVerifyPriceConsistency_InsufficientSources(t *testing.T) {
	dsm := NewDataSourceManager(10 * time.Second)

	// Only one source
	mock1 := &MockDataSource{
		name:    "source1",
		healthy: true,
		tickerData: &Ticker{
			Symbol:    "BTCUSDT",
			LastPrice: 50000.0,
		},
	}

	dsm.AddSource(mock1)

	_, _, err := dsm.VerifyPriceConsistency("BTCUSDT", 0.01)
	if err == nil {
		t.Error("Expected error with insufficient sources, got nil")
	}

	t.Logf("✅ VerifyPriceConsistency correctly handles insufficient sources")
}

// TestStartStop tests starting and stopping the manager
func TestStartStop(t *testing.T) {
	dsm := NewDataSourceManager(100 * time.Millisecond)

	mock1 := &MockDataSource{name: "source1", healthy: true}
	dsm.AddSource(mock1)

	// Start health check
	dsm.Start()

	// Wait a bit for health check to run
	time.Sleep(150 * time.Millisecond)

	// Stop
	dsm.Stop()

	// Verify health check ran (use thread-safe GetStatus method)
	statuses := dsm.GetStatus()
	if status, exists := statuses["source1"]; !exists {
		t.Error("source1 status not found")
	} else if status.SuccessCount == 0 {
		t.Error("Health check should have run at least once")
	}

	t.Logf("✅ Start/Stop cycle completed successfully")
}
