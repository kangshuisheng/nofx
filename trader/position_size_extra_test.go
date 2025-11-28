package trader

import (
    "nofx/decision"
    "nofx/market"
    "testing"

    "github.com/stretchr/testify/assert"
)

// TestComputePositionSize_RespectsAISuggestion ensures that when the AI suggests a smaller
// suggested_position_size_usd than the computed safe notional, the final notional equals the suggestion.
func TestComputePositionSize_RespectsAISuggestion(t *testing.T) {
    at := &AutoTrader{
        trader: &fakeTrader{},
        config: AutoTraderConfig{BTCETHLeverage: 10, AltcoinLeverage: 5},
    }

    d := &decision.Decision{
        Symbol:                 "BTCUSDT",
        Action:                 "open_long",
        Leverage:               10,
        StopLoss:               50300.0,
        EntryPrice:             50500.0,
        SuggestedPositionSizeUSD: 40.0, // smaller than computed safe notional
    }
    mkt := &market.Data{CurrentPrice: 50500}

    notional, _, _, err := ComputePositionSize(at, d, mkt)
    assert.NoError(t, err)
    assert.InDelta(t, 40.0, notional, 1e-6, "final notional should respect AI's smaller suggestion")
}
