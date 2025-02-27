package logging

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewLogger(t *testing.T) {
	logger := NewLogger("test")
	if logger == nil {
		t.Fatal("Expected non-nil logger")
	}
	if logger.name != "test" {
		t.Errorf("Expected logger name 'test', got '%s'", logger.name)
	}
}

func TestLogLevels(t *testing.T) {
	// Create a buffer to capture log output
	var buf bytes.Buffer
	oldOutput := globalOutput
	globalOutput = &buf
	defer func() { globalOutput = oldOutput }()

	// Create a logger
	logger := NewLogger("test")

	// Test cases
	tests := []struct {
		name        string
		logFunc     func(string, ...interface{})
		level       LogLevel
		message     string
		shouldLog   bool
		shouldMatch string
	}{
		{
			name:        "Debug level",
			logFunc:     logger.Debug,
			level:       DebugLevel,
			message:     "Debug message",
			shouldLog:   true,
			shouldMatch: "[DEBUG] [test]",
		},
		{
			name:        "Info level",
			logFunc:     logger.Info,
			level:       InfoLevel,
			message:     "Info message",
			shouldLog:   true,
			shouldMatch: "[INFO] [test]",
		},
		{
			name:        "Warn level",
			logFunc:     logger.Warn,
			level:       WarnLevel,
			message:     "Warn message",
			shouldLog:   true,
			shouldMatch: "[WARN] [test]",
		},
		{
			name:        "Error level",
			logFunc:     logger.Error,
			level:       ErrorLevel,
			message:     "Error message",
			shouldLog:   true,
			shouldMatch: "[ERROR] [test]",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Clear the buffer
			buf.Reset()

			// Set the global log level
			logMutex.Lock()
			oldLevel := globalLevel
			globalLevel = tc.level
			globalColorEnabled = false // Disable colors for testing
			logMutex.Unlock()

			// Log the message
			tc.logFunc(tc.message)

			// Check if anything was logged
			output := buf.String()
			if tc.shouldLog && output == "" {
				t.Errorf("Expected log output, but got nothing")
			} else if !tc.shouldLog && output != "" {
				t.Errorf("Expected no log output, but got: %s", output)
			}

			// Check if the output contains the expected text
			if tc.shouldLog && !strings.Contains(output, tc.shouldMatch) {
				t.Errorf("Expected output to contain '%s', got: %s", tc.shouldMatch, output)
			}
			if !tc.shouldLog && strings.Contains(output, tc.shouldMatch) {
				t.Errorf("Expected output to not contain '%s', got: %s", tc.shouldMatch, output)
			}

			// Verify message content
			if tc.shouldLog && !strings.Contains(output, tc.message) {
				t.Errorf("Expected output to contain message '%s', got: %s", tc.message, output)
			}

			// Reset global level
			logMutex.Lock()
			globalLevel = oldLevel
			logMutex.Unlock()
		})
	}
}

func TestSetGlobalLevel(t *testing.T) {
	// Test cases
	tests := []struct {
		name  string
		level string
		want  LogLevel
	}{
		{"Debug level", "debug", DebugLevel},
		{"Info level", "info", InfoLevel},
		{"Warn level", "warn", WarnLevel},
		{"Warning level", "warning", WarnLevel},
		{"Error level", "error", ErrorLevel},
		{"Fatal level", "fatal", FatalLevel},
		{"Invalid level", "invalid", InfoLevel}, // Should default to Info
		{"Case insensitive", "DEBUG", DebugLevel},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			SetGlobalLevel(tc.level)
			if globalLevel != tc.want {
				t.Errorf("SetGlobalLevel(%s) = %v, want %v", tc.level, globalLevel, tc.want)
			}
		})
	}
}

func TestSetLogFile(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "logger_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Test log file
	logFile := filepath.Join(tempDir, "test.log")

	// Set log file
	err = SetLogFile(logFile)
	if err != nil {
		t.Fatalf("SetLogFile failed: %v", err)
	}

	// Log something
	logger := NewLogger("test")
	logger.Info("Test log message")

	// Close log file (it was set as globalOutput)
	if closer, ok := globalOutput.(io.Closer); ok {
		closer.Close()
	}

	// Reset globalOutput
	globalOutput = os.Stderr

	// Check if file was created and contains the log message
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	// Check content
	if !strings.Contains(string(content), "Test log message") {
		t.Errorf("Log file does not contain expected message. Content: %s", string(content))
	}
	if !strings.Contains(string(content), "[INFO] [test]") {
		t.Errorf("Log file does not contain expected level and name. Content: %s", string(content))
	}
}

// TestColorOutput can't be fully automated since it's visual, but we can test the code path
func TestColorOutput(t *testing.T) {
	// Create a buffer to capture log output
	var buf bytes.Buffer
	oldOutput := globalOutput
	globalOutput = &buf
	defer func() { globalOutput = oldOutput }()

	// Set color output on
	logMutex.Lock()
	globalColorEnabled = true
	globalLevel = DebugLevel
	logMutex.Unlock()

	// Log messages at different levels
	logger := NewLogger("color-test")
	logger.Debug("Debug message")
	logger.Info("Info message")
	logger.Warn("Warn message")
	logger.Error("Error message")

	// Check output for color codes
	output := buf.String()
	if !strings.Contains(output, levelColors[DebugLevel]) {
		t.Errorf("Expected output to contain debug color code, but it doesn't")
	}
	if !strings.Contains(output, levelColors[InfoLevel]) {
		t.Errorf("Expected output to contain info color code, but it doesn't")
	}
	if !strings.Contains(output, levelColors[WarnLevel]) {
		t.Errorf("Expected output to contain warn color code, but it doesn't")
	}
	if !strings.Contains(output, levelColors[ErrorLevel]) {
		t.Errorf("Expected output to contain error color code, but it doesn't")
	}
	if !strings.Contains(output, resetColor) {
		t.Errorf("Expected output to contain reset color code, but it doesn't")
	}

	// Test disable colors
	buf.Reset()
	SetColorEnabled(false)
	logger.Info("No color message")
	output = buf.String()
	if strings.Contains(output, levelColors[InfoLevel]) {
		t.Errorf("Expected output to not contain color codes, but it does")
	}
}
