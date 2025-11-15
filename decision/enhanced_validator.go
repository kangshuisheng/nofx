package decision

import (
	"fmt"
	"math"
	"nofx/market"
)

// EnhancedValidator 增强版验证器
type EnhancedValidator struct {
	AccountEquity   float64
	BTCETHLeverage  int
	AltcoinLeverage int
	MarketData      map[string]*market.Data
}

// ValidationResult 验证结果
type ValidationResult struct {
	IsValid     bool     `json:"is_valid"`
	Errors      []string `json:"errors"`
	Warnings    []string `json:"warnings"`
	Suggestions []string `json:"suggestions"`
	RiskLevel   string   `json:"risk_level"`
	RiskPercent float64  `json:"risk_percent"`
}

// NewEnhancedValidator 创建增强验证器
func NewEnhancedValidator(accountEquity float64, btcLeverage, altcoinLeverage int) *EnhancedValidator {
	return &EnhancedValidator{
		AccountEquity:   accountEquity,
		BTCETHLeverage:  btcLeverage,
		AltcoinLeverage: altcoinLeverage,
		MarketData:      make(map[string]*market.Data),
	}
}

// ValidateDecision 增强版决策验证
func (ev *EnhancedValidator) ValidateDecision(d *Decision) *ValidationResult {
	result := &ValidationResult{
		IsValid:     true,
		Errors:      make([]string, 0),
		Warnings:    make([]string, 0),
		Suggestions: make([]string, 0),
		RiskLevel:   "low",
	}

	// 1. 基础验证
	if err := ev.basicValidation(d); err != nil {
		result.Errors = append(result.Errors, err.Error())
		result.IsValid = false
	}

	// 2. 风险计算和验证
	if d.Action == "open_long" || d.Action == "open_short" {
		ev.validateRisk(d, result)
		ev.validatePositionSize(d, result)
		ev.validateStopLoss(d, result)
		ev.validateTakeProfit(d, result)
		ev.validateLeverage(d, result)
	}

	// 3. 智能建议
	ev.generateSuggestions(d, result)

	// 4. 风险等级评估
	ev.assessRiskLevel(result)

	return result
}

// basicValidation 基础验证
func (ev *EnhancedValidator) basicValidation(d *Decision) error {
	validActions := map[string]bool{
		"open_long": true, "open_short": true, "close_long": true,
		"close_short": true, "update_stop_loss": true, "update_take_profit": true,
		"partial_close": true, "hold": true, "wait": true,
	}

	if !validActions[d.Action] {
		return fmt.Errorf("无效的action: %s", d.Action)
	}

	if d.Symbol == "" {
		return fmt.Errorf("交易对不能为空")
	}

	return nil
}

// validateRisk 风险验证
func (ev *EnhancedValidator) validateRisk(d *Decision, result *ValidationResult) {
	marketData, exists := ev.MarketData[d.Symbol]
	if !exists {
		result.Errors = append(result.Errors, fmt.Sprintf("缺少 %s 的市场数据", d.Symbol))
		result.IsValid = false
		return
	}

	entryPrice := marketData.CurrentPrice
	if entryPrice <= 0 {
		result.Errors = append(result.Errors, "无效的市场价格")
		result.IsValid = false
		return
	}

	// 计算潜在亏损
	quantity := d.PositionSizeUSD / entryPrice
	var potentialLossUSD float64

	if d.Action == "open_long" {
		potentialLossUSD = quantity * (entryPrice - d.StopLoss)
	} else {
		potentialLossUSD = quantity * (d.StopLoss - entryPrice)
	}

	// 风险百分比
	riskPercent := (potentialLossUSD / ev.AccountEquity) * 100
	result.RiskPercent = riskPercent

	// 验证风险预算， 单笔交易最大亏损控制在2%
	maxAllowedRisk := ev.AccountEquity * 0.02 // 2%
	if potentialLossUSD > maxAllowedRisk {
		result.Errors = append(result.Errors,
			fmt.Sprintf("风险超限: %.2f USDT (%.2f%%) > 最大允许 %.2f USDT (2%%)",
				potentialLossUSD, riskPercent, maxAllowedRisk))
		result.IsValid = false
	}

	// 风险警告级别
	if riskPercent > 1.2 {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("高风险警告: %.2f%% 接近最大限制", riskPercent))
	} else if riskPercent < 0.5 {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("低风险提示: 仅%.2f%%，可能过于保守", riskPercent))
	}
}

// validatePositionSize 仓位大小验证
func (ev *EnhancedValidator) validatePositionSize(d *Decision, result *ValidationResult) {
	// 最小开仓金额
	minSize := calculateMinPositionSize(d.Symbol, ev.AccountEquity)
	if d.PositionSizeUSD < minSize {
		result.Errors = append(result.Errors,
			fmt.Sprintf("开仓金额过小: %.2f USDT < 最小要求 %.2f USDT",
				d.PositionSizeUSD, minSize))
		result.IsValid = false
	}

	// 最大仓位限制
	maxPositionValue := ev.getMaxPositionValue(d.Symbol)
	if d.PositionSizeUSD > maxPositionValue {
		result.Errors = append(result.Errors,
			fmt.Sprintf("仓位价值超限: %.2f USDT > 最大允许 %.2f USDT",
				d.PositionSizeUSD, maxPositionValue))
		result.IsValid = false
	}

	// 仓位合理性检查
	positionRatio := d.PositionSizeUSD / ev.AccountEquity
	if positionRatio > 1.0 {
		result.Warnings = append(result.Warnings, "仓位超过账户净值，风险极高")
	} else if positionRatio < 0.01 {
		result.Warnings = append(result.Warnings, "仓位过小，可能影响收益")
	}
}

// validateStopLoss 止损验证
func (ev *EnhancedValidator) validateStopLoss(d *Decision, result *ValidationResult) {
	marketData, exists := ev.MarketData[d.Symbol]
	if !exists {
		return
	}

	currentPrice := marketData.CurrentPrice
	stopLossDistance := math.Abs(d.StopLoss-currentPrice) / currentPrice

	// 止损距离合理性
	minDistance := 0.005 // 最小0.5%
	maxDistance := 0.08  // 最大8%

	if stopLossDistance < minDistance {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("止损距离过小: %.2f%%，可能被市场噪音触发", stopLossDistance*100))
	}

	if stopLossDistance > maxDistance {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("止损距离过大: %.2f%%，风险过高", stopLossDistance*100))
	}

	// 止损价格合理性
	if d.Action == "open_long" && d.StopLoss >= currentPrice {
		result.Errors = append(result.Errors, "多头止损价格必须低于当前价格")
		result.IsValid = false
	}

	if d.Action == "open_short" && d.StopLoss <= currentPrice {
		result.Errors = append(result.Errors, "空头止损价格必须高于当前价格")
		result.IsValid = false
	}
}

// validateTakeProfit 止盈验证
func (ev *EnhancedValidator) validateTakeProfit(d *Decision, result *ValidationResult) {
	marketData, exists := ev.MarketData[d.Symbol]
	if !exists {
		return
	}

	currentPrice := marketData.CurrentPrice

	// 风险回报比
	var riskUSD, rewardUSD float64
	if d.Action == "open_long" {
		riskUSD = currentPrice - d.StopLoss
		rewardUSD = d.TakeProfit - currentPrice
	} else {
		riskUSD = d.StopLoss - currentPrice
		rewardUSD = currentPrice - d.TakeProfit
	}

	if riskUSD > 0 {
		rewardRatio := rewardUSD / riskUSD
		if rewardRatio < 1.5 {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("风险回报比偏低: 1:%.1f，建议至少1:2", rewardRatio))
		}
	}

	// 止盈价格合理性
	if d.Action == "open_long" && d.TakeProfit <= currentPrice {
		result.Errors = append(result.Errors, "多头止盈价格必须高于当前价格")
		result.IsValid = false
	}

	if d.Action == "open_short" && d.TakeProfit >= currentPrice {
		result.Errors = append(result.Errors, "空头止盈价格必须低于当前价格")
		result.IsValid = false
	}
}

// validateLeverage 杠杆验证
func (ev *EnhancedValidator) validateLeverage(d *Decision, result *ValidationResult) {
	maxLeverage := ev.AltcoinLeverage
	if d.Symbol == "BTCUSDT" || d.Symbol == "ETHUSDT" {
		maxLeverage = ev.BTCETHLeverage
	}

	if d.Leverage > maxLeverage {
		result.Errors = append(result.Errors,
			fmt.Sprintf("杠杆超限: %dx > 最大允许 %dx", d.Leverage, maxLeverage))
		result.IsValid = false
	}

	if d.Leverage < 1 {
		result.Errors = append(result.Errors, "杠杆不能小于1倍")
		result.IsValid = false
	}

	// 杠杆建议
	if d.Leverage > int(float64(maxLeverage)*0.8) {
		result.Warnings = append(result.Warnings, "使用高杠杆，请谨慎操作")
	}
}

// generateSuggestions 生成智能建议
func (ev *EnhancedValidator) generateSuggestions(d *Decision, result *ValidationResult) {
	if len(result.Errors) > 0 {
		return // 有错误时不生成建议
	}

	// 仓位建议
	if result.RiskPercent < 0.8 {
		result.Suggestions = append(result.Suggestions,
			"可考虑适当增加仓位以提高资金利用率")
	}

	// 止损建议
	marketData, exists := ev.MarketData[d.Symbol]
	if exists {
		atr := ev.getATR(d.Symbol)
		if atr > 0 {
			currentPrice := marketData.CurrentPrice
			suggestedStopLoss := currentPrice - atr*0.5 // 建议0.5倍ATR距离
			if d.Action == "open_short" {
				suggestedStopLoss = currentPrice + atr*0.5
			}

			currentDistance := math.Abs(d.StopLoss - currentPrice)
			suggestedDistance := math.Abs(suggestedStopLoss - currentPrice)

			if math.Abs(currentDistance-suggestedDistance)/currentDistance > 0.1 {
				result.Suggestions = append(result.Suggestions,
					fmt.Sprintf("建议调整止损至 %.0f (基于0.5倍ATR)", suggestedStopLoss))
			}
		}
	}

	// 信心度建议
	if d.Confidence < 70 {
		result.Suggestions = append(result.Suggestions,
			"信心度较低，建议等待更明确信号")
	}
}

// assessRiskLevel 评估风险等级
func (ev *EnhancedValidator) assessRiskLevel(result *ValidationResult) {
	if !result.IsValid {
		result.RiskLevel = "invalid"
		return
	}

	if result.RiskPercent > 1.2 {
		result.RiskLevel = "high"
	} else if result.RiskPercent > 0.8 {
		result.RiskLevel = "medium"
	} else {
		result.RiskLevel = "low"
	}
}

// 辅助函数
func (ev *EnhancedValidator) getMaxPositionValue(symbol string) float64 {
	if symbol == "BTCUSDT" || symbol == "ETHUSDT" {
		return ev.AccountEquity * 1.5
	}
	return ev.AccountEquity * 0.75
}

func (ev *EnhancedValidator) getATR(symbol string) float64 {
	if data, exists := ev.MarketData[symbol]; exists {
		// 从不同的数据层级获取ATR值，按优先级顺序
		if data.LongerTermContext != nil && data.LongerTermContext.ATR14 > 0 {
			return data.LongerTermContext.ATR14 // 优先使用4小时ATR14
		}
		if data.IntradaySeries != nil && data.IntradaySeries.ATR14 > 0 {
			return data.IntradaySeries.ATR14 // 使用3分钟ATR14
		}
		if data.DailyContext != nil && len(data.DailyContext.ATR14Values) > 0 {
			// 使用日线ATR14的最新值
			return data.DailyContext.ATR14Values[len(data.DailyContext.ATR14Values)-1]
		}
	}
	return 0
}

func (ev *EnhancedValidator) suggestStopLoss(currentPrice float64, action string, atr float64) float64 {
	atrMultiplier := 2.0
	if action == "open_long" {
		return currentPrice - (atr * atrMultiplier)
	}
	return currentPrice + (atr * atrMultiplier)
}
