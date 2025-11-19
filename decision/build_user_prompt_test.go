package decision

import (
	"strings"
	"testing"
	"time"

	"nofx/market"
)

// TestBuildUserPromptExcludesHeldPositions 验证候选币种列表中排除已持仓符号
func TestBuildUserPromptExcludesHeldPositions(t *testing.T) {
	ctx := &Context{}
	ctx.CurrentTime = time.Now().Format(time.RFC3339)
	ctx.CallCount = 1
	ctx.RuntimeMinutes = 5

	// 模拟持仓：BTCUSDT
	ctx.Positions = []PositionInfo{{Symbol: "BTCUSDT", Side: "long", EntryPrice: 100.0, MarkPrice: 110.0}}

	// 候选币种中包含 BTCUSDT（应被排除）、以及 SOLUSDT（应显示）
	ctx.CandidateCoins = []CandidateCoin{{Symbol: "BTCUSDT"}, {Symbol: "SOLUSDT"}}

	// 提供 MarketDataMap，其中包括两个符号
	ctx.MarketDataMap = map[string]*market.Data{
		"BTCUSDT": {Symbol: "BTCUSDT", CurrentPrice: 110, OpenInterest: &market.OIData{Change4h: 0}, LongerTermContext: &market.LongerTermData{ATR14: 0}},
		"SOLUSDT": {Symbol: "SOLUSDT", CurrentPrice: 5.0, OpenInterest: &market.OIData{Change4h: 0}, LongerTermContext: &market.LongerTermData{ATR14: 0}},
	}

	out := buildUserPrompt(ctx)

	// 候选币种计数应该为 1 个（排除了 BTCUSDT）
	if !strings.Contains(out, "候选币种 (1个)") {
		t.Fatalf("expected candidate coins count 1, got output:\n%s", out)
	}

	// 确认候选币种中包含 SOLUSDT，但不包含 BTCUSDT（在候选列表中）
	if !strings.Contains(out, "SOLUSDT") {
		t.Fatalf("expected SOLUSDT in candidate list, got output:\n%s", out)
	}

	// 确认当前持仓部分显示 BTCUSDT
	if !strings.Contains(out, "当前持仓") || !strings.Contains(out, "BTCUSDT") {
		t.Fatalf("expected BTCUSDT to appear in current positions, got output:\n%s", out)
	}
}
