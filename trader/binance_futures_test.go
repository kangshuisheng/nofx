package trader

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/adshao/go-binance/v2/futures"
	"github.com/stretchr/testify/assert"
)

// ============================================================
// 一、BinanceFuturesTestSuite - 继承 base test suite
// ============================================================

// BinanceFuturesTestSuite 币安合约交易器测试套件
// 继承 TraderTestSuite 并添加 Binance Futures 特定的 mock 逻辑
type BinanceFuturesTestSuite struct {
	*TraderTestSuite // 嵌入基础测试套件
	mockServer       *httptest.Server
}

// NewBinanceFuturesTestSuite 创建币安合约测试套件
func NewBinanceFuturesTestSuite(t *testing.T) *BinanceFuturesTestSuite {
	// 创建 mock HTTP 服务器
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 根据不同的 URL 路径返回不同的 mock 响应
		path := r.URL.Path

		var respBody interface{}

		switch {
		// Mock GetBalance - /fapi/v2/balance
		case path == "/fapi/v2/balance":
			respBody = []map[string]interface{}{
				{
					"accountAlias":       "test",
					"asset":              "USDT",
					"balance":            "10000.00",
					"crossWalletBalance": "10000.00",
					"crossUnPnl":         "100.50",
					"availableBalance":   "8000.00",
					"maxWithdrawAmount":  "8000.00",
				},
			}

		// Mock GetAccount - /fapi/v2/account
		case path == "/fapi/v2/account":
			respBody = map[string]interface{}{
				"totalWalletBalance":    "10000.00",
				"availableBalance":      "8000.00",
				"totalUnrealizedProfit": "100.50",
				"assets": []map[string]interface{}{
					{
						"asset":                  "USDT",
						"walletBalance":          "10000.00",
						"unrealizedProfit":       "100.50",
						"marginBalance":          "10100.50",
						"maintMargin":            "200.00",
						"initialMargin":          "2000.00",
						"positionInitialMargin":  "2000.00",
						"openOrderInitialMargin": "0.00",
						"crossWalletBalance":     "10000.00",
						"crossUnPnl":             "100.50",
						"availableBalance":       "8000.00",
						"maxWithdrawAmount":      "8000.00",
					},
				},
			}

		// Mock GetPositions - /fapi/v2/positionRisk
		case path == "/fapi/v2/positionRisk":
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

		// Mock GetMarketPrice - /fapi/v1/ticker/price and /fapi/v2/ticker/price
		case path == "/fapi/v1/ticker/price" || path == "/fapi/v2/ticker/price":
			symbol := r.URL.Query().Get("symbol")
			if symbol == "" {
				// 返回所有价格
				respBody = []map[string]interface{}{
					{"Symbol": "BTCUSDT", "Price": "50000.00", "Time": 1234567890},
					{"Symbol": "ETHUSDT", "Price": "3000.00", "Time": 1234567890},
				}
			} else if symbol == "INVALIDUSDT" {
				// 返回错误
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"code": -1121,
					"msg":  "Invalid symbol.",
				})
				return
			} else {
				// 返回单个价格（注意：即使有 symbol 参数，也要返回数组）
				price := "50000.00"
				if symbol == "ETHUSDT" {
					price = "3000.00"
				}
				respBody = []map[string]interface{}{
					{
						"Symbol": symbol,
						"Price":  price,
						"Time":   1234567890,
					},
				}
			}

		// Mock ExchangeInfo - /fapi/v1/exchangeInfo
		case path == "/fapi/v1/exchangeInfo":
			respBody = map[string]interface{}{
				"symbols": []map[string]interface{}{
					{
						"symbol":             "BTCUSDT",
						"status":             "TRADING",
						"baseAsset":          "BTC",
						"quoteAsset":         "USDT",
						"pricePrecision":     2,
						"quantityPrecision":  3,
						"baseAssetPrecision": 8,
						"quotePrecision":     8,
						"filters": []map[string]interface{}{
							{
								"filterType": "PRICE_FILTER",
								"minPrice":   "0.01",
								"maxPrice":   "1000000",
								"tickSize":   "0.01",
							},
							{
								"filterType": "LOT_SIZE",
								"minQty":     "0.001",
								"maxQty":     "10000",
								"stepSize":   "0.001",
							},
						},
					},
					{
						"symbol":             "ETHUSDT",
						"status":             "TRADING",
						"baseAsset":          "ETH",
						"quoteAsset":         "USDT",
						"pricePrecision":     2,
						"quantityPrecision":  3,
						"baseAssetPrecision": 8,
						"quotePrecision":     8,
						"filters": []map[string]interface{}{
							{
								"filterType": "PRICE_FILTER",
								"minPrice":   "0.01",
								"maxPrice":   "100000",
								"tickSize":   "0.01",
							},
							{
								"filterType": "LOT_SIZE",
								"minQty":     "0.001",
								"maxQty":     "10000",
								"stepSize":   "0.001",
							},
						},
					},
				},
			}

		// Mock CreateOrder - /fapi/v1/order (POST)
		case path == "/fapi/v1/order" && r.Method == "POST":
			symbol := r.FormValue("symbol")
			if symbol == "" {
				symbol = "BTCUSDT"
			}
			respBody = map[string]interface{}{
				"orderId":       123456,
				"symbol":        symbol,
				"status":        "FILLED",
				"clientOrderId": r.FormValue("newClientOrderId"),
				"price":         r.FormValue("price"),
				"avgPrice":      r.FormValue("price"),
				"origQty":       r.FormValue("quantity"),
				"executedQty":   r.FormValue("quantity"),
				"cumQty":        r.FormValue("quantity"),
				"cumQuote":      "1000.00",
				"timeInForce":   r.FormValue("timeInForce"),
				"type":          r.FormValue("type"),
				"reduceOnly":    r.FormValue("reduceOnly") == "true",
				"side":          r.FormValue("side"),
				"positionSide":  r.FormValue("positionSide"),
				"stopPrice":     r.FormValue("stopPrice"),
				"workingType":   r.FormValue("workingType"),
			}

		// Mock CancelOrder - /fapi/v1/order (DELETE)
		case path == "/fapi/v1/order" && r.Method == "DELETE":
			respBody = map[string]interface{}{
				"orderId": 123456,
				"symbol":  r.URL.Query().Get("symbol"),
				"status":  "CANCELED",
			}

		// Mock ListOpenOrders - /fapi/v1/openOrders
		case path == "/fapi/v1/openOrders":
			// 根據 symbol 參數返回不同的測試數據
			symbol := r.URL.Query().Get("symbol")
			if symbol == "BTCUSDT" {
				respBody = []map[string]interface{}{
					{
						"orderId":      int64(111111),
						"symbol":       "BTCUSDT",
						"status":       "NEW",
						"type":         "LIMIT",
						"side":         "BUY",
						"positionSide": "LONG",
						"price":        "45000.00",
						"origQty":      "0.01",
						"stopPrice":    "0",
					},
					{
						"orderId":      int64(222222),
						"symbol":       "BTCUSDT",
						"status":       "NEW",
						"type":         "STOP_MARKET",
						"side":         "SELL",
						"positionSide": "LONG",
						"price":        "0",
						"origQty":      "0.01",
						"stopPrice":    "44000.00",
					},
				}
			} else if symbol == "" {
				// 查詢所有幣種
				respBody = []map[string]interface{}{
					{
						"orderId":      int64(111111),
						"symbol":       "BTCUSDT",
						"status":       "NEW",
						"type":         "LIMIT",
						"side":         "BUY",
						"positionSide": "LONG",
						"price":        "45000.00",
						"origQty":      "0.01",
						"stopPrice":    "0",
					},
					{
						"orderId":      int64(333333),
						"symbol":       "ETHUSDT",
						"status":       "NEW",
						"type":         "LIMIT",
						"side":         "SELL",
						"positionSide": "SHORT",
						"price":        "2900.00",
						"origQty":      "0.1",
						"stopPrice":    "0",
					},
				}
			} else {
				// 其他幣種返回空
				respBody = []map[string]interface{}{}
			}

		// Mock CancelAllOrders - /fapi/v1/allOpenOrders (DELETE)
		case path == "/fapi/v1/allOpenOrders" && r.Method == "DELETE":
			respBody = map[string]interface{}{
				"code": 200,
				"msg":  "The operation of cancel all open order is done.",
			}

		// Mock SetLeverage - /fapi/v1/leverage
		case path == "/fapi/v1/leverage":
			// 将字符串转换为整数
			leverageStr := r.FormValue("leverage")
			leverage := 10 // 默认值
			if leverageStr != "" {
				// 注意：这里我们直接返回整数，而不是字符串
				fmt.Sscanf(leverageStr, "%d", &leverage)
			}
			respBody = map[string]interface{}{
				"leverage":         leverage,
				"maxNotionalValue": "1000000",
				"symbol":           r.FormValue("symbol"),
			}

		// Mock SetMarginType - /fapi/v1/marginType
		case path == "/fapi/v1/marginType":
			respBody = map[string]interface{}{
				"code": 200,
				"msg":  "success",
			}

		// Mock ChangePositionMode - /fapi/v1/positionSide/dual
		case path == "/fapi/v1/positionSide/dual":
			respBody = map[string]interface{}{
				"code": 200,
				"msg":  "success",
			}

		// Mock ServerTime - /fapi/v1/time
		case path == "/fapi/v1/time":
			respBody = map[string]interface{}{
				"serverTime": 1234567890000,
			}

		// Default: empty response
		default:
			respBody = map[string]interface{}{}
		}

		// 序列化响应
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(respBody)
	}))

	// 创建 futures.Client 并设置为使用 mock 服务器
	client := futures.NewClient("test_api_key", "test_secret_key")
	client.BaseURL = mockServer.URL
	client.HTTPClient = mockServer.Client()

	// 创建 FuturesTrader
	trader := &FuturesTrader{
		client:        client,
		cacheDuration: 0, // 禁用缓存以便测试
	}

	// 创建基础套件
	baseSuite := NewTraderTestSuite(t, trader)

	return &BinanceFuturesTestSuite{
		TraderTestSuite: baseSuite,
		mockServer:      mockServer,
	}
}

// Cleanup 清理资源
func (s *BinanceFuturesTestSuite) Cleanup() {
	if s.mockServer != nil {
		s.mockServer.Close()
	}
	s.TraderTestSuite.Cleanup()
}

// ============================================================
// 二、使用 BinanceFuturesTestSuite 运行通用测试
// ============================================================

// TestFuturesTrader_InterfaceCompliance 测试接口兼容性
func TestFuturesTrader_InterfaceCompliance(t *testing.T) {
	var _ Trader = (*FuturesTrader)(nil)
}

// TestFuturesTrader_CommonInterface 使用测试套件运行所有通用接口测试
func TestFuturesTrader_CommonInterface(t *testing.T) {
	// 创建测试套件
	suite := NewBinanceFuturesTestSuite(t)
	defer suite.Cleanup()

	// 运行所有通用接口测试
	suite.RunAllTests()
}

// ============================================================
// 三、币安合约特定功能的单元测试
// ============================================================

// TestNewFuturesTrader 测试创建币安合约交易器
func TestNewFuturesTrader(t *testing.T) {
	// 创建 mock HTTP 服务器
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		var respBody interface{}

		switch path {
		case "/fapi/v1/time":
			respBody = map[string]interface{}{
				"serverTime": 1234567890000,
			}
		case "/fapi/v1/positionSide/dual":
			respBody = map[string]interface{}{
				"code": 200,
				"msg":  "success",
			}
		default:
			respBody = map[string]interface{}{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(respBody)
	}))
	defer mockServer.Close()

	// 测试成功创建
	trader := NewFuturesTrader("test_api_key", "test_secret_key", "test_user", "market_only", -0.03, 60)

	// 修改 client 使用 mock server
	trader.client.BaseURL = mockServer.URL
	trader.client.HTTPClient = mockServer.Client()

	assert.NotNil(t, trader)
	assert.NotNil(t, trader.client)
	assert.Equal(t, 15*time.Second, trader.cacheDuration)
}

// TestCalculatePositionSize 测试仓位计算
func TestCalculatePositionSize(t *testing.T) {
	trader := &FuturesTrader{}

	tests := []struct {
		name         string
		balance      float64
		riskPercent  float64
		price        float64
		leverage     int
		wantQuantity float64
	}{
		{
			name:         "正常计算",
			balance:      10000,
			riskPercent:  2,
			price:        50000,
			leverage:     10,
			wantQuantity: 0.04, // (10000 * 0.02 * 10) / 50000 = 0.04
		},
		{
			name:         "高杠杆",
			balance:      10000,
			riskPercent:  1,
			price:        3000,
			leverage:     20,
			wantQuantity: 0.6667, // (10000 * 0.01 * 20) / 3000 = 0.6667
		},
		{
			name:         "低风险",
			balance:      5000,
			riskPercent:  0.5,
			price:        50000,
			leverage:     5,
			wantQuantity: 0.0025, // (5000 * 0.005 * 5) / 50000 = 0.0025
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			quantity := trader.CalculatePositionSize(tt.balance, tt.riskPercent, tt.price, tt.leverage)
			assert.InDelta(t, tt.wantQuantity, quantity, 0.0001, "计算的仓位数量不正确")
		})
	}
}

// TestGetBrOrderID 测试订单ID生成
func TestGetBrOrderID(t *testing.T) {
	// 测试3次，确保每次生成的ID都不同
	ids := make(map[string]bool)
	for i := 0; i < 3; i++ {
		id := getBrOrderID()

		// 检查格式
		assert.True(t, strings.HasPrefix(id, "x-KzrpZaP9"), "订单ID应以x-KzrpZaP9开头")

		// 检查长度（应该 <= 32）
		assert.LessOrEqual(t, len(id), 32, "订单ID长度不应超过32字符")

		// 检查唯一性
		assert.False(t, ids[id], "订单ID应该唯一")
		ids[id] = true
	}
}

// ============================================================
// 四、缓存管理测试
// ============================================================

// TestInvalidateBalanceCache 测试清除余额缓存
func TestInvalidateBalanceCache(t *testing.T) {
	suite := NewBinanceFuturesTestSuite(t)
	defer suite.Cleanup()

	trader := suite.Trader.(*FuturesTrader)
	trader.cacheDuration = 1 * time.Hour // 启用长时间缓存以便测试

	// 1. 第一次调用 GetBalance 填充缓存
	balance1, err := trader.GetBalance()
	assert.NoError(t, err)
	assert.NotNil(t, balance1)

	// 验证缓存已被填充
	assert.NotNil(t, trader.cachedBalance, "缓存应该被填充")
	assert.False(t, trader.balanceCacheTime.IsZero(), "缓存时间应该被设置")

	// 2. 清除缓存
	trader.InvalidateBalanceCache()

	// 3. 验证缓存已被清除
	assert.Nil(t, trader.cachedBalance, "缓存应该被清除")
	assert.True(t, trader.balanceCacheTime.IsZero(), "缓存时间应该被重置为零值")

	// 4. 再次调用 GetBalance 应该重新从 API 获取（而非缓存）
	balance2, err := trader.GetBalance()
	assert.NoError(t, err)
	assert.NotNil(t, balance2)
	assert.NotNil(t, trader.cachedBalance, "缓存应该重新填充")
}

// TestInvalidatePositionsCache 测试清除持仓缓存
func TestInvalidatePositionsCache(t *testing.T) {
	suite := NewBinanceFuturesTestSuite(t)
	defer suite.Cleanup()

	trader := suite.Trader.(*FuturesTrader)
	trader.cacheDuration = 1 * time.Hour // 启用长时间缓存以便测试

	// 1. 第一次调用 GetPositions 填充缓存
	positions1, err := trader.GetPositions()
	assert.NoError(t, err)
	assert.NotNil(t, positions1)

	// 验证缓存已被填充
	assert.NotNil(t, trader.cachedPositions, "缓存应该被填充")
	assert.False(t, trader.positionsCacheTime.IsZero(), "缓存时间应该被设置")

	// 2. 清除缓存
	trader.InvalidatePositionsCache()

	// 3. 验证缓存已被清除
	assert.Nil(t, trader.cachedPositions, "缓存应该被清除")
	assert.True(t, trader.positionsCacheTime.IsZero(), "缓存时间应该被重置为零值")

	// 4. 再次调用 GetPositions 应该重新从 API 获取（而非缓存）
	positions2, err := trader.GetPositions()
	assert.NoError(t, err)
	assert.NotNil(t, positions2)
	assert.NotNil(t, trader.cachedPositions, "缓存应该重新填充")
}

// TestInvalidateAllCaches 测试清除所有缓存
func TestInvalidateAllCaches(t *testing.T) {
	suite := NewBinanceFuturesTestSuite(t)
	defer suite.Cleanup()

	trader := suite.Trader.(*FuturesTrader)
	trader.cacheDuration = 1 * time.Hour // 启用长时间缓存以便测试

	// 1. 填充所有缓存
	_, err := trader.GetBalance()
	assert.NoError(t, err)
	_, err = trader.GetPositions()
	assert.NoError(t, err)

	// 验证两个缓存都被填充
	assert.NotNil(t, trader.cachedBalance, "余额缓存应该被填充")
	assert.NotNil(t, trader.cachedPositions, "持仓缓存应该被填充")

	// 2. 清除所有缓存
	trader.InvalidateAllCaches()

	// 3. 验证所有缓存都被清除
	assert.Nil(t, trader.cachedBalance, "余额缓存应该被清除")
	assert.True(t, trader.balanceCacheTime.IsZero(), "余额缓存时间应该被重置")
	assert.Nil(t, trader.cachedPositions, "持仓缓存应该被清除")
	assert.True(t, trader.positionsCacheTime.IsZero(), "持仓缓存时间应该被重置")
}

// TestTradeOperationsInvalidateCache 测试交易操作自动清除缓存
func TestTradeOperationsInvalidateCache(t *testing.T) {
	suite := NewBinanceFuturesTestSuite(t)
	defer suite.Cleanup()

	trader := suite.Trader.(*FuturesTrader)
	trader.cacheDuration = 1 * time.Hour // 启用长时间缓存以便测试

	// 子测试1：OpenLong 后缓存被清除
	t.Run("OpenLong_invalidates_cache", func(t *testing.T) {
		// 填充缓存
		_, _ = trader.GetBalance()
		_, _ = trader.GetPositions()
		assert.NotNil(t, trader.cachedBalance, "开仓前余额缓存应该存在")
		assert.NotNil(t, trader.cachedPositions, "开仓前持仓缓存应该存在")

		// 执行开多仓
		_, err := trader.OpenLong("BTCUSDT", 0.01, 10)
		assert.NoError(t, err)

		// 验证缓存被清除
		assert.Nil(t, trader.cachedBalance, "开多仓后余额缓存应该被清除")
		assert.Nil(t, trader.cachedPositions, "开多仓后持仓缓存应该被清除")
	})

	// 子测试2：OpenShort 后缓存被清除
	t.Run("OpenShort_invalidates_cache", func(t *testing.T) {
		// 重新填充缓存
		_, _ = trader.GetBalance()
		_, _ = trader.GetPositions()
		assert.NotNil(t, trader.cachedBalance)
		assert.NotNil(t, trader.cachedPositions)

		// 执行开空仓
		_, err := trader.OpenShort("ETHUSDT", 0.004, 5)
		assert.NoError(t, err)

		// 验证缓存被清除
		assert.Nil(t, trader.cachedBalance, "开空仓后余额缓存应该被清除")
		assert.Nil(t, trader.cachedPositions, "开空仓后持仓缓存应该被清除")
	})

	// 子测试3：CloseLong 后缓存被清除
	t.Run("CloseLong_invalidates_cache", func(t *testing.T) {
		// 重新填充缓存
		_, _ = trader.GetBalance()
		_, _ = trader.GetPositions()
		assert.NotNil(t, trader.cachedBalance)

		// 执行平多仓
		_, err := trader.CloseLong("BTCUSDT", 0.01)
		assert.NoError(t, err)

		// 验证缓存被清除
		assert.Nil(t, trader.cachedBalance, "平多仓后余额缓存应该被清除")
		assert.Nil(t, trader.cachedPositions, "平多仓后持仓缓存应该被清除")
	})

	// 子测试4：CloseShort 后缓存被清除
	t.Run("CloseShort_invalidates_cache", func(t *testing.T) {
		// 重新填充缓存
		_, _ = trader.GetBalance()
		_, _ = trader.GetPositions()

		// 执行平空仓
		_, err := trader.CloseShort("ETHUSDT", 0.004)
		assert.NoError(t, err)

		// 验证缓存被清除
		assert.Nil(t, trader.cachedBalance, "平空仓后余额缓存应该被清除")
		assert.Nil(t, trader.cachedPositions, "平空仓后持仓缓存应该被清除")
	})

	// 子测试5：SetStopLoss 后持仓缓存被清除
	t.Run("SetStopLoss_invalidates_positions_cache", func(t *testing.T) {
		// 重新填充缓存
		_, _ = trader.GetPositions()
		assert.NotNil(t, trader.cachedPositions)

		// 设置止损
		err := trader.SetStopLoss("BTCUSDT", "LONG", 0.01, 45000.0)
		assert.NoError(t, err)

		// 验证持仓缓存被清除（止损单会影响持仓信息）
		assert.Nil(t, trader.cachedPositions, "设置止损后持仓缓存应该被清除")
	})

	// 子测试6：SetTakeProfit 后持仓缓存被清除
	t.Run("SetTakeProfit_invalidates_positions_cache", func(t *testing.T) {
		// 重新填充缓存
		_, _ = trader.GetPositions()
		assert.NotNil(t, trader.cachedPositions)

		// 设置止盈
		err := trader.SetTakeProfit("BTCUSDT", "LONG", 0.01, 55000.0)
		assert.NoError(t, err)

		// 验证持仓缓存被清除
		assert.Nil(t, trader.cachedPositions, "设置止盈后持仓缓存应该被清除")
	})
}

// ============================================================
// 五、GetOpenOrders 测试
// ============================================================

// TestGetOpenOrders_SpecificSymbol 测试查询特定币种的未成交订单
func TestGetOpenOrders_SpecificSymbol(t *testing.T) {
	suite := NewBinanceFuturesTestSuite(t)
	defer suite.Cleanup()

	trader := suite.Trader.(*FuturesTrader)

	// 查询 BTCUSDT 的未成交订单
	orders, err := trader.GetOpenOrders("BTCUSDT")

	// 验证
	assert.NoError(t, err)
	assert.Len(t, orders, 2, "应该返回2个订单")

	// 验证第一个订单（限价单）
	assert.Equal(t, "BTCUSDT", orders[0].Symbol)
	assert.Equal(t, int64(111111), orders[0].OrderID)
	assert.Equal(t, "LIMIT", orders[0].Type)
	assert.Equal(t, "BUY", orders[0].Side)
	assert.Equal(t, "LONG", orders[0].PositionSide)
	assert.Equal(t, 45000.0, orders[0].Price)
	assert.Equal(t, 0.01, orders[0].Quantity)
	assert.Equal(t, 0.0, orders[0].StopPrice)

	// 验证第二个订单（止损单）
	assert.Equal(t, "BTCUSDT", orders[1].Symbol)
	assert.Equal(t, int64(222222), orders[1].OrderID)
	assert.Equal(t, "STOP_MARKET", orders[1].Type)
	assert.Equal(t, "SELL", orders[1].Side)
	assert.Equal(t, 44000.0, orders[1].StopPrice)
}

// TestGetOpenOrders_AllSymbols 测试查询所有币种的未成交订单
func TestGetOpenOrders_AllSymbols(t *testing.T) {
	suite := NewBinanceFuturesTestSuite(t)
	defer suite.Cleanup()

	trader := suite.Trader.(*FuturesTrader)

	// 查询所有未成交订单（symbol 为空字符串）
	orders, err := trader.GetOpenOrders("")

	// 验证
	assert.NoError(t, err)
	assert.Len(t, orders, 2, "应该返回2个订单（BTCUSDT和ETHUSDT）")

	// 验证包含不同币种
	symbols := make(map[string]bool)
	for _, order := range orders {
		symbols[order.Symbol] = true
	}
	assert.True(t, symbols["BTCUSDT"], "应该包含BTCUSDT订单")
	assert.True(t, symbols["ETHUSDT"], "应该包含ETHUSDT订单")
}

// TestGetOpenOrders_EmptyResult 测试查询无订单的币种
func TestGetOpenOrders_EmptyResult(t *testing.T) {
	suite := NewBinanceFuturesTestSuite(t)
	defer suite.Cleanup()

	trader := suite.Trader.(*FuturesTrader)

	// 查询没有订单的币种
	orders, err := trader.GetOpenOrders("XRPUSDT")

	// 验证
	assert.NoError(t, err)
	assert.Empty(t, orders, "应该返回空数组")
}

// TestGetOpenOrders_DifferentOrderTypes 测试不同类型的订单
func TestGetOpenOrders_DifferentOrderTypes(t *testing.T) {
	suite := NewBinanceFuturesTestSuite(t)
	defer suite.Cleanup()

	trader := suite.Trader.(*FuturesTrader)

	orders, err := trader.GetOpenOrders("BTCUSDT")
	assert.NoError(t, err)

	// 验证包含不同类型的订单
	orderTypes := make(map[string]bool)
	for _, order := range orders {
		orderTypes[order.Type] = true
	}

	assert.True(t, orderTypes["LIMIT"], "应该包含限价单")
	assert.True(t, orderTypes["STOP_MARKET"], "应该包含止损单")
}

// TestGetOpenOrders_ParseFloatValues 测试价格和数量解析
func TestGetOpenOrders_ParseFloatValues(t *testing.T) {
	suite := NewBinanceFuturesTestSuite(t)
	defer suite.Cleanup()

	trader := suite.Trader.(*FuturesTrader)

	orders, err := trader.GetOpenOrders("BTCUSDT")
	assert.NoError(t, err)
	assert.NotEmpty(t, orders)

	// 验证所有价格和数量都被正确解析为浮点数
	for _, order := range orders {
		// 数量应该大于0
		assert.Greater(t, order.Quantity, 0.0, "数量应该大于0")

		// 价格或止损价至少有一个大于0
		hasValidPrice := order.Price > 0 || order.StopPrice > 0
		assert.True(t, hasValidPrice, "价格或止损价至少有一个应该大于0")
	}
}
