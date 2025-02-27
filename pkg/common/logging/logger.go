// Package logging provides a structured logging system for the application.
package logging

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// LogLevel represents the severity level of a log message.
type LogLevel int

const (
	// DebugLevel logs detailed information for debugging purposes.
	DebugLevel LogLevel = iota
	// InfoLevel logs general information about application progress.
	InfoLevel
	// WarnLevel logs warnings that might require attention.
	WarnLevel
	// ErrorLevel logs errors that don't cause the application to stop.
	ErrorLevel
	// FatalLevel logs critical errors before the application terminates.
	FatalLevel
)

var (
	// levelNames maps log levels to their string representations.
	levelNames = map[LogLevel]string{
		DebugLevel: "DEBUG",
		InfoLevel:  "INFO",
		WarnLevel:  "WARN",
		ErrorLevel: "ERROR",
		FatalLevel: "FATAL",
	}

	// levelColors maps log levels to ANSI color codes for terminal output.
	levelColors = map[LogLevel]string{
		DebugLevel: "\033[36m", // Cyan
		InfoLevel:  "\033[32m", // Green
		WarnLevel:  "\033[33m", // Yellow
		ErrorLevel: "\033[31m", // Red
		FatalLevel: "\033[35m", // Magenta
	}

	// resetColor is the ANSI code to reset terminal color.
	resetColor = "\033[0m"

	// globalLevel is the minimum log level that will be output.
	globalLevel LogLevel = InfoLevel

	// globalOutput is where log messages are written.
	globalOutput io.Writer = os.Stderr

	// globalColorEnabled determines if color output is enabled.
	globalColorEnabled = true

	// logMutex protects access to the global logging state.
	logMutex sync.Mutex
)

// Logger represents a logger for a specific component.
type Logger struct {
	name string
}

// NewLogger creates a new logger with the given component name.
func NewLogger(name string) *Logger {
	return &Logger{name: name}
}

// SetGlobalLevel sets the minimum log level that will be output.
func SetGlobalLevel(level string) {
	logMutex.Lock()
	defer logMutex.Unlock()

	switch strings.ToLower(level) {
	case "debug":
		globalLevel = DebugLevel
	case "info":
		globalLevel = InfoLevel
	case "warn", "warning":
		globalLevel = WarnLevel
	case "error":
		globalLevel = ErrorLevel
	case "fatal":
		globalLevel = FatalLevel
	default:
		// If the level is invalid, default to InfoLevel
		globalLevel = InfoLevel
	}
}

// SetLogFile sets the output file for logs.
// If the file doesn't exist, it will be created.
// If the file's directory doesn't exist, it will be created.
func SetLogFile(filePath string) error {
	logMutex.Lock()
	defer logMutex.Unlock()

	// Create the directory if it doesn't exist
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	// Open the log file for appending, create it if it doesn't exist
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	// Close the previous file if it was a file
	if closer, ok := globalOutput.(io.Closer); ok && globalOutput != os.Stderr {
		closer.Close()
	}

	globalOutput = file
	globalColorEnabled = false
	return nil
}

// SetColorEnabled sets whether color output is enabled.
func SetColorEnabled(enabled bool) {
	logMutex.Lock()
	defer logMutex.Unlock()
	globalColorEnabled = enabled
}

// Debug logs a message at debug level.
func (l *Logger) Debug(format string, args ...interface{}) {
	l.log(DebugLevel, format, args...)
}

// Info logs a message at info level.
func (l *Logger) Info(format string, args ...interface{}) {
	l.log(InfoLevel, format, args...)
}

// Warn logs a message at warning level.
func (l *Logger) Warn(format string, args ...interface{}) {
	l.log(WarnLevel, format, args...)
}

// Error logs a message at error level.
func (l *Logger) Error(format string, args ...interface{}) {
	l.log(ErrorLevel, format, args...)
}

// Fatal logs a message at fatal level and then calls os.Exit(1).
func (l *Logger) Fatal(format string, args ...interface{}) {
	l.log(FatalLevel, format, args...)
	os.Exit(1)
}

// log logs a message at the specified level.
func (l *Logger) log(level LogLevel, format string, args ...interface{}) {
	// Check if this level should be logged
	logMutex.Lock()
	if level < globalLevel {
		logMutex.Unlock()
		return
	}
	output := globalOutput
	colorEnabled := globalColorEnabled
	logMutex.Unlock()

	// Get caller information
	_, file, line, ok := runtime.Caller(2)
	callerInfo := "unknown"
	if ok {
		// Extract just the filename from the full path
		file = filepath.Base(file)
		callerInfo = fmt.Sprintf("%s:%d", file, line)
	}

	// Format the message
	message := fmt.Sprintf(format, args...)
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	levelName := levelNames[level]
	
	var logLine string
	if colorEnabled {
		color := levelColors[level]
		logLine = fmt.Sprintf("%s [%s%s%s] [%s] [%s] %s\n",
			timestamp, color, levelName, resetColor, l.name, callerInfo, message)
	} else {
		logLine = fmt.Sprintf("%s [%s] [%s] [%s] %s\n",
			timestamp, levelName, l.name, callerInfo, message)
	}

	// Write to the output
	fmt.Fprint(output, logLine)
}
