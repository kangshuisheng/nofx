package market

import (
	"testing"
)

func TestCalculateADX(t *testing.T) {
	// Create a simple trend scenario
	// Price increasing steadily
	klines := make([]Kline, 50)
	startPrice := 100.0
	for i := 0; i < 50; i++ {
		klines[i] = Kline{
			High:  startPrice + float64(i)*2 + 2,
			Low:   startPrice + float64(i)*2 - 2,
			Close: startPrice + float64(i)*2,
		}
	}

	// Calculate ADX with period 14
	adx := calculateADX(klines, 14)

	// In a strong trend, ADX should be high (e.g., > 25)
	if adx < 25 {
		t.Errorf("Expected high ADX for strong trend, got %f", adx)
	}

	// Create a choppy scenario
	// Price oscillating
	klinesChoppy := make([]Kline, 50)
	for i := 0; i < 50; i++ {
		base := 100.0
		if i%2 == 0 {
			klinesChoppy[i] = Kline{High: base + 5, Low: base - 5, Close: base + 2}
		} else {
			klinesChoppy[i] = Kline{High: base + 5, Low: base - 5, Close: base - 2}
		}
	}

	adxChoppy := calculateADX(klinesChoppy, 14)

	// In a choppy market, ADX should be low (e.g., < 20)
	// Note: Perfect oscillation might result in 0 directional movement, so low ADX
	if adxChoppy > 25 {
		t.Errorf("Expected low ADX for choppy market, got %f", adxChoppy)
	}
}
