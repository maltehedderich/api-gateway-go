package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestLoggerLevels(t *testing.T) {
	var buf bytes.Buffer
	logger := New(InfoLevel, "json", &buf)

	// Debug should not be logged at Info level
	logger.Debug("debug message")
	if buf.Len() > 0 {
		t.Error("Debug message should not be logged at Info level")
	}

	// Info should be logged
	buf.Reset()
	logger.Info("info message")
	if buf.Len() == 0 {
		t.Error("Info message should be logged at Info level")
	}

	// Warn should be logged
	buf.Reset()
	logger.Warn("warn message")
	if buf.Len() == 0 {
		t.Error("Warn message should be logged at Info level")
	}

	// Error should be logged
	buf.Reset()
	logger.Error("error message")
	if buf.Len() == 0 {
		t.Error("Error message should be logged at Info level")
	}
}

func TestComponentLevel(t *testing.T) {
	var buf bytes.Buffer
	logger := New(WarnLevel, "json", &buf)
	logger.SetComponentLevel("test-component", DebugLevel)

	compLogger := logger.WithComponent("test-component")

	// Debug should be logged for component even though global level is Warn
	compLogger.Debug("debug message")
	if buf.Len() == 0 {
		t.Error("Debug message should be logged for component with Debug level")
	}
}

func TestJSONFormat(t *testing.T) {
	var buf bytes.Buffer
	logger := New(InfoLevel, "json", &buf)

	logger.Info("test message", Fields{
		"key1": "value1",
		"key2": 42,
	})

	var entry Entry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("Failed to parse JSON log: %v", err)
	}

	if entry.Message != "test message" {
		t.Errorf("Expected message 'test message', got '%s'", entry.Message)
	}
	if entry.Level != "INFO" {
		t.Errorf("Expected level INFO, got %s", entry.Level)
	}
	if entry.Fields["key1"] != "value1" {
		t.Errorf("Expected field key1=value1, got %v", entry.Fields["key1"])
	}
}

func TestTextFormat(t *testing.T) {
	var buf bytes.Buffer
	logger := New(InfoLevel, "text", &buf)

	logger.Info("test message", Fields{
		"key1": "value1",
	})

	output := buf.String()
	if !strings.Contains(output, "INFO") {
		t.Error("Text output should contain log level")
	}
	if !strings.Contains(output, "test message") {
		t.Error("Text output should contain message")
	}
	if !strings.Contains(output, "key1=value1") {
		t.Error("Text output should contain fields")
	}
}

func TestSanitization(t *testing.T) {
	var buf bytes.Buffer
	logger := New(InfoLevel, "json", &buf)

	// Set sanitize patterns
	err := logger.SetSanitizePatterns([]string{
		"(?i)password",
		"(?i)token",
		"(?i)secret",
	})
	if err != nil {
		t.Fatalf("Failed to set sanitize patterns: %v", err)
	}

	logger.Info("sensitive data", Fields{
		"password": "mySecretPassword123",
		"token":    "Bearer abc123def456",
		"username": "john",
	})

	var entry Entry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("Failed to parse JSON log: %v", err)
	}

	// Password should be sanitized
	if !strings.HasPrefix(entry.Fields["password"].(string), "***") {
		t.Error("Password field should be sanitized")
	}

	// Token should be sanitized
	if !strings.HasPrefix(entry.Fields["token"].(string), "***") {
		t.Error("Token field should be sanitized")
	}

	// Username should not be sanitized
	if entry.Fields["username"] != "john" {
		t.Error("Username field should not be sanitized")
	}
}

func TestCorrelationID(t *testing.T) {
	var buf bytes.Buffer
	logger := New(InfoLevel, "json", &buf)

	compLogger := logger.WithComponent("test")
	ctxLogger := compLogger.WithCorrelationID("corr-123")

	ctxLogger.Info("test message")

	var entry Entry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("Failed to parse JSON log: %v", err)
	}

	if entry.CorrelationID != "corr-123" {
		t.Errorf("Expected correlation ID corr-123, got %s", entry.CorrelationID)
	}
	if entry.Component != "test" {
		t.Errorf("Expected component test, got %s", entry.Component)
	}
}

func TestContextCorrelationID(t *testing.T) {
	ctx := context.Background()
	correlationID := "ctx-corr-456"

	ctx = WithCorrelationID(ctx, correlationID)
	retrieved := GetCorrelationID(ctx)

	if retrieved != correlationID {
		t.Errorf("Expected correlation ID %s, got %s", correlationID, retrieved)
	}
}

func TestFromContext(t *testing.T) {
	var buf bytes.Buffer
	Init(InfoLevel, "json", &buf)

	ctx := WithCorrelationID(context.Background(), "ctx-123")
	ctxLogger := FromContext(ctx, "test-component")

	ctxLogger.Info("test message")

	var entry Entry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("Failed to parse JSON log: %v", err)
	}

	if entry.CorrelationID != "ctx-123" {
		t.Errorf("Expected correlation ID ctx-123, got %s", entry.CorrelationID)
	}
	if entry.Component != "test-component" {
		t.Errorf("Expected component test-component, got %s", entry.Component)
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected Level
		wantErr  bool
	}{
		{"debug", DebugLevel, false},
		{"DEBUG", DebugLevel, false},
		{"info", InfoLevel, false},
		{"warn", WarnLevel, false},
		{"warning", WarnLevel, false},
		{"error", ErrorLevel, false},
		{"fatal", FatalLevel, false},
		{"invalid", InfoLevel, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			level, err := ParseLevel(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseLevel(%s) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if !tt.wantErr && level != tt.expected {
				t.Errorf("ParseLevel(%s) = %v, want %v", tt.input, level, tt.expected)
			}
		})
	}
}

func TestMergeFields(t *testing.T) {
	f1 := Fields{"a": 1, "b": 2}
	f2 := Fields{"c": 3, "d": 4}
	f3 := Fields{"b": 5} // Should override b from f1

	result := mergeFields(f1, f2, f3)

	if result["a"] != 1 {
		t.Error("Expected a=1")
	}
	if result["b"] != 5 {
		t.Error("Expected b=5 (overridden)")
	}
	if result["c"] != 3 {
		t.Error("Expected c=3")
	}
	if result["d"] != 4 {
		t.Error("Expected d=4")
	}
}
