package decision

import (
	"fmt"
	"math"
	"nofx/market"
)

// ManagementAction 管理动作
type ManagementAction struct {
	Action   string  // "update_stop_loss", "none"
	NewPrice float64 // 新的止损价格
	Reason   string  // 原因
}

// CheckManagementAction 检查持仓管理动作 (Go自动执行)
// 替代原有的 calculateManagementState，直接返回具体操作
func CheckManagementAction(pos PositionInfo, currentSL float64, marketData *market.Data) ManagementAction {
	if currentSL == 0 {
		// 没有止损，必须立即设置
		// 默认 ATR*1.5 紧凑止损（优化风控：减少单笔亏损）
		atr := 0.0
		if marketData != nil && marketData.LongerTermContext != nil {
			atr = marketData.LongerTermContext.ATR14
		}
		if atr == 0 {
			atr = pos.MarkPrice * 0.015 // 降级：1.5% (原 3%)
		}

		newSL := 0.0
		// 计算 ATR*1.5 止损
		slDist := 1.5 * atr

		// 🛡️ 硬顶保护：止损距离不得超过入场价的 2%
		// 防止高波动率下 ATR 过大导致亏损过高
		maxDist := pos.EntryPrice * 0.02
		if slDist > maxDist {
			slDist = maxDist
		}

		if pos.Side == "long" {
			newSL = pos.EntryPrice - slDist
		} else {
			newSL = pos.EntryPrice + slDist
		}
		return ManagementAction{
			Action:   "update_stop_loss",
			NewPrice: newSL,
			Reason:   fmt.Sprintf("紧急: 缺失止损保护 (默认 ATR*1.5, Max 2%%)"),
		}
	}

	if marketData == nil || marketData.LongerTermContext == nil || marketData.LongerTermContext.ATR14 == 0 {
		return ManagementAction{Action: "none"}
	}

	atr := marketData.LongerTermContext.ATR14

	// 1. 计算初始风险
	initialRisk := math.Abs(pos.EntryPrice - currentSL)
	if initialRisk == 0 {
		initialRisk = atr
	}

	// 2. 计算当前盈利
	currentProfitDist := 0.0
	if pos.Side == "long" {
		currentProfitDist = pos.MarkPrice - pos.EntryPrice
	} else {
		currentProfitDist = pos.EntryPrice - pos.MarkPrice
	}

	// 3. 计算 R:R
	rRatio := currentProfitDist / initialRisk

	// 4. 阶段 2: 风险移除 (Breakeven)
	// 条件: R:R > 1.0 且尚未保本
	if rRatio >= 1.0 {
		isBreakeven := (pos.Side == "long" && currentSL >= pos.EntryPrice) ||
			(pos.Side == "short" && currentSL <= pos.EntryPrice)

		if !isBreakeven {
			// 移动到入场价附近 (加一点点滑点保护)
			buffer := pos.EntryPrice * 0.001 // 0.1% 保护
			newSL := pos.EntryPrice
			if pos.Side == "long" {
				newSL += buffer
			} else {
				newSL -= buffer
			}
			return ManagementAction{
				Action:   "update_stop_loss",
				NewPrice: newSL,
				Reason:   fmt.Sprintf("风险移除 (R:R=%.2f > 1.0) -> 移动至保本位", rRatio),
			}
		}
	}

	// 5. 阶段 3: 利润锁定 (Trailing)
	// 条件: R:R > 2.0
	if rRatio >= 2.0 {
		// 简单的移动止损逻辑: 锁定 50% 的利润
		// 或者移动到 Entry + 1R 的位置
		targetLockPrice := 0.0
		if pos.Side == "long" {
			targetLockPrice = pos.EntryPrice + 1.0*initialRisk
			// 如果当前止损还没跟上
			if currentSL < targetLockPrice {
				return ManagementAction{
					Action:   "update_stop_loss",
					NewPrice: targetLockPrice,
					Reason:   fmt.Sprintf("利润锁定 (R:R=%.2f > 2.0) -> 锁定 1R 利润", rRatio),
				}
			}
		} else {
			targetLockPrice = pos.EntryPrice - 1.0*initialRisk
			// 如果当前止损还没跟上
			if currentSL > targetLockPrice {
				return ManagementAction{
					Action:   "update_stop_loss",
					NewPrice: targetLockPrice,
					Reason:   fmt.Sprintf("利润锁定 (R:R=%.2f > 2.0) -> 锁定 1R 利润", rRatio),
				}
			}
		}
	}

	return ManagementAction{Action: "none"}
}
