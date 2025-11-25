package decision

import "nofx/config"

// For backward compatibility, re-export RiskConfig and DefaultRiskConfig.
// This allows existing code in other packages to continue calling
// decision.DefaultRiskConfig() while the canonical implementation is in config.
type RiskConfig = config.RiskConfig

func DefaultRiskConfig() *RiskConfig {
	// Note: type alias ensures *RiskConfig is identical to *config.RiskConfig
	return config.DefaultRiskConfig()
}
