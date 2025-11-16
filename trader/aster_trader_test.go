package trader

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
)

// ============================================================
// 一、AsterTraderTestSuite - 继承 base test suite
// ============================================================

// AsterTraderTestSuite Aster交易器测试套件
// 继承 TraderTestSuite 并添加 Aster 特定的 mock 逻辑
type AsterTraderTestSuite struct {
	*TraderTestSuite // 嵌入基础测试套件
	mockServer       *httptest.Server
}

// NewAsterTraderTestSuite 创建 Aster 测试套件
func startIPv4Server(t *testing.T, handler http.Handler) *httptest.Server {
	return newTestHTTPServer(t, handler)
}

func NewAsterTraderTestSuite(t *testing.T) *AsterTraderTestSuite {
	// 创建 mock HTTP 服务器（强制IPv4，避免 sandbox 阻止IPv6 回环）
	mockServer := startIPv4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 根据不同的 URL 路径返回不同的 mock 响应
		path := r.URL.Path

		var respBody interface{}

		switch {
		// Mock GetBalance - /fapi/v3/balance (返回数组)
		case path == "/fapi/v3/balance":
			respBody = []map[string]interface{}{
				{
					"asset":              "USDT",
					"walletBalance":      "10000.00",
					"unrealizedProfit":   "100.50",
					"marginBalance":      "10100.50",
					"maintMargin":        "200.00",
					"initialMargin":      "2000.00",
					"maxWithdrawAmount":  "8000.00",
					"crossWalletBalance": "10000.00",
					"crossUnPnl":         "100.50",
					"availableBalance":   "8000.00",
				},
			}

		// Mock GetPositions - /fapi/v3/positionRisk
		case path == "/fapi/v3/positionRisk":
			respBody = []map[string]interface{}{
				{
					"symbol":           "BTCUSDT",
					"positionAmt":      "0.5",
					"entryPrice":       "50000.00",
					"markPrice":        "50500.00",
					"unRealizedProfit": "250.00",
					"liquidationPrice": "45000.00",
					"leverage":         "10",
					"positionSide":     "LONG",
				},
			}

		// Mock GetMarketPrice - /fapi/v3/ticker/price (返回单个对象)
		case path == "/fapi/v3/ticker/price":
			// 从查询参数获取symbol
			symbol := r.URL.Query().Get("symbol")
			if symbol == "" {
				symbol = "BTCUSDT"
			}
			// 根据symbol返回不同价格
			price := "50000.00"
			if symbol == "ETHUSDT" {
				price = "3000.00"
			} else if symbol == "INVALIDUSDT" {
				// 返回错误响应
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"code": -1121,
					"msg":  "Invalid symbol",
				})
				return
			}
			respBody = map[string]interface{}{
				"symbol": symbol,
				"price":  price,
			}

		// Mock ExchangeInfo - /fapi/v3/exchangeInfo
		case path == "/fapi/v3/exchangeInfo":
			respBody = map[string]interface{}{
				"symbols": []map[string]interface{}{
					{
						"symbol":             "BTCUSDT",
						"pricePrecision":     1,
						"quantityPrecision":  3,
						"baseAssetPrecision": 8,
						"quotePrecision":     8,
						"filters": []map[string]interface{}{
							{
								"filterType": "PRICE_FILTER",
								"tickSize":   "0.1",
							},
							{
								"filterType": "LOT_SIZE",
								"stepSize":   "0.001",
							},
						},
					},
					{
						"symbol":             "ETHUSDT",
						"pricePrecision":     2,
						"quantityPrecision":  3,
						"baseAssetPrecision": 8,
						"quotePrecision":     8,
						"filters": []map[string]interface{}{
							{
								"filterType": "PRICE_FILTER",
								"tickSize":   "0.01",
							},
							{
								"filterType": "LOT_SIZE",
								"stepSize":   "0.001",
							},
						},
					},
				},
			}

		// Mock CreateOrder - /fapi/v1/order and /fapi/v3/order
		case (path == "/fapi/v1/order" || path == "/fapi/v3/order") && r.Method == "POST":
			// 从请求中解析参数以确定symbol
			bodyBytes, _ := io.ReadAll(r.Body)
			var orderParams map[string]interface{}
			json.Unmarshal(bodyBytes, &orderParams)

			symbol := "BTCUSDT"
			if s, ok := orderParams["symbol"].(string); ok {
				symbol = s
			}

			respBody = map[string]interface{}{
				"orderId": 123456,
				"symbol":  symbol,
				"status":  "FILLED",
				"side":    orderParams["side"],
				"type":    orderParams["type"],
			}

		// Mock CancelOrder - /fapi/v1/order (DELETE)
		case path == "/fapi/v1/order" && r.Method == "DELETE":
			respBody = map[string]interface{}{
				"orderId": 123456,
				"symbol":  "BTCUSDT",
				"status":  "CANCELED",
			}

		// Mock ListOpenOrders - /fapi/v1/openOrders and /fapi/v3/openOrders
		case path == "/fapi/v1/openOrders" || path == "/fapi/v3/openOrders":
			respBody = []map[string]interface{}{}

		// Mock SetLeverage - /fapi/v1/leverage
		case path == "/fapi/v1/leverage":
			respBody = map[string]interface{}{
				"leverage": 10,
				"symbol":   "BTCUSDT",
			}

		// Mock SetMarginMode - /fapi/v1/marginType
		case path == "/fapi/v1/marginType":
			respBody = map[string]interface{}{
				"code": 200,
				"msg":  "success",
			}

		// Default: empty response
		default:
			respBody = map[string]interface{}{}
		}

		// 序列化响应
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(respBody)
	}))

	// 生成一个测试用的私钥
	privateKey, _ := crypto.GenerateKey()

	// 创建 mock trader，使用 mock server 的 URL
	trader := &AsterTrader{
		ctx:             context.Background(),
		user:            "0x1234567890123456789012345678901234567890",
		signer:          "0xabcdefabcdefabcdefabcdefabcdefabcdefabcd",
		privateKey:      privateKey,
		client:          mockServer.Client(),
		baseURL:         mockServer.URL, // 使用 mock server 的 URL
		symbolPrecision: make(map[string]SymbolPrecision),
	}

	// 创建基础套件
	baseSuite := NewTraderTestSuite(t, trader)

	return &AsterTraderTestSuite{
		TraderTestSuite: baseSuite,
		mockServer:      mockServer,
	}
}

// Cleanup 清理资源
func (s *AsterTraderTestSuite) Cleanup() {
	if s.mockServer != nil {
		s.mockServer.Close()
	}
	s.TraderTestSuite.Cleanup()
}

// ============================================================
// 二、使用 AsterTraderTestSuite 运行通用测试
// ============================================================

// TestAsterTrader_InterfaceCompliance 测试接口兼容性
func TestAsterTrader_InterfaceCompliance(t *testing.T) {
	var _ Trader = (*AsterTrader)(nil)
}

// TestAsterTrader_CommonInterface 使用测试套件运行所有通用接口测试
func TestAsterTrader_CommonInterface(t *testing.T) {
	// 创建测试套件
	suite := NewAsterTraderTestSuite(t)
	defer suite.Cleanup()

	// 运行所有通用接口测试
	suite.RunAllTests()
}

// ============================================================
// 三、Aster 特定功能的单元测试
// ============================================================

// TestNewAsterTrader 测试创建 Aster 交易器
func TestNewAsterTrader(t *testing.T) {
	tests := []struct {
		name          string
		user          string
		signer        string
		privateKeyHex string
		wantError     bool
		errorContains string
	}{
		{
			name:          "成功创建",
			user:          "0x1234567890123456789012345678901234567890",
			signer:        "0xabcdefabcdefabcdefabcdefabcdefabcdefabcd",
			privateKeyHex: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			wantError:     false,
		},
		{
			name:          "无效私钥格式",
			user:          "0x1234567890123456789012345678901234567890",
			signer:        "0xabcdefabcdefabcdefabcdefabcdefabcdefabcd",
			privateKeyHex: "invalid_key",
			wantError:     true,
			errorContains: "解析私钥失败",
		},
		{
			name:          "带0x前缀的私钥",
			user:          "0x1234567890123456789012345678901234567890",
			signer:        "0xabcdefabcdefabcdefabcdefabcdefabcdefabcd",
			privateKeyHex: "0x0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			wantError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trader, err := NewAsterTrader(tt.user, tt.signer, tt.privateKeyHex)

			if tt.wantError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				assert.Nil(t, trader)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, trader)
				if trader != nil {
					assert.Equal(t, tt.user, trader.user)
					assert.Equal(t, tt.signer, trader.signer)
					assert.NotNil(t, trader.privateKey)
				}
			}
		})
	}
}

// ============================================================
// 三、重试机制测试（使用 Mock HTTP Server）
// ============================================================

// TestCancelAllOrdersWithRetry_Success_FirstAttempt 测试第一次就成功（无需重试）
func TestCancelAllOrdersWithRetry_Success_FirstAttempt(t *testing.T) {
	callCount := 0

	// 创建 mock server：第一次就成功
	mockServer := startIPv4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/fapi/v3/allOpenOrders" && r.Method == "DELETE" {
			callCount++
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"code": 200,
				"msg":  "success",
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockServer.Close()

	// 创建 trader（临时使用测试私钥）
	trader, _ := NewAsterTrader(
		"0x1234567890123456789012345678901234567890",
		"0xabcdefabcdefabcdefabcdefabcdefabcdefabcd",
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
	)
	trader.baseURL = mockServer.URL

	// 执行测试
	err := trader.CancelAllOrdersWithRetry("BTCUSDT", 3)

	// 验证：成功且只调用1次
	assert.NoError(t, err)
	assert.Equal(t, 1, callCount, "第一次成功时不应重试")
}

// TestCancelAllOrdersWithRetry_Success_SecondAttempt 测试第二次才成功
func TestCancelAllOrdersWithRetry_Success_SecondAttempt(t *testing.T) {
	callCount := 0

	// Mock：第一次失败，第二次成功
	mockServer := startIPv4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/fapi/v3/allOpenOrders" && r.Method == "DELETE" {
			callCount++
			if callCount == 1 {
				// 第一次失败
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"code": -1001,
					"msg":  "Internal server error",
				})
				return
			}
			// 第二次成功
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"code": 200,
				"msg":  "success",
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockServer.Close()

	trader, _ := NewAsterTrader(
		"0x1234567890123456789012345678901234567890",
		"0xabcdefabcdefabcdefabcdefabcdefabcdefabcd",
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
	)
	trader.baseURL = mockServer.URL

	// 执行测试（记录开始时间）
	startTime := time.Now()
	err := trader.CancelAllOrdersWithRetry("BTCUSDT", 3)
	duration := time.Since(startTime)

	// 验证：成功、调用2次、延迟约1秒
	assert.NoError(t, err)
	assert.Equal(t, 2, callCount, "第二次成功时应该调用2次")
	assert.GreaterOrEqual(t, duration, 1*time.Second, "第一次失败应延迟1秒后重试")
	assert.Less(t, duration, 2*time.Second, "不应超过2秒（1秒延迟+容错）")
}

// TestCancelAllOrdersWithRetry_Success_ThirdAttempt 测试第三次才成功
func TestCancelAllOrdersWithRetry_Success_ThirdAttempt(t *testing.T) {
	callCount := 0

	mockServer := startIPv4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/fapi/v3/allOpenOrders" && r.Method == "DELETE" {
			callCount++
			if callCount <= 2 {
				// 前两次失败
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"code": -1001,
					"msg":  fmt.Sprintf("attempt %d failed", callCount),
				})
				return
			}
			// 第三次成功
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"code": 200,
				"msg":  "success",
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockServer.Close()

	trader, _ := NewAsterTrader(
		"0x1234567890123456789012345678901234567890",
		"0xabcdefabcdefabcdefabcdefabcdefabcdefabcd",
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
	)
	trader.baseURL = mockServer.URL

	startTime := time.Now()
	err := trader.CancelAllOrdersWithRetry("BTCUSDT", 3)
	duration := time.Since(startTime)

	assert.NoError(t, err)
	assert.Equal(t, 3, callCount, "第三次成功时应该调用3次")
	assert.GreaterOrEqual(t, duration, 3*time.Second, "两次重试应延迟1s+2s=3s")
	assert.Less(t, duration, 4*time.Second, "不应超过4秒（3秒延迟+容错）")
}

// TestCancelAllOrdersWithRetry_AllFailed 测试所有重试都失败
func TestCancelAllOrdersWithRetry_AllFailed(t *testing.T) {
	callCount := 0

	mockServer := startIPv4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/fapi/v3/allOpenOrders" && r.Method == "DELETE" {
			callCount++
			// 所有调用都失败
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"code": -1003,
				"msg":  "persistent network failure",
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockServer.Close()

	trader, _ := NewAsterTrader(
		"0x1234567890123456789012345678901234567890",
		"0xabcdefabcdefabcdefabcdefabcdefabcdefabcd",
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
	)
	trader.baseURL = mockServer.URL

	startTime := time.Now()
	err := trader.CancelAllOrdersWithRetry("BTCUSDT", 3)
	duration := time.Since(startTime)

	assert.Error(t, err)
	assert.Equal(t, 3, callCount, "应该重试3次")
	assert.Contains(t, err.Error(), "重試 3 次後仍失敗", "错误信息应包含重试次数")
	assert.GreaterOrEqual(t, duration, 3*time.Second, "3次失败应延迟1s+2s=3s")
	assert.Less(t, duration, 4*time.Second, "不应超过4秒（3秒延迟+容错）")
}

// TestCancelAllOrdersWithRetry_RetryIntervals 测试重试间隔递增
func TestCancelAllOrdersWithRetry_RetryIntervals(t *testing.T) {
	var timestamps []time.Time

	mockServer := startIPv4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/fapi/v3/allOpenOrders" && r.Method == "DELETE" {
			timestamps = append(timestamps, time.Now())
			// 所有调用都失败
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"code": -1001,
				"msg":  "always fail",
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockServer.Close()

	trader, _ := NewAsterTrader(
		"0x1234567890123456789012345678901234567890",
		"0xabcdefabcdefabcdefabcdefabcdefabcdefabcd",
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
	)
	trader.baseURL = mockServer.URL

	_ = trader.CancelAllOrdersWithRetry("BTCUSDT", 3)

	assert.Len(t, timestamps, 3, "应该调用3次")

	if len(timestamps) == 3 {
		interval1 := timestamps[1].Sub(timestamps[0])
		assert.GreaterOrEqual(t, interval1, 1*time.Second, "第1次重试应延迟1秒")
		assert.Less(t, interval1, 1500*time.Millisecond, "第1次重试延迟应接近1秒")

		interval2 := timestamps[2].Sub(timestamps[1])
		assert.GreaterOrEqual(t, interval2, 2*time.Second, "第2次重试应延迟2秒")
		assert.Less(t, interval2, 2500*time.Millisecond, "第2次重试延迟应接近2秒")
	}
}

// TestCancelAllOrdersWithRetry_MaxRetries 测试自定义最大重试次数
func TestCancelAllOrdersWithRetry_MaxRetries(t *testing.T) {
	tests := []struct {
		name          string
		maxRetries    int
		expectedCalls int
		expectedDelay time.Duration
		maxDelay      time.Duration
	}{
		{
			name:          "重试1次（总共2次调用）",
			maxRetries:    2,
			expectedCalls: 2,
			expectedDelay: 1 * time.Second,
			maxDelay:      2 * time.Second,
		},
		{
			name:          "重试4次（总共5次调用）",
			maxRetries:    5,
			expectedCalls: 5,
			expectedDelay: 10 * time.Second,
			maxDelay:      11 * time.Second,
		},
		{
			name:          "无重试（仅1次调用）",
			maxRetries:    1,
			expectedCalls: 1,
			expectedDelay: 0 * time.Second,
			maxDelay:      500 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callCount := 0

			mockServer := startIPv4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				path := r.URL.Path
				if path == "/fapi/v3/allOpenOrders" && r.Method == "DELETE" {
					callCount++
					w.WriteHeader(http.StatusInternalServerError)
					json.NewEncoder(w).Encode(map[string]interface{}{
						"code": -1001,
						"msg":  "always fail",
					})
					return
				}
				w.WriteHeader(http.StatusNotFound)
			}))
			defer mockServer.Close()

			trader, _ := NewAsterTrader(
				"0x1234567890123456789012345678901234567890",
				"0xabcdefabcdefabcdefabcdefabcdefabcdefabcd",
				"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			)
			trader.baseURL = mockServer.URL

			startTime := time.Now()
			err := trader.CancelAllOrdersWithRetry("ETHUSDT", tt.maxRetries)
			duration := time.Since(startTime)

			assert.Error(t, err)
			assert.Equal(t, tt.expectedCalls, callCount, "调用次数应该等于maxRetries")
			assert.GreaterOrEqual(t, duration, tt.expectedDelay, "延迟应该符合预期")
			assert.Less(t, duration, tt.maxDelay, "延迟不应超过最大值")
		})
	}
}
