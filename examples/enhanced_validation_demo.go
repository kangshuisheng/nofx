package main

import (
	"fmt"
	"nofx/decision"
	"nofx/market"
)

func main() {
	fmt.Println("🚀 增强验证机制演示")
	fmt.Println("===================")

	// 模拟账户设置
	accountEquity := 10000.0 // 1万美元账户
	btcEthLeverage := 10
	altcoinLeverage := 5

	// 创建增强验证器
	validator := decision.NewEnhancedValidator(accountEquity, btcEthLeverage, altcoinLeverage)

	// 模拟市场数据
	mockMarketData := &market.Data{
		Symbol:       "BTCUSDT",
		CurrentPrice: 100000,
		IntradaySeries: &market.IntradayData{
			ATR14: 2000,
		},
		LongerTermContext: &market.LongerTermData{
			ATR14: 2500,
		},
		DailyContext: &market.DailyData{
			ATR14Values: []float64{2200, 2300, 2400},
		},
	}
	validator.MarketData["BTCUSDT"] = mockMarketData

	// 测试场景1：高风险决策（仓位过大）
	fmt.Println("\n📊 场景1：高风险决策（仓位过大）")
	fmt.Println("----------------------------------")
	highRiskDecision := &decision.Decision{
		Symbol:          "BTCUSDT",
		Action:          "open_long",
		Leverage:        10,
		PositionSizeUSD: 8000, // 80% 账户仓位
		StopLoss:        98000,
		TakeProfit:      102000,
		Confidence:      85,
		Reasoning:       "技术分析显示突破",
	}

	result1 := validator.ValidateDecision(highRiskDecision)
	printValidationResult("高风险决策", result1)

	// 测试场景2：合理决策
	fmt.Println("\n📊 场景2：合理决策")
	fmt.Println("------------------")
	goodDecision := &decision.Decision{
		Symbol:          "BTCUSDT",
		Action:          "open_long",
		Leverage:        3,
		PositionSizeUSD: 1500, // 15% 账户仓位
		StopLoss:        99000,
		TakeProfit:      103000,
		Confidence:      75,
		Reasoning:       "趋势跟踪策略",
	}

	result2 := validator.ValidateDecision(goodDecision)
	printValidationResult("合理决策", result2)

	// 测试场景3：杠杆超限
	fmt.Println("\n📊 场景3：杠杆超限")
	fmt.Println("------------------")
	leverageDecision := &decision.Decision{
		Symbol:          "BTCUSDT",
		Action:          "open_short",
		Leverage:        15, // 超过BTC最大杠杆10x
		PositionSizeUSD: 1000,
		StopLoss:        101000,
		TakeProfit:      99000,
		Confidence:      70,
		Reasoning:       "回调做空",
	}

	result3 := validator.ValidateDecision(leverageDecision)
	printValidationResult("杠杆超限决策", result3)

	// 测试场景4：止损设置不合理
	fmt.Println("\n📊 场景4：止损设置不合理")
	fmt.Println("--------------------------")
	badStopLossDecision := &decision.Decision{
		Symbol:          "BTCUSDT",
		Action:          "open_long",
		Leverage:        5,
		PositionSizeUSD: 2000,
		StopLoss:        99900, // 距离当前价格太近（仅0.1%）
		TakeProfit:      105000,
		Confidence:      80,
		Reasoning:       "日内交易",
	}

	result4 := validator.ValidateDecision(badStopLossDecision)
	printValidationResult("止损不合理决策", result4)

	fmt.Println("\n✅ 演示完成！")
	fmt.Println("增强验证机制已成功集成到系统中。")
}

func printValidationResult(scenario string, result *decision.ValidationResult) {
	fmt.Printf("\n%s 验证结果：\n", scenario)
	fmt.Printf("有效性: %t\n", result.IsValid)
	fmt.Printf("风险等级: %s\n", result.RiskLevel)
	fmt.Printf("风险比例: %.2f%%\n", result.RiskPercent)

	if len(result.Errors) > 0 {
		fmt.Printf("错误: %v\n", result.Errors)
	}

	if len(result.Warnings) > 0 {
		fmt.Printf("警告: %v\n", result.Warnings)
	}

	if len(result.Suggestions) > 0 {
		fmt.Printf("建议: %v\n", result.Suggestions)
	}
}
