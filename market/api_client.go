package market

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"nofx/hook"
	"strconv"
	"time"
)

const (
	defaultBaseURL = "https://fapi.binance.com"
)

var baseURL = defaultBaseURL

type APIClient struct {
	client *http.Client
}

func NewAPIClient() *APIClient {
	client := &http.Client{
		Timeout: 60 * time.Second, // Increased from 30s to 60s
	}

	hookRes := hook.HookExec[hook.SetHttpClientResult](hook.SET_HTTP_CLIENT, client)
	if hookRes != nil && hookRes.Error() == nil {
		log.Printf("Using HTTP client from Hook")
		client = hookRes.GetResult()
	}

	return &APIClient{
		client: client,
	}
}

func (c *APIClient) GetExchangeInfo() (*ExchangeInfo, error) {
	url := fmt.Sprintf("%s/fapi/v1/exchangeInfo", baseURL)
	resp, err := c.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var exchangeInfo ExchangeInfo
	err = json.Unmarshal(body, &exchangeInfo)
	if err != nil {
		return nil, err
	}

	return &exchangeInfo, nil
}

func (c *APIClient) GetKlines(symbol, interval string, limit int) ([]Kline, error) {
	const maxRetries = 3
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		klines, err := c.getKlinesAttempt(symbol, interval, limit, attempt)
		if err == nil {
			return klines, nil
		}

		lastErr = err
		if attempt < maxRetries {
			backoff := time.Duration(attempt) * 2 * time.Second
			log.Printf("⚠️ GetKlines attempt %d/%d failed for %s: %v, retrying in %v...",
				attempt, maxRetries, symbol, err, backoff)
			time.Sleep(backoff)
		}
	}

	// 如果所有重试都失败，尝试从多数据源管理器获取（故障转移）
	if WSMonitorCli != nil && WSMonitorCli.dsManager != nil {
		log.Printf("⚠️  Binance API 失败，尝试从多数据源池获取 %s %s 数据...", symbol, interval)
		klines, err := WSMonitorCli.dsManager.GetKlinesWithFallback(symbol, interval, limit)
		if err == nil {
			log.Printf("✅ 故障转移成功：从备用数据源获取 %s %s 数据", symbol, interval)
			return klines, nil
		}
		log.Printf("⚠️  多数据源池也失败: %v", err)
	}

	return nil, fmt.Errorf("failed after %d attempts: %w", maxRetries, lastErr)
}

func (c *APIClient) getKlinesAttempt(symbol, interval string, limit int, attempt int) ([]Kline, error) {
	url := fmt.Sprintf("%s/fapi/v1/klines", baseURL)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	q := req.URL.Query()
	q.Add("symbol", symbol)
	q.Add("interval", interval)
	q.Add("limit", strconv.Itoa(limit))
	req.URL.RawQuery = q.Encode()

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body failed: %w", err)
	}

	// Check HTTP status code first
	if resp.StatusCode != http.StatusOK {
		// Try to parse as Binance error response
		var binanceErr BinanceErrorResponse
		if json.Unmarshal(body, &binanceErr) == nil && binanceErr.Code != 0 {
			log.Printf("❌ Binance API error for %s (attempt %d): %v", symbol, attempt, &binanceErr)
			return nil, &binanceErr
		}
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	// Try to parse as error response first (in case of rate limit)
	var binanceErr BinanceErrorResponse
	if json.Unmarshal(body, &binanceErr) == nil && binanceErr.Code != 0 {
		log.Printf("❌ Binance API error for %s (attempt %d): %v", symbol, attempt, &binanceErr)
		return nil, &binanceErr
	}

	// Parse as normal kline response
	var klineResponses []KlineResponse
	err = json.Unmarshal(body, &klineResponses)
	if err != nil {
		log.Printf("❌ Failed to parse K-line data for %s (attempt %d), response: %s",
			symbol, attempt, string(body[:min(200, len(body))]))
		return nil, fmt.Errorf("parse K-line JSON failed: %w", err)
	}

	var klines []Kline
	for i, kr := range klineResponses {
		kline, err := parseKline(kr)
		if err != nil {
			log.Printf("⚠️ Failed to parse K-line entry %d for %s: %v", i, symbol, err)
			continue
		}
		klines = append(klines, kline)
	}

	if len(klines) == 0 {
		return nil, fmt.Errorf("no valid K-line data returned")
	}

	return klines, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// setBaseURLForTesting allows tests to override the Binance API base URL.
func setBaseURLForTesting(url string) {
	baseURL = url
}

func parseKline(kr KlineResponse) (Kline, error) {
	var kline Kline

	if len(kr) < 11 {
		return kline, fmt.Errorf("invalid kline data")
	}

	// 解析各个字段
	kline.OpenTime = int64(kr[0].(float64))
	kline.Open, _ = strconv.ParseFloat(kr[1].(string), 64)
	kline.High, _ = strconv.ParseFloat(kr[2].(string), 64)
	kline.Low, _ = strconv.ParseFloat(kr[3].(string), 64)
	kline.Close, _ = strconv.ParseFloat(kr[4].(string), 64)
	kline.Volume, _ = strconv.ParseFloat(kr[5].(string), 64)
	kline.CloseTime = int64(kr[6].(float64))
	kline.QuoteVolume, _ = strconv.ParseFloat(kr[7].(string), 64)
	kline.Trades = int(kr[8].(float64))
	kline.TakerBuyBaseVolume, _ = strconv.ParseFloat(kr[9].(string), 64)
	kline.TakerBuyQuoteVolume, _ = strconv.ParseFloat(kr[10].(string), 64)

	return kline, nil
}

func (c *APIClient) GetCurrentPrice(symbol string) (float64, error) {
	url := fmt.Sprintf("%s/fapi/v1/ticker/price", baseURL)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, err
	}

	q := req.URL.Query()
	q.Add("symbol", symbol)
	req.URL.RawQuery = q.Encode()

	resp, err := c.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var ticker PriceTicker
	err = json.Unmarshal(body, &ticker)
	if err != nil {
		return 0, err
	}

	price, err := strconv.ParseFloat(ticker.Price, 64)
	if err != nil {
		return 0, err
	}

	return price, nil
}

// GetOpenInterest 获取持仓量（P0修复：用于OI历史数据采集）
func (c *APIClient) GetOpenInterest(symbol string) (*OIData, error) {
	url := fmt.Sprintf("%s/fapi/v1/openInterest?symbol=%s", baseURL, symbol)

	resp, err := c.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		OpenInterest string `json:"openInterest"`
		Symbol       string `json:"symbol"`
		Time         int64  `json:"time"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	oi, _ := strconv.ParseFloat(result.OpenInterest, 64)

	return &OIData{
		Latest:       oi,
		Average:      oi * 0.999, // 近似平均值
		ActualPeriod: "snapshot", // 標記為快照數據，非計算值
	}, nil
}

// GetOpenInterestHistory retrieves historical OI data (for backfilling on startup)
// period: "5m", "15m", "30m", "1h", "2h", "4h", "6h", "12h", "1d"
// limit: default 30, max 500 (we need 20 15-minute data points = 5 hours)
func (c *APIClient) GetOpenInterestHistory(symbol string, period string, limit int) ([]OISnapshot, error) {
	const maxRetries = 3
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		snapshots, err := c.getOpenInterestHistoryAttempt(symbol, period, limit, attempt)
		if err == nil {
			return snapshots, nil
		}

		lastErr = err
		if attempt < maxRetries {
			backoff := time.Duration(attempt) * 2 * time.Second
			log.Printf("⚠️ GetOpenInterestHistory attempt %d/%d failed for %s: %v, retrying in %v...",
				attempt, maxRetries, symbol, err, backoff)
			time.Sleep(backoff)
		}
	}

	return nil, fmt.Errorf("failed after %d attempts: %w", maxRetries, lastErr)
}

func (c *APIClient) getOpenInterestHistoryAttempt(symbol string, period string, limit int, attempt int) ([]OISnapshot, error) {
	url := fmt.Sprintf("%s/futures/data/openInterestHist", baseURL)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	q := req.URL.Query()
	q.Add("symbol", symbol)
	q.Add("period", period)
	q.Add("limit", strconv.Itoa(limit))
	req.URL.RawQuery = q.Encode()

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body failed: %w", err)
	}

	// Check HTTP status code
	if resp.StatusCode != http.StatusOK {
		var binanceErr BinanceErrorResponse
		if json.Unmarshal(body, &binanceErr) == nil && binanceErr.Code != 0 {
			log.Printf("❌ Binance API error for OI history %s (attempt %d): %v", symbol, attempt, &binanceErr)
			return nil, &binanceErr
		}
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	// Try to parse as error response first
	var binanceErr BinanceErrorResponse
	if json.Unmarshal(body, &binanceErr) == nil && binanceErr.Code != 0 {
		log.Printf("❌ Binance API error for OI history %s (attempt %d): %v", symbol, attempt, &binanceErr)
		return nil, &binanceErr
	}

	var histData []struct {
		Symbol               string `json:"symbol"`
		SumOpenInterest      string `json:"sumOpenInterest"`
		SumOpenInterestValue string `json:"sumOpenInterestValue"`
		Timestamp            int64  `json:"timestamp"`
	}

	if err := json.Unmarshal(body, &histData); err != nil {
		log.Printf("❌ Failed to parse OI history for %s (attempt %d), response: %s",
			symbol, attempt, string(body[:min(200, len(body))]))
		return nil, fmt.Errorf("parse OI history JSON failed: %w", err)
	}

	// Convert to OISnapshot format
	snapshots := make([]OISnapshot, 0, len(histData))
	for _, item := range histData {
		oi, _ := strconv.ParseFloat(item.SumOpenInterest, 64)
		snapshots = append(snapshots, OISnapshot{
			Value:     oi,
			Timestamp: time.Unix(item.Timestamp/1000, 0), // Binance returns millisecond timestamp
		})
	}

	return snapshots, nil
}
