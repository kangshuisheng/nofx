package market

import (
	"testing"
	"time"
)

// TestGetKlinesWithRetry tests the K-line retrieval with retry logic
func TestGetKlinesWithRetry(t *testing.T) {
	client := NewAPIClient()

	tests := []struct {
		name     string
		symbol   string
		interval string
		limit    int
		wantErr  bool
	}{
		{
			name:     "Valid BTCUSDT 3m K-lines",
			symbol:   "BTCUSDT",
			interval: "3m",
			limit:    10,
			wantErr:  false,
		},
		{
			name:     "Valid ETHUSDT 15m K-lines",
			symbol:   "ETHUSDT",
			interval: "15m",
			limit:    20,
			wantErr:  false,
		},
		{
			name:     "Invalid symbol should fail",
			symbol:   "INVALIDSYMBOL",
			interval: "1m",
			limit:    5,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			klines, err := client.GetKlines(tt.symbol, tt.interval, tt.limit)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetKlines() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(klines) == 0 {
					t.Errorf("GetKlines() returned empty result for valid symbol")
				}
				if len(klines) > tt.limit {
					t.Errorf("GetKlines() returned more klines (%d) than limit (%d)", len(klines), tt.limit)
				}
				t.Logf("✅ Successfully retrieved %d K-lines for %s", len(klines), tt.symbol)
			}
		})

		// Add delay between tests to avoid rate limiting
		time.Sleep(500 * time.Millisecond)
	}
}

// TestGetOpenInterestHistoryWithRetry tests OI history retrieval
func TestGetOpenInterestHistoryWithRetry(t *testing.T) {
	client := NewAPIClient()

	tests := []struct {
		name    string
		symbol  string
		period  string
		limit   int
		wantErr bool
	}{
		{
			name:    "Valid BTCUSDT 15m OI",
			symbol:  "BTCUSDT",
			period:  "15m",
			limit:   20,
			wantErr: false,
		},
		{
			name:    "Valid ETHUSDT 1h OI",
			symbol:  "ETHUSDT",
			period:  "1h",
			limit:   10,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snapshots, err := client.GetOpenInterestHistory(tt.symbol, tt.period, tt.limit)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetOpenInterestHistory() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(snapshots) == 0 {
					t.Errorf("GetOpenInterestHistory() returned empty result for valid symbol")
				}
				t.Logf("✅ Successfully retrieved %d OI snapshots for %s", len(snapshots), tt.symbol)
			}
		})

		// Add delay between tests to avoid rate limiting
		time.Sleep(500 * time.Millisecond)
	}
}

// TestBinanceErrorResponse tests error response parsing
func TestBinanceErrorResponse(t *testing.T) {
	err := &BinanceErrorResponse{
		Code: -1003,
		Msg:  "Too many requests",
	}

	expectedMsg := "Binance API error (code -1003): Too many requests"
	if err.Error() != expectedMsg {
		t.Errorf("BinanceErrorResponse.Error() = %v, want %v", err.Error(), expectedMsg)
	}
}

// TestTimeoutConfiguration tests that timeout is properly set
func TestTimeoutConfiguration(t *testing.T) {
	client := NewAPIClient()

	if client.client.Timeout != 60*time.Second {
		t.Errorf("Client timeout = %v, want 60s", client.client.Timeout)
	}
	t.Logf("✅ HTTP client timeout properly set to %v", client.client.Timeout)
}

// BenchmarkGetKlines benchmarks K-line retrieval
func BenchmarkGetKlines(b *testing.B) {
	client := NewAPIClient()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.GetKlines("BTCUSDT", "3m", 10)
		if err != nil {
			b.Logf("Request failed: %v", err)
		}
		time.Sleep(100 * time.Millisecond) // Rate limit protection
	}
}
