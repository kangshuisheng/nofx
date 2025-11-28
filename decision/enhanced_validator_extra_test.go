package decision

import (
    "math"
    "nofx/market"
    "testing"

    "github.com/stretchr/testify/assert"
)

// TestEnhancedValidator_ClampsLargeAISuggestion verifies that when AI suggests a size
// larger than system limit the validator will clamp it (warning) and not return an error.
func TestEnhancedValidator_ClampsLargeAISuggestion(t *testing.T) {
    // Construct a mock market where price = 100.0
    mkt := &market.Data{CurrentPrice: 100.0}

    // Decision uses a stop loss that results in finalNotional around 43.2 (with default risk 2%)
    d := &Decision{
        Symbol:                 "DOGEUSDT", // altcoin uses MaxNotionalAlt=60 by default
        Action:                 "open_long",
        Leverage:               1,
        EntryPrice:             100.0,
        StopLoss:               53.74, // stopPct ~= 0.4626 -> finalNotional ~= (equity*0.02*0.9)/0.4626
        SuggestedPositionSizeUSD: 60.0, // intentionally above computed finalNotional (~43)
    }

    // Use accountEquity 1000 -> riskUSD=20 -> targetRiskUSD=18 -> maxNotionalByRisk ~= 18/0.4626 ~= 38.9
    // But default MaxNotionalAlt = 60, so finalNotional expected ~= 38.9
    // Call validation with mock market data
    err := validateDecisionWithMarketData(d, 1000.0, 10, 5, nil, mkt)

    assert.NoError(t, err, "validator should not error when AI suggested position > final notional; it should clamp")

    // After validation the SuggestedPositionSizeUSD should be clamped to something <= the computed safe notional
    assert.True(t, d.SuggestedPositionSizeUSD <= 60.0)
    // make sure it was actually reduced from original suggested value
    assert.True(t, d.SuggestedPositionSizeUSD < 60.0)
    // It should be a finite positive number
    assert.True(t, d.SuggestedPositionSizeUSD > 0.0)
    assert.False(t, math.IsNaN(d.SuggestedPositionSizeUSD))
}
