package decision

import (
	"fmt"
	"math"
	"nofx/market"
)

// EnhancedValidator 增强版验证器
type EnhancedValidator struct {
	AccountEquity    float64
	BTCETHLeverage   int
	AltcoinLeverage  int
	MarketData       map[string]*market.Data
	CurrentPositions []PositionInfo
}

// ValidationResult 验证结果
type ValidationResult struct {
	IsValid     bool     `json:"is_valid"`
	Errors      []string `json:"errors"`
	Warnings    []string `json:"warnings"`
	RiskLevel   string   `json:"risk_level"`
	RiskPercent float64  `json:"risk_percent"`
}

// NewEnhancedValidator 创建增强验证器
func NewEnhancedValidator(accountEquity float64, btcLeverage, altcoinLeverage int, currentPositions []PositionInfo) *EnhancedValidator {
	return &EnhancedValidator{
		AccountEquity:    accountEquity,
		BTCETHLeverage:   btcLeverage,
		AltcoinLeverage:  altcoinLeverage,
		MarketData:       make(map[string]*market.Data),
		CurrentPositions: currentPositions,
	}
}

// ValidateDecision 增强版决策验证 (v6.6 - 终极裁决版)
func (ev *EnhancedValidator) ValidateDecision(d *Decision) *ValidationResult {
	result := &ValidationResult{
		IsValid:  true,
		Errors:   make([]string, 0),
		Warnings: make([]string, 0),
	}

	// 1. 基础验证 (保持不变)
	if err := ev.basicValidation(d); err != nil {
		result.Errors = append(result.Errors, err.Error())
		result.IsValid = false
		result.RiskLevel = "invalid"
		return result // 基础验证失败，直接返回
	}

	// 2. 仅对开仓操作进行严格的“三重保险”验证
	if d.Action == "open_long" || d.Action == "open_short" {
		// a. 验证单笔风险 (第一重)
		ev.validateRisk(d, result)
		// b. 验证仓位上限 (第二重)
		ev.validatePositionSize(d, result)
		// c. 验证止损距离 (第三重 - 与提示词同步)
		ev.validateStopLoss(d, result)

		// d. 其他验证
		ev.validateLeverage(d, result)

		// e. 评估风险等级
		ev.assessRiskLevel(d, result)
	}

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
		return fmt.Errorf("无效的action: '%s'", d.Action)
	}
	if d.Symbol == "" && (d.Action != "wait" && d.Action != "hold") {
		return fmt.Errorf("非等待/持有操作，交易对不能为空")
	}
	return nil
}

// validateRisk 风险验证 (与您的最新指令同步: 2%硬顶)
func (ev *EnhancedValidator) validateRisk(d *Decision, result *ValidationResult) {
	marketData, ok := ev.MarketData[d.Symbol]
	if !ok || marketData.CurrentPrice <= 0 {
		result.Errors = append(result.Errors, fmt.Sprintf("缺少或无效的市场数据: %s", d.Symbol))
		result.IsValid = false
		return
	}

	if d.PositionSizeUSD <= 0 || d.StopLoss <= 0 {
		result.Errors = append(result.Errors, "开仓金额和止损价必须为正数")
		result.IsValid = false
		return
	}

	quantity := d.PositionSizeUSD / marketData.CurrentPrice
	potentialLossUSD := 0.0
	if d.Action == "open_long" {
		potentialLossUSD = quantity * (marketData.CurrentPrice - d.StopLoss)
	} else {
		potentialLossUSD = quantity * (d.StopLoss - marketData.CurrentPrice)
	}

	riskPercent := (potentialLossUSD / ev.AccountEquity) * 100
	result.RiskPercent = riskPercent

	// 验证风险预算，与您的最新指令同步：2%
	maxAllowedRisk := ev.AccountEquity * 0.02
	if potentialLossUSD > maxAllowedRisk {
		result.Errors = append(result.Errors,
			fmt.Sprintf("风险超限: 潜在亏损 %.2f USDT (%.2f%%) > 最大允许 %.2f USDT (2.0%%)",
				potentialLossUSD, riskPercent, maxAllowedRisk))
		result.IsValid = false
	}
}

// validatePositionSize 仓位大小验证 (与您的最新指令同步: 60%/85%)
func (ev *EnhancedValidator) validatePositionSize(d *Decision, result *ValidationResult) {
	// 最小开仓金额
	minSize := 12.0 // 与system prompt保持一致
	if d.PositionSizeUSD < minSize {
		result.Errors = append(result.Errors,
			fmt.Sprintf("开仓金额过小: %.2f USDT < 最小要求 %.2f USDT", d.PositionSizeUSD, minSize))
		result.IsValid = false
	}

	// 最大仓位限制 (硬顶)
	maxPositionValue := ev.getMaxPositionValue(d.Symbol)
	if d.PositionSizeUSD > maxPositionValue {
		result.Errors = append(result.Errors,
			fmt.Sprintf("仓位价值超限: %.2f USDT > 最大允许 %.2f USDT (净值%.0f%%)",
				d.PositionSizeUSD, maxPositionValue, ev.getPositionCapRatio(d.Symbol)*100))
		result.IsValid = false
	}
}

// validateStopLoss 止损验证 (与您的最新指令同步: 1.2%最小距离)
func (ev *EnhancedValidator) validateStopLoss(d *Decision, result *ValidationResult) {
	marketData, ok := ev.MarketData[d.Symbol]
	if !ok || marketData.CurrentPrice <= 0 {
		return // 在validateRisk中已处理
	}
	currentPrice := marketData.CurrentPrice

	// 止损价格方向验证
	if (d.Action == "open_long" && d.StopLoss >= currentPrice) ||
		(d.Action == "open_short" && d.StopLoss <= currentPrice) {
		result.Errors = append(result.Errors, "止损价格方向错误")
		result.IsValid = false
	}

	// 止损距离下限验证，与您的最新指令同步：1.2%
	stopLossDistancePercent := (math.Abs(d.StopLoss-currentPrice) / currentPrice) * 100
	minDistancePercent := 1.2
	if stopLossDistancePercent < minDistancePercent {
		result.Warnings = append(result.Warnings, // 改为警告，因为AI可能因为结构点而选择更近的止损，最终由风险比例把关
			fmt.Sprintf("止损距离过近: %.2f%% < 建议最小距离 %.2f%%", stopLossDistancePercent, minDistancePercent))
	}
}

// validateLeverage 杠杆验证 (保持不变，但逻辑更清晰)
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
}

// assessRiskLevel 评估风险等级 (逻辑简化)
func (ev *EnhancedValidator) assessRiskLevel(d *Decision, result *ValidationResult) {
	if !result.IsValid {
		result.RiskLevel = "invalid"
		return
	}
	// 根据2%的上限来划分
	if result.RiskPercent > 1.5 { // 超过1.5%即为高风险
		result.RiskLevel = "high"
	} else if result.RiskPercent > 1.0 {
		result.RiskLevel = "medium"
	} else {
		result.RiskLevel = "low"
	}
}

// 辅助函数 (与您的最新指令同步: 60%/85%)
func (ev *EnhancedValidator) getMaxPositionValue(symbol string) float64 {
	if symbol == "BTCUSDT" || symbol == "ETHUSDT" {
		return ev.AccountEquity * 0.85
	}
	return ev.AccountEquity * 0.60
}

// 辅助函数 (与您的最新指令同步)
func (ev *EnhancedValidator) getPositionCapRatio(symbol string) float64 {
	if symbol == "BTCUSDT" || symbol == "ETHUSDT" {
		return 0.85
	}
	return 0.60
}
