package decision

import (
	"testing"
)

// TestLeverageFallback 测试杠杆超限时的自动修正功能
func TestLeverageFallback(t *testing.T) {
	tests := []struct {
		name            string
		decision        Decision
		accountEquity   float64
		btcEthLeverage  int
		altcoinLeverage int
		wantLeverage    int // 期望修正后的杠杆值
		wantError       bool
	}{
		{
			name: "山寨币杠杆超限_自动修正为上限",
			decision: Decision{
				Symbol:          "SOLUSDT",
				Action:          "open_long",
				Leverage:        20, // 超过上限
				PositionSizeUSD: 100,
				StopLoss:        50,
				TakeProfit:      200,
			},
			accountEquity:   100,
			btcEthLeverage:  10,
			altcoinLeverage: 5, // 上限 5x
			wantLeverage:    5, // 应该修正为 5
			wantError:       false,
		},
		{
			name: "BTC杠杆超限_自动修正为上限",
			decision: Decision{
				Symbol:          "BTCUSDT",
				Action:          "open_long",
				Leverage:        20, // 超过上限
				PositionSizeUSD: 1000,
				StopLoss:        90000,
				TakeProfit:      110000,
			},
			accountEquity:   100,
			btcEthLeverage:  10, // 上限 10x
			altcoinLeverage: 5,
			wantLeverage:    10, // 应该修正为 10
			wantError:       false,
		},
		{
			name: "杠杆在上限内_不修正",
			decision: Decision{
				Symbol:          "ETHUSDT",
				Action:          "open_short",
				Leverage:        5, // 未超限
				PositionSizeUSD: 500,
				StopLoss:        4000,
				TakeProfit:      3000,
			},
			accountEquity:   100,
			btcEthLeverage:  10,
			altcoinLeverage: 5,
			wantLeverage:    5, // 保持不变
			wantError:       false,
		},
		{
			name: "杠杆为0_应该报错",
			decision: Decision{
				Symbol:          "SOLUSDT",
				Action:          "open_long",
				Leverage:        0, // 无效
				PositionSizeUSD: 100,
				StopLoss:        50,
				TakeProfit:      200,
			},
			accountEquity:   100,
			btcEthLeverage:  10,
			altcoinLeverage: 5,
			wantLeverage:    0,
			wantError:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDecision(&tt.decision, tt.accountEquity, tt.btcEthLeverage, tt.altcoinLeverage)

			// 检查错误状态
			if (err != nil) != tt.wantError {
				t.Errorf("validateDecision() error = %v, wantError %v", err, tt.wantError)
				return
			}

			// 如果不应该报错，检查杠杆是否被正确修正
			if !tt.wantError && tt.decision.Leverage != tt.wantLeverage {
				t.Errorf("Leverage not corrected: got %d, want %d", tt.decision.Leverage, tt.wantLeverage)
			}
		})
	}
}

// TestCheckThousandsSeparatorsOutsideStrings tests the thousands separator validation
func TestCheckThousandsSeparatorsOutsideStrings(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		wantErr bool
	}{
		{
			name:    "Valid: comma inside string value",
			json:    `[{"symbol": "BTCUSDT", "action": "wait", "reasoning": "价格不在精确入场范围(做多需≤102,707),期望值不足"}]`,
			wantErr: false,
		},
		{
			name:    "Valid: multiple commas in string",
			json:    `[{"symbol": "BTCUSDT", "reasoning": "价格从98,000上升到102,707"}]`,
			wantErr: false,
		},
		{
			name:    "Valid: normal JSON without thousands separators",
			json:    `[{"symbol": "BTCUSDT", "price": 102707, "action": "long"}]`,
			wantErr: false,
		},
		{
			name:    "Invalid: thousands separator in number value",
			json:    `[{"symbol": "BTCUSDT", "price": 102,707}]`,
			wantErr: true,
		},
		{
			name:    "Invalid: thousands separator in array",
			json:    `[{"symbol": "BTCUSDT", "prices": [98,000, 102,707]}]`,
			wantErr: true,
		},
		{
			name:    "Valid: escaped quotes in string",
			json:    `[{"reasoning": "价格\"102,707\"较高"}]`,
			wantErr: false,
		},
		{
			name:    "Valid: comma in multiple string fields",
			json:    `[{"symbol": "BTCUSDT", "reasoning1": "价格102,707", "reasoning2": "目标98,000"}]`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkThousandsSeparatorsOutsideStrings(tt.json)
			if (err != nil) != tt.wantErr {
				t.Errorf("checkThousandsSeparatorsOutsideStrings() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				t.Logf("✅ Correctly accepted: %s", tt.json[:min(60, len(tt.json))])
			} else if err != nil {
				t.Logf("✅ Correctly rejected: %v", err)
			}
		})
	}
}

// TestValidateJSONFormat tests the complete JSON validation
func TestValidateJSONFormat(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		wantErr bool
	}{
		{
			name:    "Valid: standard decision array",
			json:    `[{"symbol": "BTCUSDT", "action": "long", "reasoning": "good entry"}]`,
			wantErr: false,
		},
		{
			name:    "Valid: comma in reasoning string",
			json:    `[{"symbol": "BTCUSDT", "action": "wait", "reasoning": "价格不在精确入场范围(做多需≤102,707),期望值不足"}]`,
			wantErr: false,
		},
		{
			name:    "Invalid: does not start with [{",
			json:    `{"symbol": "BTCUSDT"}`,
			wantErr: true,
		},
		{
			name:    "Invalid: range symbol ~",
			json:    `[{"symbol": "BTCUSDT", "price": "98000~102000"}]`,
			wantErr: true,
		},
		{
			name:    "Invalid: pure number array",
			json:    `[1, 2, 3]`,
			wantErr: true,
		},
		{
			name:    "Valid: whitespace before [{",
			json:    `  [{"symbol": "BTCUSDT", "action": "long"}]`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateJSONFormat(tt.json)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateJSONFormat() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				t.Logf("✅ Validation passed")
			} else if err != nil {
				t.Logf("✅ Correctly caught error: %v", err)
			}
		})
	}
}

// TestRealWorldAIResponse tests with actual AI response from error log
func TestRealWorldAIResponse(t *testing.T) {
	// This is the actual JSON that caused the error
	realWorldJSON := `[{"symbol": "BTCUSDT", "action": "wait", "reasoning": "价格不在精确入场范围(做多需≤102,707),期望值不足,等待更好时机"}]`

	err := validateJSONFormat(realWorldJSON)
	if err != nil {
		t.Errorf("Real-world AI response should be valid, but got error: %v", err)
	} else {
		t.Logf("✅ Real-world AI response validated successfully")
	}

	err = checkThousandsSeparatorsOutsideStrings(realWorldJSON)
	if err != nil {
		t.Errorf("Real-world AI response should pass thousands separator check, but got error: %v", err)
	} else {
		t.Logf("✅ Real-world AI response passed thousands separator check")
	}
}
