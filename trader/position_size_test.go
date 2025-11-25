package trader

import (
	"nofx/decision"
	"nofx/market"
	"testing"

	"github.com/stretchr/testify/assert"
)

// fakeTrader implements Trader minimal method used by ComputePositionSize
type fakeTrader struct{ balance map[string]interface{} }

func (f *fakeTrader) GetBalance() (map[string]interface{}, error) {
	if f.balance == nil {
		return map[string]interface{}{"availableBalance": 10000.0}, nil
	}
	return f.balance, nil
}
func (f *fakeTrader) GetPositions() ([]map[string]interface{}, error) { return nil, nil }
func (f *fakeTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	return nil, nil
}
func (f *fakeTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	return nil, nil
}
func (f *fakeTrader) OpenLongLimit(symbol string, quantity float64, price float64, leverage int) (map[string]interface{}, error) {
	return nil, nil
}
func (f *fakeTrader) OpenShortLimit(symbol string, quantity float64, price float64, leverage int) (map[string]interface{}, error) {
	return nil, nil
}
func (f *fakeTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	return nil, nil
}
func (f *fakeTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	return nil, nil
}
func (f *fakeTrader) SetLeverage(symbol string, leverage int) error         { return nil }
func (f *fakeTrader) SetMarginMode(symbol string, isCrossMargin bool) error { return nil }
func (f *fakeTrader) GetMarketPrice(symbol string) (float64, error)         { return 0, nil }
func (f *fakeTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	return nil
}
func (f *fakeTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	return nil
}
func (f *fakeTrader) CancelStopLossOrders(symbol string) error                       { return nil }
func (f *fakeTrader) CancelTakeProfitOrders(symbol string) error                     { return nil }
func (f *fakeTrader) CancelAllOrders(symbol string) error                            { return nil }
func (f *fakeTrader) CancelStopOrders(symbol string) error                           { return nil }
func (f *fakeTrader) UpdateStopLoss(symbol, side string, stopPrice float64) error    { return nil }
func (f *fakeTrader) FormatQuantity(symbol string, quantity float64) (string, error) { return "", nil }
func (f *fakeTrader) GetOpenOrders(symbol string) ([]decision.OpenOrderInfo, error) {
	return []decision.OpenOrderInfo{}, nil
}

// TestComputePositionSize_Long_CappedByMaxNotional ensures that even if the calculated
// risk-based notional is huge, it is capped by MaxNotionalBTC default config (80 USDT)
func TestComputePositionSize_Long_CappedByMaxNotional(t *testing.T) {
	at := &AutoTrader{
		trader: &fakeTrader{},
		config: AutoTraderConfig{BTCETHLeverage: 10, AltcoinLeverage: 5},
	}

	d := &decision.Decision{
		Symbol:     "BTCUSDT",
		Action:     "open_long",
		Leverage:   10,
		StopLoss:   49500.0,
		EntryPrice: 50500.0,
	}
	// market data with price 50500
	mkt := &market.Data{CurrentPrice: 50500}

	notional, quantity, _, err := ComputePositionSize(at, d, mkt)
	assert.NoError(t, err)
	// default max notional btc = 80 in config
	assert.LessOrEqual(t, notional, 80.0)
	// quantity equals notional / price
	expectedQty := notional / 50500.0
	assert.InDelta(t, expectedQty, quantity, 1e-8)
}

// TestComputePositionSize_Long_RiskBased ensures computed notional limited by risk when stop is tight
func TestComputePositionSize_Long_RiskBased(t *testing.T) {
	at := &AutoTrader{
		trader: &fakeTrader{},
		config: AutoTraderConfig{BTCETHLeverage: 10, AltcoinLeverage: 5},
	}

	d := &decision.Decision{
		Symbol:     "BTCUSDT",
		Action:     "open_long",
		Leverage:   10,
		StopLoss:   50300.0, // very tight stop (price 50500 -> 0.39% stop)
		EntryPrice: 50500.0,
	}
	mkt := &market.Data{CurrentPrice: 50500}

	notional, quantity, riskUSD, err := ComputePositionSize(at, d, mkt)
	assert.NoError(t, err)
	// with availableBal 10000, max single trade risk 0.02 => riskUSD = 200
	// stopPct = (50500-50300)/50500 = ~0.003960; maxNotionalByRisk = 200/0.00396 = ~50505 -> capped by max notional (80)
	// So expect still <= 80
	assert.LessOrEqual(t, notional, 80.0)
	assert.Greater(t, riskUSD, 0.0)
	assert.InDelta(t, notional/50500.0, quantity, 1e-8)
}

// TestComputePositionSize_MinNotionalError ensures very small final notional triggers error
func TestComputePositionSize_MinNotionalError(t *testing.T) {
	at := &AutoTrader{
		trader: &fakeTrader{},
		config: AutoTraderConfig{BTCETHLeverage: 10, AltcoinLeverage: 5},
	}

	d := &decision.Decision{
		Symbol:     "BTCUSDT",
		Action:     "open_long",
		Leverage:   10,
		StopLoss:   25250.0, // set stop to 50% distance -> stopPct=0.5
		RiskUSD:    1.0,     // small risk to produce small finalNotional
		EntryPrice: 50500.0,
	}
	mkt := &market.Data{CurrentPrice: 50500}

	_, _, _, err := ComputePositionSize(at, d, mkt)
	if err == nil {
		t.Fatalf("expected error for min notional, got nil")
	}
}

// TestComputePositionSize_AltCoin_MaxNotional ensures alt coin uses MaxNotionalAlt (60)
func TestComputePositionSize_AltCoin_MaxNotional(t *testing.T) {
	at := &AutoTrader{
		trader: &fakeTrader{},
		config: AutoTraderConfig{BTCETHLeverage: 10, AltcoinLeverage: 5},
	}

	d := &decision.Decision{
		Symbol:     "DOGEUSDT",
		Action:     "open_long",
		Leverage:   5,
		StopLoss:   0.0029,
		EntryPrice: 0.003,
	}
	mkt := &market.Data{CurrentPrice: 0.003}

	notional, _, _, err := ComputePositionSize(at, d, mkt)
	assert.NoError(t, err)
	// default MaxNotionalAlt = 60
	assert.LessOrEqual(t, notional, 60.0)
}

// TestComputePositionSize_MarginCap ensures finalNotional is reduced when margin requirement exceeds available balance
func TestComputePositionSize_MarginCap(t *testing.T) {
	ft := &fakeTrader{balance: map[string]interface{}{"availableBalance": 20.0}}
	at := &AutoTrader{
		trader: ft,
		config: AutoTraderConfig{BTCETHLeverage: 10, AltcoinLeverage: 5},
	}

	d := &decision.Decision{
		Symbol:     "BTCUSDT",
		Action:     "open_long",
		Leverage:   10,
		StopLoss:   50499.0, // very tight stop (price 50500 -> stopPct ~0.0000198) to amplify notional
		EntryPrice: 50500.0,
		RiskUSD:    1000.0, // large risk so maxNotionalByRisk huge
	}
	mkt := &market.Data{CurrentPrice: 50500}

	notional, _, _, err := ComputePositionSize(at, d, mkt)
	assert.NoError(t, err)
	// requiredMargin is notional/leverage, which must be <= availableBalance; expected cap by avail*leverage*0.99
	expectedCap := 20.0 * float64(d.Leverage) * 0.99
	assert.LessOrEqual(t, notional, expectedCap)
}
