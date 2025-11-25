package decision

// RiskConfig 统一风控参数配置
// 集中管理所有止损、风险相关的百分比参数，避免硬编码分散
type RiskConfig struct {
	// 单笔风险限制
	MaxSingleTradeRiskPct float64 // 单笔交易最大风险百分比 (默认 2%)

	// 初始止损设置
	DefaultStopLossATRMultiplier float64 // 默认止损 ATR 倍数 (默认 2.5)
	DefaultStopLossPct           float64 // 降级止损百分比 (无 ATR 时使用, 默认 2.5%)
	MaxStopLossPct               float64 // 硬顶保护: 止损距离不得超过 (默认 2.5%)

	// 持仓管理阈值
	BreakevenRRRatio float64 // 触发保本的 R:R 比例 (默认 1.0)
	TrailingRRRatio  float64 // 触发移动止损的 R:R 比例 (默认 2.0)

	// 账户级风控
	MaxDailyLossPct float64 // 最大日亏损百分比 (默认 5%)
	MaxDrawdownPct  float64 // 最大回撤百分比 (默认 10%)
}

// DefaultRiskConfig 返回默认风控配置
// 统一所有风控参数，确保系统各模块使用一致的风险控制标准
func DefaultRiskConfig() *RiskConfig {
	return &RiskConfig{
		// 核心风控: 单笔最大风险 2%
		MaxSingleTradeRiskPct: 0.02, // 2%

		// 止损设置: 2.5% 为基准
		DefaultStopLossATRMultiplier: 2.5,   // ATR 倍数
		DefaultStopLossPct:           0.025, // 2.5% (无 ATR 时降级)
		MaxStopLossPct:               0.025, // 2.5% (硬顶保护)

		// 持仓管理: R:R 阈值
		BreakevenRRRatio: 1.0, // R:R >= 1.0 触发保本
		TrailingRRRatio:  2.0, // R:R >= 2.0 触发移动止损

		// 账户级风控
		MaxDailyLossPct: 5.0,  // 5% 日亏损上限
		MaxDrawdownPct:  10.0, // 10% 回撤上限
	}
}
