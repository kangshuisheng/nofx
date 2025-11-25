package trader

import (
	"nofx/config"
	"testing"
)

func TestValidateNotional_RejectsAboveLimit(t *testing.T) {
	cfg := config.DefaultRiskConfig()
	cfg.MaxNotionalBTC = 100.0 // 100 USDT 上限（测试场景）

	if err := ValidateNotional("BTCUSDT", 200.0); err == nil {
		t.Fatalf("expected error for notional 200 > 100, got nil")
	}
}

func TestValidateNotional_AllowBelowLimit(t *testing.T) {
	cfg := config.DefaultRiskConfig()
	cfg.MaxNotionalBTC = 100.0

	if err := ValidateNotional("BTCUSDT", 50.0); err != nil {
		t.Fatalf("unexpected error for notional 50 <= 100: %v", err)
	}
}
