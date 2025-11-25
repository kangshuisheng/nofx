package trader

import (
	"fmt"
	"math"
	"nofx/config"
	"nofx/decision"
	"nofx/market"
	"strings"
)

// ComputePositionSize 计算最终名义价值(notional)与下单数量(quantity)
// 强制在 Go 端执行仓位大小计算、风控与上限裁剪，避免直接信任 AI 的 position_size_usd
// 返回值: (notional, quantity, appliedRiskUSD, error)
func ComputePositionSize(at *AutoTrader, d *decision.Decision, mkt *market.Data) (float64, float64, float64, error) {
	if d == nil {
		return 0, 0, 0, fmt.Errorf("decision is nil")
	}
	if mkt == nil {
		return 0, 0, 0, fmt.Errorf("market data is nil for %s", d.Symbol)
	}

	cfg := config.DefaultRiskConfig()

	// 获取账户余额信息
	balance, err := at.trader.GetBalance()
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to get balance: %w", err)
	}
	available := 0.0
	if v, ok := balance["availableBalance"].(float64); ok {
		available = v
	}
	if available <= 0 {
		return 0, 0, 0, fmt.Errorf("available balance is zero or unknown")
	}

	// 价格：优先使用 entry price，否则当前市场价
	price := mkt.CurrentPrice
	if d.EntryPrice > 0 {
		price = d.EntryPrice
	}
	if price <= 0 {
		return 0, 0, 0, fmt.Errorf("invalid market/entry price: %.8f", price)
	}

	// 杠杆
	leverage := d.Leverage
	if leverage <= 0 {
		// fallback to BTC/ALT leverage depending on symbol
		if strings.Contains(strings.ToUpper(d.Symbol), "BTC") || strings.Contains(strings.ToUpper(d.Symbol), "ETH") {
			leverage = at.config.BTCETHLeverage
		} else {
			leverage = at.config.AltcoinLeverage
		}
		if leverage <= 0 {
			leverage = 1
		}
	}

	// 止损比例 (绝对值)
	stop := d.StopLoss
	side := "LONG"
	if strings.Contains(strings.ToLower(d.Action), "short") {
		side = "SHORT"
	}

	stopPct := 0.0
	if stop > 0 {
		if side == "LONG" {
			if price <= stop {
				return 0, 0, 0, fmt.Errorf("long stop_loss must be less than entry/current price")
			}
			stopPct = (price - stop) / price
		} else {
			if price >= stop {
				return 0, 0, 0, fmt.Errorf("short stop_loss must be greater than entry/current price")
			}
			stopPct = (stop - price) / price
		}
	}
	// 如果没有 stop 或者 stopPct == 0, 使用默认 stop pct
	if stopPct <= 0 {
		stopPct = cfg.DefaultStopLossPct
	}
	if stopPct <= 0 {
		// 保险防护
		stopPct = 0.01
	}

	// 单笔风险价值 (USD)
	riskUSD := available * cfg.MaxSingleTradeRiskPct
	if d.RiskUSD > 0 {
		// 如果AI提供特定 RiskUSD(可选），但不要超出 cfg 的单笔risk上限
		if d.RiskUSD < riskUSD {
			riskUSD = d.RiskUSD
		}
	}

	// 通过止损比例反算最大名义价值
	maxNotionalByRisk := 0.0
	if stopPct > 0 {
		maxNotionalByRisk = riskUSD / stopPct
	}

	// 币种单独名义上限
	useMaxNotional := cfg.MaxNotionalAlt
	upSym := strings.ToUpper(d.Symbol)
	if strings.Contains(upSym, "BTC") || strings.Contains(upSym, "ETH") {
		useMaxNotional = cfg.MaxNotionalBTC
	}

	// 初始最终名义: 由风险得出
	finalNotional := maxNotionalByRisk
	// cap by config maximum
	if useMaxNotional > 0 && finalNotional > useMaxNotional {
		finalNotional = useMaxNotional
	}

	// Make sure margin requirement fits available balance
	requiredMargin := finalNotional / float64(leverage)
	if requiredMargin > available {
		// reduce notional to available balance * leverage (leave small headroom)
		finalNotional = available * float64(leverage) * 0.99
		requiredMargin = finalNotional / float64(leverage)
	}

	// Safety: force finalNotional at least minimal exchange notional (conservative)
	const minNotional = 10.0
	if finalNotional < minNotional {
		return 0, 0, 0, fmt.Errorf("final notional %.2f USDT is below minimum notional %.2f USDT", finalNotional, minNotional)
	}

	// Ensure finalNotional is positive
	finalNotional = math.Max(0, finalNotional)

	// quantity
	quantity := finalNotional / price
	if quantity <= 0 {
		return 0, 0, 0, fmt.Errorf("computed quantity <= 0")
	}

	return finalNotional, quantity, riskUSD, nil
}
