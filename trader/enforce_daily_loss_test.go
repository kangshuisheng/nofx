package trader

import (
	"testing"
	"time"
)

// Reproducer: If the trader's equity drops due to a pending order (i.e. unrealized change)
// we should not treat that as a realized daily loss trigger. This test encodes the
// current (buggy) behaviour expectation so it fails and guides the fix.
func TestEnforceRiskLimitsCountsUnrealizedLoss_Buggy(t *testing.T) {
	at := &AutoTrader{}

	// configure a non-zero daily baseline and ensure we don't re-sync baseline
	at.dailyPnLBase = 120.18
	at.needsDailyBaseline = false

	// dailyRealizedPnL is 0 (no realized losses yet)
	at.dailyRealizedPnL = 0

	// config with 10% max daily loss
	at.config.MaxDailyLoss = 10.0
	at.config.StopTradingTime = 1 * time.Minute

	// Simulate equity drop caused by unrealized / reserved funds (e.g., limit order)
	currentEquity := 120.18 - 18.47 // -> -18.47 absolute

	reason, triggered := at.enforceRiskLimits(currentEquity)

	// Desired behaviour: unrealized equity changes (e.g., caused by an unfilled limit order)
	// should NOT be counted as realized daily losses and therefore SHOULD NOT trigger
	if triggered {
		t.Fatalf("expected risk limits NOT to trigger for unrealized loss, but it did: reason=%q", reason)
	}
}

func TestEnforceRiskLimitsTriggersOnRealizedLoss(t *testing.T) {
	at := &AutoTrader{}

	// configure daily baseline
	at.dailyPnLBase = 1000
	at.needsDailyBaseline = false

	// simulate realized losses (e.g., closed trades)
	at.dailyRealizedPnL = -70.0

	// config with 10% max daily loss -> threshold = -12.018
	at.config.MaxDailyLoss = 10.0
	at.config.StopTradingTime = 1 * time.Minute

	// currentEquity larger or smaller doesn't matter because enforcement should
	// use realizedPnL only. Provide currentEquity that's not deeply negative.
	currentEquity := 100.0

	reason, triggered := at.enforceRiskLimits(currentEquity)

	if !triggered {
		t.Fatalf("expected risk limits to trigger for realized loss, but it did not: reason=%q", reason)
	}
}
