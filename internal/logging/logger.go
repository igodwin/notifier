package logging

import (
	"fmt"
	"io"
	"log"
	"os"
	"time"
)

// Logger provides structured logging with ISO 8601 timestamps
type Logger struct {
	*log.Logger
	level LogLevel
}

// LogLevel represents the logging level
type LogLevel int

const (
	DebugLevel LogLevel = iota
	InfoLevel
	WarnLevel
	ErrorLevel
)

// New creates a new logger with ISO 8601 timestamp format
func New(level LogLevel, output io.Writer) *Logger {
	if output == nil {
		output = os.Stdout
	}

	return &Logger{
		Logger: log.New(output, "", 0), // No flags, we'll format ourselves
		level:  level,
	}
}

// NewFromConfig creates a logger from configuration
func NewFromConfig(levelStr string, outputPath string) (*Logger, error) {
	level := parseLevel(levelStr)

	var output io.Writer
	switch outputPath {
	case "stdout", "":
		output = os.Stdout
	case "stderr":
		output = os.Stderr
	default:
		file, err := os.OpenFile(outputPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, err
		}
		output = file
	}

	return New(level, output), nil
}

// formatMessage formats a log message with ISO 8601 timestamp
func (l *Logger) formatMessage(level string, msg string) string {
	timestamp := time.Now().UTC().Format(time.RFC3339)
	return timestamp + " [" + level + "] " + msg
}

// Debug logs a debug message
func (l *Logger) Debug(msg string) {
	if l.level <= DebugLevel {
		l.Logger.Println(l.formatMessage("DEBUG", msg))
	}
}

// Debugf logs a formatted debug message
func (l *Logger) Debugf(format string, args ...interface{}) {
	if l.level <= DebugLevel {
		msg := sprintf(format, args...)
		l.Logger.Println(l.formatMessage("DEBUG", msg))
	}
}

// Info logs an info message
func (l *Logger) Info(msg string) {
	if l.level <= InfoLevel {
		l.Logger.Println(l.formatMessage("INFO", msg))
	}
}

// Infof logs a formatted info message
func (l *Logger) Infof(format string, args ...interface{}) {
	if l.level <= InfoLevel {
		msg := sprintf(format, args...)
		l.Logger.Println(l.formatMessage("INFO", msg))
	}
}

// Warn logs a warning message
func (l *Logger) Warn(msg string) {
	if l.level <= WarnLevel {
		l.Logger.Println(l.formatMessage("WARN", msg))
	}
}

// Warnf logs a formatted warning message
func (l *Logger) Warnf(format string, args ...interface{}) {
	if l.level <= WarnLevel {
		msg := sprintf(format, args...)
		l.Logger.Println(l.formatMessage("WARN", msg))
	}
}

// Error logs an error message
func (l *Logger) Error(msg string) {
	if l.level <= ErrorLevel {
		l.Logger.Println(l.formatMessage("ERROR", msg))
	}
}

// Errorf logs a formatted error message
func (l *Logger) Errorf(format string, args ...interface{}) {
	if l.level <= ErrorLevel {
		msg := sprintf(format, args...)
		l.Logger.Println(l.formatMessage("ERROR", msg))
	}
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(msg string) {
	l.Logger.Println(l.formatMessage("FATAL", msg))
	os.Exit(1)
}

// Fatalf logs a formatted fatal message and exits
func (l *Logger) Fatalf(format string, args ...interface{}) {
	msg := sprintf(format, args...)
	l.Logger.Println(l.formatMessage("FATAL", msg))
	os.Exit(1)
}

// parseLevel parses a log level string
func parseLevel(levelStr string) LogLevel {
	switch levelStr {
	case "debug":
		return DebugLevel
	case "info":
		return InfoLevel
	case "warn", "warning":
		return WarnLevel
	case "error":
		return ErrorLevel
	default:
		return InfoLevel
	}
}

// sprintf is a helper using fmt
func sprintf(format string, args ...interface{}) string {
	return fmt.Sprintf(format, args...)
}
