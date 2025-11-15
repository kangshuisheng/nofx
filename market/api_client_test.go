package market

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// TestGetKlinesWithRetry tests the K-line retrieval with retry logic without real network calls.
func TestGetKlinesWithRetry(t *testing.T) {
	client := NewAPIClient()
	cleanup := setupMockBinanceServer(t, client)
	defer cleanup()

	tests := []struct {
		name     string
		symbol   string
		interval string
		limit    int
		wantErr  bool
	}{
		{
			name:     "Valid BTCUSDT 3m K-lines (with retry)",
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
				t.Fatalf("GetKlines() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				if len(klines) == 0 {
					t.Fatalf("GetKlines() returned empty result for valid symbol")
				}
				if len(klines) > tt.limit {
					t.Fatalf("GetKlines() returned more klines (%d) than limit (%d)", len(klines), tt.limit)
				}
			}
		})
	}
}

// TestGetOpenInterestHistoryWithRetry tests OI history retrieval without calling the real API.
func TestGetOpenInterestHistoryWithRetry(t *testing.T) {
	client := NewAPIClient()
	cleanup := setupMockBinanceServer(t, client)
	defer cleanup()

	tests := []struct {
		name    string
		symbol  string
		period  string
		limit   int
		wantErr bool
	}{
		{
			name:    "Valid BTCUSDT 15m OI (with retry)",
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
				t.Fatalf("GetOpenInterestHistory() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr && len(snapshots) == 0 {
				t.Fatalf("GetOpenInterestHistory() returned empty result for valid symbol")
			}
		})
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
}

// BenchmarkGetKlines benchmarks K-line retrieval
func BenchmarkGetKlines(b *testing.B) {
	client := NewAPIClient()
	cleanup := setupMockBinanceServer(b, client)
	defer cleanup()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.GetKlines("BTCUSDT", "3m", 10)
		if err != nil {
			b.Logf("Request failed: %v", err)
		}
		time.Sleep(100 * time.Millisecond) // Rate limit protection
	}
}

type handlerRoundTripper struct {
	handler http.Handler
}

func (rt handlerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	recorder := httptest.NewRecorder()
	rt.handler.ServeHTTP(recorder, req)
	return recorder.Result(), nil
}

func setupMockBinanceServer(tb testing.TB, client *APIClient) func() {
	tb.Helper()

	var mu sync.Mutex
	klinesAttempts := make(map[string]int)
	oiAttempts := make(map[string]int)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/fapi/v1/klines":
			handleMockKlines(w, r, &mu, klinesAttempts)
		case "/futures/data/openInterestHist":
			handleMockOpenInterest(w, r, &mu, oiAttempts)
		default:
			http.NotFound(w, r)
		}
	})

	originalClient := client.client
	client.client = &http.Client{
		Timeout:   5 * time.Second,
		Transport: handlerRoundTripper{handler: handler},
	}
	setBaseURLForTesting("http://mock.binance.local")

	return func() {
		client.client = originalClient
		setBaseURLForTesting(defaultBaseURL)
	}
}

func handleMockKlines(w http.ResponseWriter, r *http.Request, mu *sync.Mutex, attempts map[string]int) {
	symbol := r.URL.Query().Get("symbol")
	if symbol == "INVALIDSYMBOL" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": -1121,
			"msg":  "Invalid symbol.",
		})
		return
	}

	mu.Lock()
	attempt := attempts[symbol]
	attempts[symbol] = attempt + 1
	mu.Unlock()

	if symbol == "BTCUSDT" && attempt == 0 {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("temporary error"))
		return
	}

	response := [][]any{
		{
			float64(1609459200000),
			"42000.00",
			"42100.00",
			"41900.00",
			"42050.00",
			"100.0",
			float64(1609459260000),
			"2000000.00",
			float64(150),
			"60.0",
			"40000.0",
		},
	}

	_ = json.NewEncoder(w).Encode(response)
}

func handleMockOpenInterest(w http.ResponseWriter, r *http.Request, mu *sync.Mutex, attempts map[string]int) {
	symbol := r.URL.Query().Get("symbol")

	mu.Lock()
	attempt := attempts[symbol]
	attempts[symbol] = attempt + 1
	mu.Unlock()

	if symbol == "BTCUSDT" && attempt == 0 {
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": -1003,
			"msg":  "Too many requests",
		})
		return
	}

	response := []map[string]any{
		{
			"symbol":               symbol,
			"sumOpenInterest":      "12345.67",
			"sumOpenInterestValue": "890123.45",
			"timestamp":            1609459200000,
		},
		{
			"symbol":               symbol,
			"sumOpenInterest":      "12500.00",
			"sumOpenInterestValue": "900000.00",
			"timestamp":            1609459260000,
		},
	}

	_ = json.NewEncoder(w).Encode(response)
}
