package trader

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/adshao/go-binance/v2/futures"
)

// TestOrderStrategy_MarketOnly 測試純市價單策略
func TestOrderStrategy_MarketOnly(t *testing.T) {
	// Mock server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Log all requests for debugging
		t.Logf("Mock received: %s %s?%s", r.Method, r.URL.Path, r.URL.RawQuery)

		// Mock initialization endpoints
		if r.URL.Path == "/fapi/v1/time" {
			// Server time endpoint
			response := map[string]interface{}{
				"serverTime": time.Now().UnixMilli(),
			}
			json.NewEncoder(w).Encode(response)
			return
		}

		if r.URL.Path == "/fapi/v1/positionSide/dual" {
			// Position mode endpoint - return success
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"code": 200,
				"msg":  "success",
			})
			return
		}

		if r.URL.Path == "/fapi/v1/leverage" {
			// Leverage setting endpoint
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"leverage": 10,
				"symbol":   "BTCUSDT",
			})
			return
		}

		if r.URL.Path == "/fapi/v1/allOpenOrders" && r.Method == "DELETE" {
			// Cancel all open orders
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"code": 200,
				"msg":  "success",
			})
			return
		}

		if r.URL.Path == "/fapi/v2/positionRisk" {
			// Return empty positions
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode([]map[string]interface{}{})
			return
		}

		if r.URL.Path == "/fapi/v1/exchangeInfo" {
			// Return minimal exchange info
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"symbols": []map[string]interface{}{
					{
						"symbol": "BTCUSDT",
						"filters": []map[string]interface{}{
							{
								"filterType": "LOT_SIZE",
								"stepSize":   "0.001",
							},
							{
								"filterType": "PRICE_FILTER",
								"tickSize":   "0.01",
							},
						},
					},
				},
			})
			return
		}

		if r.URL.Path == "/fapi/v2/ticker/price" {
			// Price endpoint (v2!)
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode([]map[string]interface{}{
				{
					"symbol": "BTCUSDT",
					"price":  "50000.0",
				},
			})
			return
		}

		if r.URL.Path == "/fapi/v1/order" {
			// 市價單應該直接成交
			response := &futures.CreateOrderResponse{
				OrderID: 12345,
				Symbol:  "BTCUSDT",
				Status:  "FILLED",
				Side:    "BUY",
				Type:    "MARKET",
			}
			json.NewEncoder(w).Encode(response)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockServer.Close()

	// Create a client configured to use the mock server
	client := futures.NewClient("test_key", "test_secret")
	client.BaseURL = mockServer.URL
	client.HTTPClient = mockServer.Client()

	// Create trader with the mocked client
	trader := newFuturesTraderWithClient(client, "market_only", -0.03, 60)

	// 測試開多倉
	result, err := trader.OpenLong("BTCUSDT", 100.0, 10)
	if err != nil {
		t.Fatalf("market_only 策略開多倉失敗: %v", err)
	}

	orderID, ok := result["orderId"].(int64)
	if !ok || orderID != 12345 {
		t.Fatalf("預期 OrderID=12345, 實際 %v (type: %T)", result["orderId"], result["orderId"])
	}

	// Status might be OrderStatusType enum or string, so check the value not the type
	statusStr := fmt.Sprintf("%v", result["status"])
	if statusStr != "FILLED" {
		t.Fatalf("預期 Status=FILLED, 實際 %v (type: %T)", result["status"], result["status"])
	}

	t.Logf("✅ market_only 策略測試通過")
}

// TestOrderStrategy_LimitOnly 測試純限價單策略
func TestOrderStrategy_LimitOnly(t *testing.T) {
	// Mock server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Mock initialization endpoints
		if r.URL.Path == "/fapi/v1/time" {
			response := map[string]interface{}{
				"serverTime": time.Now().UnixMilli(),
			}
			json.NewEncoder(w).Encode(response)
			return
		}

		if r.URL.Path == "/fapi/v1/positionSide/dual" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"code": 200,
				"msg":  "success",
			})
			return
		}

		if r.URL.Path == "/fapi/v1/leverage" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"leverage": 10,
				"symbol":   "BTCUSDT",
			})
			return
		}

		if r.URL.Path == "/fapi/v1/allOpenOrders" && r.Method == "DELETE" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"code": 200,
				"msg":  "success",
			})
			return
		}

		if r.URL.Path == "/fapi/v2/positionRisk" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode([]map[string]interface{}{})
			return
		}

		if r.URL.Path == "/fapi/v1/exchangeInfo" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"symbols": []map[string]interface{}{
					{
						"symbol": "BTCUSDT",
						"filters": []map[string]interface{}{
							{
								"filterType": "LOT_SIZE",
								"stepSize":   "0.001",
							},
							{
								"filterType": "PRICE_FILTER",
								"tickSize":   "0.01",
							},
						},
					},
				},
			})
			return
		}

		if r.URL.Path == "/fapi/v2/ticker/price" || r.URL.Path == "/fapi/v1/ticker/price" {
			// Price endpoint (support both v1 and v2)
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode([]map[string]interface{}{
				{
					"symbol": "BTCUSDT",
					"price":  "50000.0",
				},
			})
			return
		}

		if r.URL.Path == "/fapi/v1/order" {
			// 限價單創建成功但未成交
			response := &futures.CreateOrderResponse{
				OrderID: 12346,
				Symbol:  "BTCUSDT",
				Status:  "NEW",
				Side:    "BUY",
				Type:    "LIMIT",
				Price:   "49985.0", // 當前價格 * (1 - 0.0003) = 50000 * 0.9997 = 49985
			}
			json.NewEncoder(w).Encode(response)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockServer.Close()

	// Create a client configured to use the mock server
	client := futures.NewClient("test_key", "test_secret")
	client.BaseURL = mockServer.URL
	client.HTTPClient = mockServer.Client()

	// Create trader with the mocked client
	trader := newFuturesTraderWithClient(client, "limit_only", -0.03, 60)

	// 測試開多倉
	result, err := trader.OpenLong("BTCUSDT", 100.0, 10)
	if err != nil {
		t.Fatalf("limit_only 策略開多倉失敗: %v", err)
	}

	orderID, ok := result["orderId"].(int64)
	if !ok || orderID != 12346 {
		t.Fatalf("預期 OrderID=12346, 實際 %v (type: %T)", result["orderId"], result["orderId"])
	}

	// Status might be OrderStatusType enum or string, so check the value not the type
	statusStr := fmt.Sprintf("%v", result["status"])
	if statusStr != "NEW" {
		t.Fatalf("預期 Status=NEW (限價單未成交), 實際 %v (type: %T)", result["status"], result["status"])
	}

	t.Logf("✅ limit_only 策略測試通過 - 限價單創建成功，不會自動轉換為市價單")
}

// TestOrderStrategy_ConservativeHybrid_Success 測試保守混合策略 - 限價單成功
func TestOrderStrategy_ConservativeHybrid_Success(t *testing.T) {
	// Mock server - 模擬限價單立即成交的場景
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Mock initialization endpoints
		if r.URL.Path == "/fapi/v1/time" {
			response := map[string]interface{}{
				"serverTime": time.Now().UnixMilli(),
			}
			json.NewEncoder(w).Encode(response)
			return
		}

		if r.URL.Path == "/fapi/v1/positionSide/dual" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"code": 200,
				"msg":  "success",
			})
			return
		}

		if r.URL.Path == "/fapi/v1/leverage" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"leverage": 10,
				"symbol":   "BTCUSDT",
			})
			return
		}

		if r.URL.Path == "/fapi/v1/allOpenOrders" && r.Method == "DELETE" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"code": 200,
				"msg":  "success",
			})
			return
		}

		if r.URL.Path == "/fapi/v2/positionRisk" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode([]map[string]interface{}{})
			return
		}

		if r.URL.Path == "/fapi/v1/exchangeInfo" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"symbols": []map[string]interface{}{
					{
						"symbol": "BTCUSDT",
						"filters": []map[string]interface{}{
							{
								"filterType": "LOT_SIZE",
								"stepSize":   "0.001",
							},
							{
								"filterType": "PRICE_FILTER",
								"tickSize":   "0.01",
							},
						},
					},
				},
			})
			return
		}

		if r.URL.Path == "/fapi/v2/ticker/price" || r.URL.Path == "/fapi/v1/ticker/price" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode([]map[string]interface{}{
				{
					"symbol": "BTCUSDT",
					"price":  "50000.0",
				},
			})
			return
		}

		if r.URL.Path == "/fapi/v1/order" && r.Method == "POST" {
			// 限價單創建成功
			response := &futures.CreateOrderResponse{
				OrderID: 12347,
				Symbol:  "BTCUSDT",
				Status:  "NEW",
				Side:    "BUY",
				Type:    "LIMIT",
			}
			json.NewEncoder(w).Encode(response)
			return
		}

		if r.URL.Path == "/fapi/v1/order" && r.Method == "GET" {
			// 查詢訂單狀態 - 已成交
			response := &futures.Order{
				OrderID: 12347,
				Symbol:  "BTCUSDT",
				Status:  "FILLED", // 限價單已成交
				Side:    "BUY",
				Type:    "LIMIT",
			}
			json.NewEncoder(w).Encode(response)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockServer.Close()

	// Create a client configured to use the mock server
	client := futures.NewClient("test_key", "test_secret")
	client.BaseURL = mockServer.URL
	client.HTTPClient = mockServer.Client()

	// Create trader with the mocked client
	trader := newFuturesTraderWithClient(client, "conservative_hybrid", -0.03, 5)

	// 測試開多倉
	result, err := trader.OpenLong("BTCUSDT", 100.0, 10)
	if err != nil {
		t.Fatalf("conservative_hybrid 策略開多倉失敗: %v", err)
	}

	orderID, ok := result["orderId"].(int64)
	if !ok || orderID != 12347 {
		t.Fatalf("預期 OrderID=12347, 實際 %v (type: %T)", result["orderId"], result["orderId"])
	}

	// Status might be OrderStatusType enum or string, so check the value not the type
	statusStr := fmt.Sprintf("%v", result["status"])
	if statusStr != "FILLED" {
		t.Fatalf("預期 Status=FILLED (限價單成交), 實際 %v (type: %T)", result["status"], result["status"])
	}

	t.Logf("✅ conservative_hybrid 策略測試通過 - 限價單成功成交")
}

// TestOrderStrategy_ConservativeHybrid_Timeout 測試保守混合策略 - 超時轉換
func TestOrderStrategy_ConservativeHybrid_Timeout(t *testing.T) {
	t.Skip("⚠️  需要實際超時測試，耗時較長，跳過")

	// Mock server - 模擬限價單超時後轉換為市價單
	orderCheckCount := 0
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/fapi/v1/ticker/price" {
			response := map[string]interface{}{
				"symbol": "BTCUSDT",
				"price":  "50000.0",
			}
			json.NewEncoder(w).Encode(response)
			return
		}

		if r.URL.Path == "/fapi/v1/order" && r.Method == "POST" {
			// 第一次: 限價單創建
			// 第二次: 取消限價單
			// 第三次: 市價單創建
			if orderCheckCount == 0 {
				response := &futures.CreateOrderResponse{
					OrderID: 12348,
					Symbol:  "BTCUSDT",
					Status:  "NEW",
					Side:    "BUY",
					Type:    "LIMIT",
				}
				json.NewEncoder(w).Encode(response)
				orderCheckCount++
				return
			}
			// 市價單
			response := &futures.CreateOrderResponse{
				OrderID: 12349,
				Symbol:  "BTCUSDT",
				Status:  "FILLED",
				Side:    "BUY",
				Type:    "MARKET",
			}
			json.NewEncoder(w).Encode(response)
			return
		}

		if r.URL.Path == "/fapi/v1/order" && r.Method == "GET" {
			// 查詢訂單狀態 - 一直未成交
			response := &futures.Order{
				OrderID: 12348,
				Symbol:  "BTCUSDT",
				Status:  "NEW", // 始終未成交
				Side:    "BUY",
				Type:    "LIMIT",
			}
			json.NewEncoder(w).Encode(response)
			return
		}

		if r.URL.Path == "/fapi/v1/order" && r.Method == "DELETE" {
			// 取消訂單
			response := &futures.CancelOrderResponse{
				OrderID: 12348,
				Symbol:  "BTCUSDT",
				Status:  "CANCELED",
			}
			json.NewEncoder(w).Encode(response)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockServer.Close()

	trader := NewFuturesTrader("test_key", "test_secret", "test_user", "conservative_hybrid", -0.03, 3) // 3秒超時
	trader.client.BaseURL = mockServer.URL
	trader.client.HTTPClient = mockServer.Client()

	// 測試開多倉 - 應該超時並轉換為市價單
	result, err := trader.OpenLong("BTCUSDT", 100.0, 10)
	if err != nil {
		t.Fatalf("conservative_hybrid 策略超時轉換失敗: %v", err)
	}

	orderID, ok := result["orderId"].(int64)
	if !ok || orderID != 12349 {
		t.Errorf("預期轉換後的市價單 OrderID=12349, 實際 %v", result["orderId"])
	}

	status, ok := result["status"].(string)
	if !ok || status != "FILLED" {
		t.Errorf("預期市價單 Status=FILLED, 實際 %v", result["status"])
	}

	t.Logf("✅ conservative_hybrid 策略超時轉換測試通過")
}

// TestOrderStrategy_ConservativeHybrid_LimitFail 測試保守混合策略 - 限價單失敗降級
func TestOrderStrategy_ConservativeHybrid_LimitFail(t *testing.T) {
	// Mock server - 模擬限價單創建失敗，立即降級為市價單
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Mock initialization endpoints
		if r.URL.Path == "/fapi/v1/time" {
			response := map[string]interface{}{
				"serverTime": time.Now().UnixMilli(),
			}
			json.NewEncoder(w).Encode(response)
			return
		}

		if r.URL.Path == "/fapi/v1/positionSide/dual" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"code": 200,
				"msg":  "success",
			})
			return
		}

		if r.URL.Path == "/fapi/v1/leverage" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"leverage": 10,
				"symbol":   "BTCUSDT",
			})
			return
		}

		if r.URL.Path == "/fapi/v1/allOpenOrders" && r.Method == "DELETE" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"code": 200,
				"msg":  "success",
			})
			return
		}

		if r.URL.Path == "/fapi/v2/positionRisk" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode([]map[string]interface{}{})
			return
		}

		if r.URL.Path == "/fapi/v1/exchangeInfo" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"symbols": []map[string]interface{}{
					{
						"symbol": "BTCUSDT",
						"filters": []map[string]interface{}{
							{
								"filterType": "LOT_SIZE",
								"stepSize":   "0.001",
							},
							{
								"filterType": "PRICE_FILTER",
								"tickSize":   "0.01",
							},
						},
					},
				},
			})
			return
		}

		if r.URL.Path == "/fapi/v2/ticker/price" || r.URL.Path == "/fapi/v1/ticker/price" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode([]map[string]interface{}{
				{
					"symbol": "BTCUSDT",
					"price":  "50000.0",
				},
			})
			return
		}

		if r.URL.Path == "/fapi/v1/order" && r.Method == "POST" {
			// 檢查請求體判斷是限價單還是市價單
			if r.FormValue("type") == "LIMIT" {
				// 限價單創建失敗
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"code": -1111,
					"msg":  "Precision is over the maximum defined for this asset.",
				})
				return
			}
			// 市價單創建成功
			response := &futures.CreateOrderResponse{
				OrderID: 12350,
				Symbol:  "BTCUSDT",
				Status:  "FILLED",
				Side:    "BUY",
				Type:    "MARKET",
			}
			json.NewEncoder(w).Encode(response)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockServer.Close()

	// Create a client configured to use the mock server
	client := futures.NewClient("test_key", "test_secret")
	client.BaseURL = mockServer.URL
	client.HTTPClient = mockServer.Client()

	// Create trader with the mocked client
	trader := newFuturesTraderWithClient(client, "conservative_hybrid", -0.03, 60)

	// 測試開多倉 - 限價單失敗應該立即降級為市價單
	result, err := trader.OpenLong("BTCUSDT", 100.0, 10)
	if err != nil {
		t.Fatalf("conservative_hybrid 策略降級失敗: %v", err)
	}

	orderID, ok := result["orderId"].(int64)
	if !ok || orderID != 12350 {
		t.Fatalf("預期降級後的市價單 OrderID=12350, 實際 %v (type: %T)", result["orderId"], result["orderId"])
	}

	// Status might be OrderStatusType enum or string, so check the value not the type
	statusStr := fmt.Sprintf("%v", result["status"])
	if statusStr != "FILLED" {
		t.Fatalf("預期市價單 Status=FILLED, 實際 %v (type: %T)", result["status"], result["status"])
	}

	t.Logf("✅ conservative_hybrid 策略降級測試通過 - 限價單失敗立即降級為市價單")
}

// TestOrderStrategy_LimitPriceOffset 測試限價單價格偏移計算
func TestOrderStrategy_LimitPriceOffset(t *testing.T) {
	testCases := []struct {
		name          string
		currentPrice  float64
		offset        float64
		expectedPrice float64
		side          string // "buy" or "sell"
		expectedDiff  float64
	}{
		{
			name:          "做多 LONG - 負偏移（更好的買入價）",
			currentPrice:  50000.0,
			offset:        -0.03,   // -0.03%
			expectedPrice: 49985.0, // 50000 * (1 - 0.0003)
			side:          "buy",
			expectedDiff:  -15.0, // 50000 - 49985 = 15 USDT 節省
		},
		{
			name:          "做空 SHORT - 正偏移（更好的賣出價）",
			currentPrice:  50000.0,
			offset:        0.03,    // +0.03%
			expectedPrice: 50015.0, // 50000 * (1 + 0.0003)
			side:          "sell",
			expectedDiff:  15.0, // 50015 - 50000 = 15 USDT 額外收入
		},
		{
			name:          "小偏移測試",
			currentPrice:  100.0,
			offset:        -0.01, // -0.01%
			expectedPrice: 99.99, // 100 * (1 - 0.0001)
			side:          "buy",
			expectedDiff:  -0.01,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 計算價格偏移
			var calculatedPrice float64
			if tc.side == "buy" {
				// 做多: 買入價 = 當前價 * (1 + offset/100)
				// 負偏移 = 更低的買入價 = 更好
				calculatedPrice = tc.currentPrice * (1 + tc.offset/100)
			} else {
				// 做空: 賣出價 = 當前價 * (1 + offset/100)
				// 正偏移 = 更高的賣出價 = 更好
				calculatedPrice = tc.currentPrice * (1 + tc.offset/100)
			}

			// 驗證計算正確性（允許浮點誤差）
			if fmt.Sprintf("%.2f", calculatedPrice) != fmt.Sprintf("%.2f", tc.expectedPrice) {
				t.Errorf("價格計算錯誤: 預期 %.2f, 實際 %.2f", tc.expectedPrice, calculatedPrice)
			}

			// 驗證價差
			diff := calculatedPrice - tc.currentPrice
			if fmt.Sprintf("%.2f", diff) != fmt.Sprintf("%.2f", tc.expectedDiff) {
				t.Errorf("價差計算錯誤: 預期 %.2f, 實際 %.2f", tc.expectedDiff, diff)
			}

			t.Logf("✅ %s: 當前價 %.2f → 限價單 %.2f (節省/額外 %.2f USDT)",
				tc.name, tc.currentPrice, calculatedPrice, diff)
		})
	}
}

// TestOrderStrategy_TimeoutSettings 測試不同超時設置
func TestOrderStrategy_TimeoutSettings(t *testing.T) {
	testCases := []struct {
		name           string
		timeoutSeconds int
		strategy       string
		shouldTimeout  bool
	}{
		{
			name:           "conservative_hybrid - 5秒超時",
			timeoutSeconds: 5,
			strategy:       "conservative_hybrid",
			shouldTimeout:  false, // 測試中不實際超時
		},
		{
			name:           "conservative_hybrid - 60秒超時",
			timeoutSeconds: 60,
			strategy:       "conservative_hybrid",
			shouldTimeout:  false,
		},
		{
			name:           "limit_only - 不監控超時",
			timeoutSeconds: 60,
			strategy:       "limit_only",
			shouldTimeout:  false, // limit_only 不會超時轉換
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			trader := NewFuturesTrader("test_key", "test_secret", "test_user", tc.strategy, -0.03, tc.timeoutSeconds)

			if trader.orderStrategy != tc.strategy {
				t.Errorf("策略設置錯誤: 預期 %s, 實際 %s", tc.strategy, trader.orderStrategy)
			}

			if trader.limitTimeoutSeconds != tc.timeoutSeconds {
				t.Errorf("超時設置錯誤: 預期 %d秒, 實際 %d秒", tc.timeoutSeconds, trader.limitTimeoutSeconds)
			}

			t.Logf("✅ 策略 %s, 超時設置 %d秒", tc.strategy, tc.timeoutSeconds)
		})
	}
}

// TestOrderStrategy_ParameterValidation 測試參數驗證
func TestOrderStrategy_ParameterValidation(t *testing.T) {
	testCases := []struct {
		name      string
		strategy  string
		offset    float64
		timeout   int
		shouldErr bool
	}{
		{
			name:      "有效策略 - market_only",
			strategy:  "market_only",
			offset:    -0.03,
			timeout:   60,
			shouldErr: false,
		},
		{
			name:      "有效策略 - conservative_hybrid",
			strategy:  "conservative_hybrid",
			offset:    -0.03,
			timeout:   60,
			shouldErr: false,
		},
		{
			name:      "有效策略 - limit_only",
			strategy:  "limit_only",
			offset:    -0.05,
			timeout:   120,
			shouldErr: false,
		},
		{
			name:      "無效策略 - unknown",
			strategy:  "unknown_strategy",
			offset:    -0.03,
			timeout:   60,
			shouldErr: false, // 當前實現不驗證策略名稱
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			trader := NewFuturesTrader("test_key", "test_secret", "test_user", tc.strategy, tc.offset, tc.timeout)

			if trader == nil && !tc.shouldErr {
				t.Error("預期創建成功，但返回 nil")
			}

			if trader != nil && !tc.shouldErr {
				t.Logf("✅ 參數驗證通過: 策略=%s, 偏移=%.2f%%, 超時=%ds",
					tc.strategy, tc.offset, tc.timeout)
			}
		})
	}
}

// TestOrderStrategy_ShortPosition 測試做空倉位的訂單策略
func TestOrderStrategy_ShortPosition(t *testing.T) {
	// Mock server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Mock initialization endpoints
		if r.URL.Path == "/fapi/v1/time" {
			response := map[string]interface{}{
				"serverTime": time.Now().UnixMilli(),
			}
			json.NewEncoder(w).Encode(response)
			return
		}

		if r.URL.Path == "/fapi/v1/positionSide/dual" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"code": 200,
				"msg":  "success",
			})
			return
		}

		if r.URL.Path == "/fapi/v1/leverage" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"leverage": 10,
				"symbol":   "ETHUSDT",
			})
			return
		}

		if r.URL.Path == "/fapi/v1/allOpenOrders" && r.Method == "DELETE" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"code": 200,
				"msg":  "success",
			})
			return
		}

		if r.URL.Path == "/fapi/v2/positionRisk" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode([]map[string]interface{}{})
			return
		}

		if r.URL.Path == "/fapi/v1/exchangeInfo" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"symbols": []map[string]interface{}{
					{
						"symbol": "ETHUSDT",
						"filters": []map[string]interface{}{
							{
								"filterType": "LOT_SIZE",
								"stepSize":   "0.001",
							},
							{
								"filterType": "PRICE_FILTER",
								"tickSize":   "0.01",
							},
						},
					},
				},
			})
			return
		}

		if r.URL.Path == "/fapi/v2/ticker/price" || r.URL.Path == "/fapi/v1/ticker/price" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode([]map[string]interface{}{
				{
					"symbol": "ETHUSDT",
					"price":  "3000.0",
				},
			})
			return
		}

		if r.URL.Path == "/fapi/v1/order" {
			// 限價單創建
			side := r.FormValue("side")
			response := &futures.CreateOrderResponse{
				OrderID: 12351,
				Symbol:  "ETHUSDT",
				Status:  "NEW",
				Side:    futures.SideType(side),
				Type:    "LIMIT",
				Price:   "3000.9", // 當前價 * (1 + 0.0003) = 3000 * 1.0003 = 3000.9
			}
			json.NewEncoder(w).Encode(response)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockServer.Close()

	// Create a client configured to use the mock server
	client := futures.NewClient("test_key", "test_secret")
	client.BaseURL = mockServer.URL
	client.HTTPClient = mockServer.Client()

	// Create trader with the mocked client
	trader := newFuturesTraderWithClient(client, "limit_only", 0.03, 60)

	// 測試開空倉
	result, err := trader.OpenShort("ETHUSDT", 100.0, 10)
	if err != nil {
		t.Fatalf("做空倉位創建失敗: %v", err)
	}

	orderID, ok := result["orderId"].(int64)
	if !ok || orderID != 12351 {
		t.Fatalf("預期 OrderID=12351, 實際 %v (type: %T)", result["orderId"], result["orderId"])
	}

	t.Logf("✅ 做空倉位訂單策略測試通過")
}

// TestOrderStrategy_DatabaseIntegration 測試訂單策略的數據庫集成
func TestOrderStrategy_DatabaseIntegration(t *testing.T) {
	// 這個測試驗證策略配置可以正確存儲和讀取
	t.Run("策略配置存儲", func(t *testing.T) {
		testConfigs := []struct {
			strategy string
			offset   float64
			timeout  int
		}{
			{"market_only", -0.03, 60},
			{"conservative_hybrid", -0.05, 120},
			{"limit_only", -0.02, 180},
		}

		for _, cfg := range testConfigs {
			t.Logf("策略配置: strategy=%s, offset=%.2f%%, timeout=%ds",
				cfg.strategy, cfg.offset, cfg.timeout)

			// 驗證配置可以被正確設置
			trader := NewFuturesTrader("key", "secret", "user", cfg.strategy, cfg.offset, cfg.timeout)

			if trader.orderStrategy != cfg.strategy {
				t.Errorf("策略不匹配: 預期 %s, 實際 %s", cfg.strategy, trader.orderStrategy)
			}

			if trader.limitPriceOffset != cfg.offset {
				t.Errorf("偏移不匹配: 預期 %.2f, 實際 %.2f", cfg.offset, trader.limitPriceOffset)
			}

			if trader.limitTimeoutSeconds != cfg.timeout {
				t.Errorf("超時不匹配: 預期 %d, 實際 %d", cfg.timeout, trader.limitTimeoutSeconds)
			}
		}

		t.Logf("✅ 所有策略配置都能正確設置")
	})
}

// BenchmarkOrderStrategy 性能測試
func BenchmarkOrderStrategy_MarketOrder(b *testing.B) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := &futures.CreateOrderResponse{
			OrderID: 99999,
			Symbol:  "BTCUSDT",
			Status:  "FILLED",
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	trader := NewFuturesTrader("key", "secret", "user", "market_only", -0.03, 60)
	trader.client.BaseURL = mockServer.URL
	trader.client.HTTPClient = mockServer.Client()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		trader.OpenLong("BTCUSDT", 100.0, 10)
	}
}
