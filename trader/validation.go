package trader

import (
	"fmt"
	"nofx/config"
)

// ValidateNotional 在下单前校验名义价值是否超过允许上限
func ValidateNotional(symbol string, notionalValue float64) error {
	cfg := config.DefaultRiskConfig()
	var maxNotional float64
	if symbol == "BTCUSDT" || symbol == "ETHUSDT" {
		maxNotional = cfg.MaxNotionalBTC
	} else {
		maxNotional = cfg.MaxNotionalAlt
	}
	if maxNotional > 0 && notionalValue > maxNotional {
		return fmt.Errorf("❌ 名义价值超限: %.2f USDT > 最大允许 %.2f USDT (符号: %s)",
			notionalValue, maxNotional, symbol)
	}
	return nil
}
