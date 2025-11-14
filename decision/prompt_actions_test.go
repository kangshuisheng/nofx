package decision

import (
	"strings"
	"testing"
)

// TestPromptContainsAllValidActions tests that the AI prompt includes all 9 valid actions
// This test verifies fix for issue #982/#984
func TestPromptContainsAllValidActions(t *testing.T) {
	// Generate the prompt
	prompt := buildSystemPrompt(100.0, 5, 5, "default")

	// Define all 9 valid actions that must be present in the prompt
	validActions := []string{
		"open_long",
		"open_short",
		"close_long",
		"close_short",
		"update_stop_loss",   // Issue #982: This was missing
		"update_take_profit", // Issue #982: This was missing
		"partial_close",      // Issue #982: This was missing
		"hold",
		"wait",
	}

	// Verify each action is mentioned in the prompt
	for _, action := range validActions {
		if !strings.Contains(prompt, action) {
			t.Errorf("❌ AI prompt is missing valid action: %s", action)
			t.Logf("This would cause AI to guess action names and fail validation")
		}
	}

	// Verify the action list appears in the field description
	actionListPattern := "open_long | open_short | close_long | close_short | update_stop_loss | update_take_profit | partial_close | hold | wait"
	if !strings.Contains(prompt, actionListPattern) {
		t.Errorf("❌ Prompt does not contain the complete action list")
		t.Logf("Expected pattern: %s", actionListPattern)

		// Print the actual action section for debugging
		if idx := strings.Index(prompt, "action:"); idx != -1 {
			end := idx + 200
			if end > len(prompt) {
				end = len(prompt)
			}
			t.Logf("Actual action section: %s", prompt[idx:end])
		}
	} else {
		t.Logf("✅ Prompt contains all 9 valid actions")
	}
}

// TestValidateDecisionAcceptsAllActions verifies that validateDecision accepts all 9 actions
// This ensures the prompt and validation logic are in sync
func TestValidateDecisionAcceptsAllActions(t *testing.T) {
	validActions := []string{
		"open_long",
		"open_short",
		"close_long",
		"close_short",
		"update_stop_loss",
		"update_take_profit",
		"partial_close",
		"hold",
		"wait",
	}

	for _, action := range validActions {
		t.Run(action, func(t *testing.T) {
			decision := Decision{
				Symbol:     "BTCUSDT",
				Action:     action,
				Reasoning:  "Test reasoning",
				Confidence: 80,
			}

			// For open actions, add required fields
			if action == "open_long" {
				decision.Leverage = 5
				decision.PositionSizeUSD = 100
				decision.StopLoss = 95000
				decision.TakeProfit = 105000
				decision.RiskUSD = 50
			}
			if action == "open_short" {
				decision.Leverage = 5
				decision.PositionSizeUSD = 100
				decision.StopLoss = 105000 // For short: stop loss > take profit
				decision.TakeProfit = 95000
				decision.RiskUSD = 50
			}

			// For update/partial actions, add required fields
			if action == "update_stop_loss" {
				decision.NewStopLoss = 96000
			}
			if action == "update_take_profit" {
				decision.NewTakeProfit = 104000
			}
			if action == "partial_close" {
				decision.ClosePercentage = 50
			}

			err := validateDecision(&decision, 100.0, 5, 5)
			if err != nil {
				t.Errorf("❌ validateDecision rejected valid action '%s': %v", action, err)
			} else {
				t.Logf("✅ Action '%s' is accepted by validation", action)
			}
		})
	}
}

// TestPromptAndValidationInSync verifies that prompt and validation use the same action set
func TestPromptAndValidationInSync(t *testing.T) {
	prompt := buildSystemPrompt(100.0, 5, 5, "default")

	// Expected actions from validateDecision
	expectedActions := []string{
		"open_long",
		"open_short",
		"close_long",
		"close_short",
		"update_stop_loss",
		"update_take_profit",
		"partial_close",
		"hold",
		"wait",
	}

	// Verify all expected actions are in prompt
	missingActions := []string{}
	for _, expected := range expectedActions {
		if !strings.Contains(prompt, expected) {
			missingActions = append(missingActions, expected)
		}
	}

	if len(missingActions) > 0 {
		t.Errorf("❌ Prompt is missing %d actions: %v", len(missingActions), missingActions)
	} else {
		t.Logf("✅ Prompt and validation are in sync with all %d actions", len(expectedActions))
	}
}
