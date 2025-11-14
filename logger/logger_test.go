package logger

import (
	"bytes"
	"io"
	"testing"

	"github.com/sirupsen/logrus"
)

// TestInit tests basic logger initialization
func TestInit(t *testing.T) {
	// Test with nil config (should use defaults)
	err := Init(nil)
	if err != nil {
		t.Errorf("Init with nil config failed: %v", err)
	}

	if Log == nil {
		t.Fatal("Log global variable is nil after Init")
	}

	if Log.Level != logrus.InfoLevel {
		t.Errorf("Expected Info level, got %v", Log.Level)
	}
}

// TestInitWithSimpleConfig tests initialization with simple configuration
func TestInitWithSimpleConfig(t *testing.T) {
	tests := []struct {
		name          string
		level         string
		expectedLevel logrus.Level
	}{
		{
			name:          "Debug level",
			level:         "debug",
			expectedLevel: logrus.DebugLevel,
		},
		{
			name:          "Info level",
			level:         "info",
			expectedLevel: logrus.InfoLevel,
		},
		{
			name:          "Warn level",
			level:         "warn",
			expectedLevel: logrus.WarnLevel,
		},
		{
			name:          "Error level",
			level:         "error",
			expectedLevel: logrus.ErrorLevel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := InitWithSimpleConfig(tt.level)
			if err != nil {
				t.Errorf("InitWithSimpleConfig failed: %v", err)
			}

			if Log.Level != tt.expectedLevel {
				t.Errorf("Expected %v level, got %v", tt.expectedLevel, Log.Level)
			}
		})
	}
}

// TestInitFromParams tests initialization with parameters
func TestInitFromParams(t *testing.T) {
	// Test without Telegram
	err := InitFromParams("info", false, "", 0)
	if err != nil {
		t.Errorf("InitFromParams without Telegram failed: %v", err)
	}

	if Log == nil {
		t.Fatal("Log is nil")
	}

	// Note: Testing with Telegram requires valid bot token and chat ID
	// which we don't have in test environment, so we skip it
}

// TestLoggingFunctions tests various logging functions
func TestLoggingFunctions(t *testing.T) {
	// Initialize logger with buffer to capture output
	err := InitWithSimpleConfig("debug")
	if err != nil {
		t.Fatalf("Failed to initialize logger: %v", err)
	}

	// Create a buffer to capture output
	var buf bytes.Buffer
	Log.SetOutput(&buf)

	// Test Debug
	Debug("debug message")
	if buf.Len() == 0 {
		t.Error("Debug message was not written")
	}
	buf.Reset()

	// Test Info
	Info("info message")
	if buf.Len() == 0 {
		t.Error("Info message was not written")
	}
	buf.Reset()

	// Test Warn
	Warn("warn message")
	if buf.Len() == 0 {
		t.Error("Warn message was not written")
	}
	buf.Reset()

	// Test Error
	Error("error message")
	if buf.Len() == 0 {
		t.Error("Error message was not written")
	}
	buf.Reset()

	// Test formatted logging
	Debugf("debug %s", "formatted")
	if buf.Len() == 0 {
		t.Error("Debugf message was not written")
	}
	buf.Reset()

	Infof("info %s", "formatted")
	if buf.Len() == 0 {
		t.Error("Infof message was not written")
	}
	buf.Reset()

	Warnf("warn %s", "formatted")
	if buf.Len() == 0 {
		t.Error("Warnf message was not written")
	}
	buf.Reset()

	Errorf("error %s", "formatted")
	if buf.Len() == 0 {
		t.Error("Errorf message was not written")
	}
}

// TestWithFields tests logging with fields
func TestWithFields(t *testing.T) {
	err := InitWithSimpleConfig("info")
	if err != nil {
		t.Fatalf("Failed to initialize logger: %v", err)
	}

	var buf bytes.Buffer
	Log.SetOutput(&buf)

	// Test WithFields
	entry := WithFields(logrus.Fields{
		"trader_id": "test-123",
		"action":    "open_long",
	})

	if entry == nil {
		t.Fatal("WithFields returned nil")
	}

	entry.Info("test message with fields")
	output := buf.String()

	if len(output) == 0 {
		t.Error("WithFields log message was not written")
	}

	// Check if fields are present in output
	// Note: exact format depends on logrus formatter
	if !contains(output, "test message with fields") {
		t.Error("Message content not found in output")
	}
}

// TestWithField tests logging with single field
func TestWithField(t *testing.T) {
	err := InitWithSimpleConfig("info")
	if err != nil {
		t.Fatalf("Failed to initialize logger: %v", err)
	}

	var buf bytes.Buffer
	Log.SetOutput(&buf)

	entry := WithField("trader_id", "test-456")
	if entry == nil {
		t.Fatal("WithField returned nil")
	}

	entry.Info("single field test")
	output := buf.String()

	if len(output) == 0 {
		t.Error("WithField log message was not written")
	}

	if !contains(output, "single field test") {
		t.Error("Message content not found in output")
	}
}

// TestShutdown tests graceful shutdown
func TestShutdown(t *testing.T) {
	err := InitWithSimpleConfig("info")
	if err != nil {
		t.Fatalf("Failed to initialize logger: %v", err)
	}

	// Call Shutdown (should not panic even if no Telegram hook)
	Shutdown()

	// Verify telegramHook is nil
	if telegramHook != nil {
		t.Error("telegramHook should be nil after Shutdown")
	}
}

// TestLogLevelParsing tests invalid log level handling
func TestLogLevelParsing(t *testing.T) {
	// Test with invalid level (should default to info)
	err := Init(&Config{Level: "invalid-level"})
	if err != nil {
		t.Errorf("Init with invalid level failed: %v", err)
	}

	// Should fallback to InfoLevel
	if Log.Level != logrus.InfoLevel {
		t.Errorf("Expected InfoLevel as fallback, got %v", Log.Level)
	}
}

// TestConfigSetDefaults tests Config.SetDefaults
func TestConfigSetDefaults(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()

	if cfg.Level == "" {
		t.Error("SetDefaults did not set Level")
	}

	// Test with existing values (should not override)
	cfg2 := &Config{Level: "debug"}
	originalLevel := cfg2.Level
	cfg2.SetDefaults()

	if cfg2.Level != originalLevel {
		t.Error("SetDefaults should not override existing Level")
	}
}

// TestReporter tests logger output capture
func TestReporterCapture(t *testing.T) {
	err := InitWithSimpleConfig("debug")
	if err != nil {
		t.Fatalf("Failed to initialize logger: %v", err)
	}

	// Capture output
	var buf bytes.Buffer
	oldOutput := Log.Out
	Log.SetOutput(&buf)
	defer Log.SetOutput(oldOutput)

	// Log at different levels
	testMessages := []struct {
		level   string
		message string
		logFunc func(...interface{})
	}{
		{"debug", "debug test", Debug},
		{"info", "info test", Info},
		{"warn", "warn test", Warn},
		{"error", "error test", Error},
	}

	for _, tm := range testMessages {
		buf.Reset()
		tm.logFunc(tm.message)

		output := buf.String()
		if !contains(output, tm.message) {
			t.Errorf("Expected %s message in output, got: %s", tm.level, output)
		}
	}
}

// TestConcurrentLogging tests concurrent logging safety
func TestConcurrentLogging(t *testing.T) {
	err := InitWithSimpleConfig("info")
	if err != nil {
		t.Fatalf("Failed to initialize logger: %v", err)
	}

	// Discard output to focus on concurrency safety
	Log.SetOutput(io.Discard)

	// Run concurrent logging operations
	done := make(chan bool)
	numGoroutines := 10
	messagesPerGoroutine := 100

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			for j := 0; j < messagesPerGoroutine; j++ {
				Infof("Goroutine %d - Message %d", id, j)
				WithField("goroutine", id).Debugf("Debug message %d", j)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines to finish
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	t.Log("✅ Concurrent logging completed without panic")
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsSubstring(s, substr)
}

func containsSubstring(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
